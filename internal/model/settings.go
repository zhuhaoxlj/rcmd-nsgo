// Package model holds the app's persisted settings: which letter opens
// which app, and the behaviours the settings window exposes.
package model

import (
	"encoding/json"
	"sort"
	"strings"
)

// Settings is what the app remembers between launches.
type Settings struct {
	// Language is the language the UI shows, "zh" or "en".
	Language string `json:"language,omitempty"`
	// Assignments maps a letter (a-z, 0-9) to a bundle identifier.
	Assignments map[string]string `json:"assignments"`
	// Names remembers the display name of an assigned app that may not run.
	Names map[string]string `json:"names"`
	// Ignored bundle identifiers never take part in the switcher.
	Ignored []string `json:"ignored"`
	// AssignWithOption lets Option+letter assign the frontmost app.
	AssignWithOption bool `json:"assignWithOption"`
	// CycleWindows focuses the next window of an app already in front
	// when its letter is pressed again.
	CycleWindows bool `json:"cycleWindows"`
	// HidePanel switches without showing the panel: the letter alone
	// picks the app, and the listing skips icons and window counts.
	HidePanel bool `json:"hidePanel"`
}

// Default returns the settings a fresh install starts with.
func Default() Settings {
	return Settings{
		Language:         "zh",
		Assignments:      map[string]string{},
		Names:            map[string]string{},
		AssignWithOption: true,
		CycleWindows:     true,
	}
}

// Load decodes raw, falling back to defaults for anything missing.
func Load(raw string) Settings {
	s := Default()
	if raw == "" {
		return s
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Default()
	}
	if s.Assignments == nil {
		s.Assignments = map[string]string{}
	}
	if s.Names == nil {
		s.Names = map[string]string{}
	}
	if s.Language == "" {
		// Settings written before the UI had languages keep the original one.
		s.Language = "zh"
	}
	return s
}

// Encode renders the settings for storage.
func (s Settings) Encode() string {
	b, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// NormalizeLetter keeps the first character, lowercased, when it is a
// letter or digit; otherwise it returns "".
func NormalizeLetter(v string) string {
	for _, r := range strings.ToLower(strings.TrimSpace(v)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return string(r)
		}
		return ""
	}
	return ""
}

// Assign stores bundle under letter, replacing whatever held it and the
// letter that bundle held before.
func (s *Settings) Assign(letter, bundle, name string) {
	letter = NormalizeLetter(letter)
	if letter == "" || bundle == "" {
		return
	}
	for l, b := range s.Assignments {
		if b == bundle && l != letter {
			delete(s.Assignments, l)
		}
	}
	s.Assignments[letter] = bundle
	if name != "" {
		s.Names[bundle] = name
	}
}

// RemoveAssignment clears the letter and forgets its name when nothing
// else uses that bundle.
func (s *Settings) RemoveAssignment(letter string) {
	bundle, ok := s.Assignments[letter]
	if !ok {
		return
	}
	delete(s.Assignments, letter)
	for _, b := range s.Assignments {
		if b == bundle {
			return
		}
	}
	delete(s.Names, bundle)
}

// Name returns the remembered name of bundle.
func (s Settings) Name(bundle string) string { return s.Names[bundle] }

// IsIgnored reports whether bundle is on the ignore list.
func (s Settings) IsIgnored(bundle string) bool {
	for _, b := range s.Ignored {
		if b == bundle {
			return true
		}
	}
	return false
}

// Ignore adds bundle to the ignore list once.
func (s *Settings) Ignore(bundle string) {
	if bundle == "" || s.IsIgnored(bundle) {
		return
	}
	s.Ignored = append(s.Ignored, bundle)
	sort.Strings(s.Ignored)
}

// Unignore removes bundle from the ignore list.
func (s *Settings) Unignore(bundle string) {
	out := s.Ignored[:0]
	for _, b := range s.Ignored {
		if b != bundle {
			out = append(out, b)
		}
	}
	s.Ignored = out
}
