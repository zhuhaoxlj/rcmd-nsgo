package app

import (
	"testing"

	"github.com/mark/rcmd-nsgo/internal/helper"
	"github.com/mark/rcmd-nsgo/internal/model"
)

func app(name, bundle string, windows int) helper.App {
	return helper.App{BundleID: bundle, Name: name, Windows: windows}
}

func TestSuggestRowsKeepsAssignedLetters(t *testing.T) {
	s := model.Default()
	s.Assign("t", "com.apple.Terminal", "Terminal")
	rows := suggestRows([]helper.App{
		app("Safari", "com.apple.Safari", 2),
		app("Terminal", "com.apple.Terminal", 1),
	}, s, "")
	if len(rows) != 2 {
		t.Fatalf("rows: %+v", rows)
	}
	for _, r := range rows {
		if r.bundle == "com.apple.Terminal" && (!r.assigned || r.letter != "t") {
			t.Fatalf("terminal: %+v", r)
		}
	}
}

func TestSuggestRowsLettersDoNotCollide(t *testing.T) {
	s := model.Default()
	rows := suggestRows([]helper.App{
		app("Cindy", "com.xd.cindycn", 1),
		app("Clash Verge", "io.github.clash-verge", 1),
		app("Chat", "com.example.chat", 1),
	}, s, "")
	seen := map[string]bool{}
	for _, r := range rows {
		if r.letter == "" {
			continue
		}
		if seen[r.letter] {
			t.Fatalf("duplicate letter %q in %+v", r.letter, rows)
		}
		seen[r.letter] = true
	}
}

func TestSuggestRowsPrefersWindows(t *testing.T) {
	s := model.Default()
	rows := suggestRows([]helper.App{
		app("Hidden", "com.example.hidden", 0),
		app("Shown", "com.example.shown", 3),
	}, s, "")
	if len(rows) != 1 || rows[0].name != "Shown" {
		t.Fatalf("rows: %+v", rows)
	}
}

func TestSuggestRowsFallsBackWithoutWindows(t *testing.T) {
	// With no windows reported, every running app is still offered, so
	// the picker works before Accessibility is granted.
	s := model.Default()
	rows := suggestRows([]helper.App{
		app("A", "com.example.a", 0),
		app("B", "com.example.b", 0),
	}, s, "")
	if len(rows) != 2 {
		t.Fatalf("rows: %+v", rows)
	}
}

func TestSuggestRowsSearchKeepsLetters(t *testing.T) {
	s := model.Default()
	apps := []helper.App{
		app("alpha", "com.example.alpha", 1),
		app("bravo", "com.example.bravo", 1),
	}
	all := suggestRows(apps, s, "")
	filtered := suggestRows(apps, s, "bravo")
	if len(filtered) != 1 {
		t.Fatalf("filtered: %+v", filtered)
	}
	for _, r := range all {
		if r.name == "bravo" && r.letter != filtered[0].letter {
			t.Fatalf("search changed the suggested letter: %q vs %q", r.letter, filtered[0].letter)
		}
	}
}

func TestLastLetter(t *testing.T) {
	for in, want := range map[string]string{
		"w":   "w",
		"Aw":  "w", // typing over the letter already there
		"A":   "a", // the letter there now
		"w!":  "w", // punctuation after the key
		"":    "",  // cleared
		"  ":  "",  // no key at all
		"微信W": "w", // non-latin before the key
	} {
		if got := lastLetter(in); got != want {
			t.Fatalf("lastLetter(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildEntriesSkipsIgnoredEvenWhenAssigned(t *testing.T) {
	s := &State{settings: model.Default()}
	s.settings.Assign("s", "com.apple.Safari", "Safari")
	s.settings.Ignore("com.apple.Safari")
	entries := s.buildEntries([]helper.App{app("Safari", "com.apple.Safari", 1)})
	for _, e := range entries {
		if e.Bundle == "com.apple.Safari" {
			t.Fatalf("ignored app still listed: %+v", entries)
		}
	}
	// Its letter stays reserved, so another app cannot quietly take it.
	if entries = s.buildEntries([]helper.App{app("Slack", "com.tinyspeck.slackmacgap", 1)}); entries[0].Letter == "s" {
		t.Fatalf("ignored app's letter was handed out: %+v", entries)
	}
}
