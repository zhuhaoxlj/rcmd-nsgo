// Package app wires the NSGo views to the native helper: it owns the
// settings, the app switcher's state machine and the menu bar item.
package app

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"

	"github.com/mark/rcmd-nsgo/internal/helper"
	"github.com/mark/rcmd-nsgo/internal/i18n"
	"github.com/mark/rcmd-nsgo/internal/model"
)

// Entry is one app the switcher shows, with the letter that opens it.
type Entry struct {
	Letter  string
	Bundle  string
	Name    string
	Active  bool
	Windows int
	Icon    []byte
	Running bool
}

func entriesEqual(a, b []Entry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Letter != b[i].Letter || a[i].Bundle != b[i].Bundle || a[i].Active != b[i].Active || a[i].Windows != b[i].Windows {
			return false
		}
	}
	return true
}

// State is the app's shared state. Signals are created in the app scope
// so the settings window, the panel and the tray all read the same values.
type State struct {
	cx       *core.Scope
	store    core.Storage
	settings model.Settings

	version    *core.Signal[int]
	entries    *core.Signal[[]Entry]
	selected   *core.Signal[string]
	status     *core.Signal[string]
	permission *core.Signal[helper.Permission]
	pickable   *core.Signal[[]helper.App]
	// lang follows settings.Language, so views rebuild when it changes.
	lang *core.Signal[string]

	bridge         *helper.Bridge
	panel          *desktop.Window
	settingsWindow *desktop.Window

	holding bool
	paused  bool
}

// New builds the app's state and starts the helper.
func New(cx *core.Scope) *State {
	s := &State{
		cx:    cx,
		store: cx.Storage(),
	}
	if raw, ok := s.store.GetItem("settings"); ok {
		s.settings = model.Load(raw)
	} else {
		s.settings = model.Default()
	}
	s.version = core.NewSignal(cx, 0)
	s.entries = core.NewSignalFunc(cx, []Entry{}, entriesEqual)
	s.selected = core.NewSignal(cx, "")
	s.status = core.NewSignal(cx, "")
	s.permission = core.NewSignal(cx, helper.Permission{})
	s.pickable = core.NewSignalFunc(cx, []helper.App{}, nil)
	s.lang = core.NewSignal(cx, string(i18n.Normalize(s.settings.Language)))
	return s
}

// tr is the text of key in the current language.
func (s *State) tr(key string) string { return i18n.T(s.language(), key) }

// trf is tr with fmt's verbs filled in.
func (s *State) trf(key string, args ...any) string { return i18n.TF(s.language(), key, args...) }

// language is the language the UI shows.
func (s *State) language() i18n.Lang { return i18n.Normalize(s.settings.Language) }

// setLanguage switches the UI language and saves it.
func (s *State) setLanguage(v string) {
	next := i18n.Normalize(v)
	if s.language() == next {
		return
	}
	s.settings.Language = string(next)
	s.save()
	s.lang.Set(string(next))
}

// Start opens the app's surfaces and begins listening to the helper.
func (s *State) Start() {
	cx := s.cx
	desktop.SetShutdownMode(cx, desktop.ShutdownOnExplicit)

	s.settingsWindow = s.openSettingsWindow(true)

	s.panel = desktop.OpenPanel(cx, desktop.PanelOptions{
		Width: 760, Height: 500, NoActivate: true,
	}, func(cx *core.Scope) core.Node { return s.SwitcherView(cx) })

	desktop.NewTray(cx, desktop.TrayOptions{
		Title:   "\u2318",
		Tooltip: s.tr("tray.tooltip"),
		Menu: func() []desktop.MenuItem {
			return []desktop.MenuItem{
				{Label: s.tr("tray.settings"), OnSelect: func() {
					s.showSettings()
				}},
				{Label: s.tr("tray.pause"), Checkable: true, Checked: s.paused, OnSelect: func() {
					s.paused = !s.paused
				}},
				{Separator: true},
				{Label: s.tr("tray.accessibility"), OnSelect: func() {
					cx.OpenURL("x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility", nil)
				}},
				{Separator: true},
				{Label: s.tr("tray.quit"), Role: desktop.MenuQuit, OnSelect: func() { cx.Quit() }},
			}
		},
	})

	s.startHelper()
}

// openSettingsWindow opens the settings window and returns it. Closing it
// hides it instead of closing it: a menu bar app has no other window to
// come back to, and Show on a closed window does nothing.
func (s *State) openSettingsWindow(hidden bool) *desktop.Window {
	w := desktop.OpenWindow(s.cx, desktop.WindowOptions{
		Title: "rcmd-nsgo", Key: "settings", Width: 680, Height: 760, Hidden: hidden,
	}, func(cx *core.Scope) core.Node { return s.SettingsView(cx) })
	if w != nil {
		w.OnClosing(s.cx, func(closing *desktop.WindowClosing) {
			closing.Cancel()
			w.Hide()
		})
	}
	return w
}

// showSettings brings the settings window up, reopening it when it was
// closed outright.
func (s *State) showSettings() {
	if s.settingsWindow == nil || s.settingsWindow.Closed() {
		s.settingsWindow = s.openSettingsWindow(false)
	}
	s.refreshPickableApps()
	if s.settingsWindow != nil {
		s.settingsWindow.Show()
		s.settingsWindow.Focus()
	}
}

func (s *State) startHelper() {
	path, err := helper.Find()
	if err != nil {
		s.status.Set(s.trf("status.helperNotBuilt", err))
		s.showSettings()
		return
	}
	bridge, err := helper.Start(path)
	if err != nil {
		s.status.Set(s.trf("status.helperFailed", err))
		s.showSettings()
		return
	}
	s.bridge = bridge

	go func() {
		for ev := range bridge.Events() {
			e := ev
			s.cx.Post(func() { s.onEvent(e) })
		}
	}()

	// Ask once at launch; the system only shows a prompt the first time.
	go func() { _ = bridge.RequestPermissions() }()
	s.refreshPermissions()
	// The settings window was built before the helper answered, so its
	// app picker starts empty: fill it as soon as the helper is up.
	s.refreshPickableApps()

	// Show the settings window when the app still needs a permission.
	go func() {
		time.Sleep(600 * time.Millisecond)
		s.cx.Post(func() {
			p := s.permission.GetUntracked()
			if !p.Accessibility || !p.InputMonitoring {
				s.showSettings()
			}
		})
	}()
}

func (s *State) onEvent(ev helper.Event) {
	switch ev.Event {
	case "hold":
		if ev.On {
			s.beginHold()
		} else {
			s.endHold()
		}
	case "letter":
		s.onLetter(ev.Letter, ev.Option, ev.Shift)
	case "esc":
		s.cancel()
	case "status":
		s.permission.Set(helper.Permission{Accessibility: ev.Access, InputMonitoring: ev.Input})
	}
}

func (s *State) beginHold() {
	if s.paused || s.holding || s.panel == nil {
		return
	}
	s.holding = true
	s.selected.Set("")
	s.status.Set("")
	s.panel.Show()
	s.refreshApps()
}

func (s *State) endHold() {
	if !s.holding {
		return
	}
	s.holding = false
	letter := s.selected.Get()
	s.selected.Set("")
	if s.panel != nil {
		s.panel.Hide()
	}
	if letter == "" {
		return
	}
	var chosen Entry
	for _, e := range s.entries.Get() {
		if e.Letter == letter {
			chosen = e
			break
		}
	}
	if chosen.Bundle == "" {
		return
	}
	go func() {
		if err := s.bridge.Activate(chosen.Bundle); err != nil {
			s.cx.Post(func() { s.status.Set(s.trf("status.couldNotOpen", chosen.Name, err)) })
		}
	}()
}

func (s *State) cancel() {
	s.holding = false
	s.selected.Set("")
	if s.panel != nil {
		s.panel.Hide()
	}
}

func (s *State) onLetter(letter string, option bool, shift string) {
	if !s.holding || s.bridge == nil {
		return
	}
	letter = model.NormalizeLetter(letter)
	if letter == "" {
		return
	}
	if option && s.settings.AssignWithOption {
		go func() {
			app, err := s.bridge.Frontmost()
			s.cx.Post(func() {
				if err != nil || app.BundleID == "" {
					s.status.Set(s.tr("status.noAppInFront"))
					return
				}
				s.settings.Assign(letter, app.BundleID, app.Name)
				s.save()
				s.status.Set(fmt.Sprintf("%s → %s", strings.ToUpper(letter), app.Name))
				s.selected.Set(letter)
				s.refreshApps()
			})
		}()
		return
	}
	if shift == "right" && s.settings.CycleWindows {
		// Force cycle among apps that share the letter: pick the next one.
		s.cycle(letter)
		return
	}
	s.selected.Set(letter)
}

func (s *State) cycle(letter string) {
	list := s.entries.Get()
	var matches []Entry
	for _, e := range list {
		if e.Letter == letter {
			matches = append(matches, e)
		}
	}
	if len(matches) == 0 {
		return
	}
	current := s.selected.Get()
	index := 0
	for i, e := range matches {
		if e.Letter == current {
			index = (i + 1) % len(matches)
			break
		}
	}
	s.selected.Set(matches[index].Letter)
}

func (s *State) refreshApps() {
	if s.bridge == nil {
		return
	}
	go func() {
		apps, err := s.bridge.Apps()
		s.cx.Post(func() {
			if err != nil {
				s.status.Set(s.trf("status.couldNotList", err))
				return
			}
			s.entries.Set(s.buildEntries(apps))
		})
	}()
}

func (s *State) buildEntries(apps []helper.App) []Entry {
	used := map[string]bool{}
	byBundle := map[string]Entry{}

	// Static assignments come first: they keep their letters.
	for letter, bundle := range s.settings.Assignments {
		used[letter] = true
		byBundle[bundle] = Entry{Letter: letter, Bundle: bundle, Name: s.settings.Name(bundle)}
	}

	var running []helper.App
	for _, a := range apps {
		if a.BundleID == "" || s.settings.IsIgnored(a.BundleID) {
			continue
		}
		running = append(running, a)
	}
	sort.SliceStable(running, func(i, j int) bool {
		if running[i].Active != running[j].Active {
			return running[i].Active
		}
		return strings.ToLower(running[i].Name) < strings.ToLower(running[j].Name)
	})

	order := []string{}
	for _, a := range running {
		e, has := byBundle[a.BundleID]
		if !has {
			e = Entry{Letter: nextLetter(a.Name, used), Bundle: a.BundleID, Name: a.Name}
			if e.Letter != "" {
				used[e.Letter] = true
			}
		}
		e.Name = a.Name
		e.Active = a.Active
		e.Windows = a.Windows
		e.Running = true
		if a.Icon != "" {
			if data, err := base64.StdEncoding.DecodeString(a.Icon); err == nil {
				e.Icon = data
			}
		}
		byBundle[a.BundleID] = e
		order = append(order, a.BundleID)
	}

	seen := map[string]bool{}
	for _, bundle := range order {
		seen[bundle] = true
	}
	for bundle, e := range byBundle {
		if !seen[bundle] {
			e.Running = false
			byBundle[bundle] = e
			order = append(order, bundle)
		}
	}

	entries := make([]Entry, 0, len(order))
	for _, bundle := range order {
		e := byBundle[bundle]
		if e.Letter == "" {
			continue
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return letterRank(entries[i].Letter) < letterRank(entries[j].Letter)
	})
	return entries
}

func nextLetter(name string, used map[string]bool) string {
	lower := strings.ToLower(name)
	for _, r := range lower {
		if r >= 'a' && r <= 'z' && !used[string(r)] {
			return string(r)
		}
	}
	for r := 'a'; r <= 'z'; r++ {
		if !used[string(r)] && !strings.ContainsRune(lower, r) {
			return string(r)
		}
	}
	for r := '0'; r <= '9'; r++ {
		if !used[string(r)] {
			return string(r)
		}
	}
	return ""
}

func letterRank(letter string) int {
	if len(letter) == 0 {
		return 100
	}
	c := letter[0]
	if c >= 'a' && c <= 'z' {
		return int(c - 'a')
	}
	if c >= '0' && c <= '9' {
		return 26 + int(c-'0')
	}
	return 100
}

func (s *State) save() {
	_ = s.store.SetItem("settings", s.settings.Encode())
	s.version.Update(func(v int) int { return v + 1 })
}

func (s *State) refreshPermissions() {
	if s.bridge == nil {
		return
	}
	go func() {
		p, err := s.bridge.Permissions()
		s.cx.Post(func() {
			if err == nil {
				s.permission.Set(p)
			}
		})
	}()
}

// SortedAssignments returns the static assignments by letter.
func (s *State) SortedAssignments() [][2]string {
	out := make([][2]string, 0, len(s.settings.Assignments))
	for letter, bundle := range s.settings.Assignments {
		out = append(out, [2]string{letter, bundle})
	}
	sort.Slice(out, func(i, j int) bool { return letterRank(out[i][0]) < letterRank(out[j][0]) })
	return out
}

// refreshPickableApps fills pickable with the running apps the settings
// window offers for assignment: every regular app that is not ignored,
// the active one first, then by name.
func (s *State) refreshPickableApps() {
	if s.bridge == nil {
		return
	}
	go func() {
		apps, err := s.bridge.Apps()
		s.cx.Post(func() {
			if err != nil {
				return
			}
			out := make([]helper.App, 0, len(apps))
			for _, a := range apps {
				if a.BundleID == "" || s.settings.IsIgnored(a.BundleID) {
					continue
				}
				out = append(out, a)
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].Active != out[j].Active {
					return out[i].Active
				}
				// Same order buildEntries uses, so a suggested letter here
				// matches the letter the switcher shows for that app.
				return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
			})
			s.pickable.Set(out)
		})
	}()
}

// PickableApps is the running apps the settings window may assign.
func (s *State) PickableApps() []helper.App { return s.pickable.Get() }

// AssignNext gives bundle the next free letter, or letter when it is
// given, and saves. It returns the letter used, "" when every letter is
// taken.
func (s *State) AssignNext(letter, bundle, name string) string {
	if letter == "" {
		used := map[string]bool{}
		for l := range s.settings.Assignments {
			used[l] = true
		}
		letter = nextLetter(name, used)
	}
	if letter == "" {
		return ""
	}
	s.settings.Assign(letter, bundle, name)
	s.save()
	return letter
}
