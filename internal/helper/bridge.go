// Package helper runs the small native macOS helper that watches the
// keyboard and moves other applications' windows.
//
// This mirrors rcmd's own design: the shipped app embeds a privileged
// helper (rcmd used Hammerspoon) and talks to it over a local channel.
// Here the channel is JSON lines on the helper's standard streams, so
// the app stays a single NSGo process plus one child.
package helper

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// App is one running application the helper reports.
type App struct {
	BundleID string `json:"bundleID"`
	Name     string `json:"name"`
	PID      int    `json:"pid"`
	Active   bool   `json:"active"`
	Windows  int    `json:"windows"`
	// Icon is a base64 encoded PNG of the application's icon.
	Icon string `json:"icon,omitempty"`
}

// Permission is what the system currently allows the helper to do.
type Permission struct {
	Accessibility   bool `json:"accessibility"`
	InputMonitoring bool `json:"inputMonitoring"`
}

// Event is something the helper heard on the keyboard.
type Event struct {
	Event  string `json:"event"`
	On     bool   `json:"on,omitempty"`
	Letter string `json:"letter,omitempty"`
	Option bool   `json:"option,omitempty"`
	Shift  string `json:"shift,omitempty"`
	Access bool   `json:"accessibility,omitempty"`
	Input  bool   `json:"inputMonitoring,omitempty"`
}

type response struct {
	ID     uint64          `json:"id"`
	OK     bool            `json:"ok"`
	Error  string          `json:"error"`
	Result json.RawMessage `json:"result"`
}

// Bridge owns the helper process and its two channels.
type Bridge struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	events  chan Event
	mu      sync.Mutex
	pending map[uint64]chan response
	seq     atomic.Uint64
	done    chan struct{}
	close   sync.Once
}

// Find locates the helper binary: RCMD_HELPER, next to the running
// executable, or the development build directory.
func Find() (string, error) {
	if p := os.Getenv("RCMD_HELPER"); p != "" {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "rcmd-helper")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
		// Inside a built .app the executable sits in Contents/MacOS, and
		// the helper is placed beside it; a development run keeps it in
		// the module's build directory.
		for _, rel := range []string{"../../build/rcmd-helper", "../build/rcmd-helper", "build/rcmd-helper"} {
			candidate = filepath.Join(filepath.Dir(exe), rel)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return filepath.Clean(candidate), nil
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(wd, "build", "rcmd-helper")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("rcmd-helper not found; run make helper")
}

// Start launches the helper and begins reading its events.
func Start(path string) (*Bridge, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	b := &Bridge{
		cmd:     cmd,
		stdin:   stdin,
		events:  make(chan Event, 64),
		pending: map[uint64]chan response{},
		done:    make(chan struct{}),
	}
	go b.read(stdout)
	go func() {
		_ = cmd.Wait()
		b.closeOnce()
	}()
	return b, nil
}

func (b *Bridge) read(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var probe struct {
			Event string `json:"event"`
			ID    uint64 `json:"id"`
		}
		if err := json.Unmarshal(line, &probe); err != nil {
			continue
		}
		if probe.Event != "" {
			var ev Event
			if json.Unmarshal(line, &ev) == nil {
				select {
				case b.events <- ev:
				default:
				}
			}
			continue
		}
		if probe.ID == 0 {
			continue
		}
		var resp response
		if json.Unmarshal(line, &resp) != nil {
			continue
		}
		b.mu.Lock()
		ch := b.pending[resp.ID]
		delete(b.pending, resp.ID)
		b.mu.Unlock()
		if ch != nil {
			ch <- resp
		}
	}
	b.closeOnce()
}

// Events is the stream of keyboard events the helper hears.
func (b *Bridge) Events() <-chan Event { return b.events }

// Call runs one action and decodes its result into out.
func (b *Bridge) Call(action string, payload any, out any) error {
	id := b.seq.Add(1)
	req := map[string]any{"id": id, "action": action}
	if payload != nil {
		req["payload"] = payload
	}
	line, err := json.Marshal(req)
	if err != nil {
		return err
	}
	ch := make(chan response, 1)
	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()
	if _, err := b.stdin.Write(append(line, '\n')); err != nil {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return err
	}
	resp := <-ch
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "helper refused " + action
		}
		return errors.New(resp.Error)
	}
	if out != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("decode %s: %w", action, err)
		}
	}
	return nil
}

// Apps lists the applications the switcher may show.
func (b *Bridge) Apps() ([]App, error) {
	var apps []App
	err := b.Call("apps", nil, &apps)
	return apps, err
}

// Activate brings bundle to the front, starting it when it is not running.
func (b *Bridge) Activate(bundle string) error {
	return b.Call("activate", map[string]any{"bundle": bundle}, nil)
}

// FocusWindow raises window index of bundle's process.
func (b *Bridge) FocusWindow(bundle string, index int) error {
	return b.Call("focusWindow", map[string]any{"bundle": bundle, "index": index}, nil)
}

// MinimizeOthers minimizes every window of bundle except the front one.
func (b *Bridge) MinimizeOthers(bundle string) error {
	return b.Call("minimizeOthers", map[string]any{"bundle": bundle}, nil)
}

// MinimizeAll minimizes every window of bundle.
func (b *Bridge) MinimizeAll(bundle string) error {
	return b.Call("minimizeAll", map[string]any{"bundle": bundle}, nil)
}

// Hide hides bundle.
func (b *Bridge) Hide(bundle string) error {
	return b.Call("hide", map[string]any{"bundle": bundle}, nil)
}

// Frontmost reports the application in front.
func (b *Bridge) Frontmost() (App, error) {
	var app App
	err := b.Call("frontmost", nil, &app)
	return app, err
}

// Permissions reports what the system allows.
func (b *Bridge) Permissions() (Permission, error) {
	var p Permission
	err := b.Call("permissions", nil, &p)
	return p, err
}

// RequestPermissions asks the system to show its permission prompts.
func (b *Bridge) RequestPermissions() error {
	return b.Call("requestPermissions", nil, nil)
}

// Close ends the helper.
func (b *Bridge) Close() {
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
	_ = b.stdin.Close()
	b.closeOnce()
}

func (b *Bridge) closeOnce() {
	b.close.Do(func() {
		close(b.done)
		b.mu.Lock()
		for id, ch := range b.pending {
			close(ch)
			delete(b.pending, id)
		}
		b.mu.Unlock()
	})
}
