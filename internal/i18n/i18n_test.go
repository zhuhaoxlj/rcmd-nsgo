package i18n

import (
	"strings"
	"testing"
)

func TestEveryMessageHasBothLanguages(t *testing.T) {
	for key, text := range messages {
		if strings.TrimSpace(text[0]) == "" {
			t.Fatalf("%s has no Chinese text", key)
		}
		if strings.TrimSpace(text[1]) == "" {
			t.Fatalf("%s has no English text", key)
		}
	}
}

func TestTFollowsTheLanguage(t *testing.T) {
	if got := T(Chinese, "switcher.title"); got != "切换应用" {
		t.Fatalf("Chinese: %q", got)
	}
	if got := T(English, "switcher.title"); got != "Switch to app" {
		t.Fatalf("English: %q", got)
	}
}

func TestTOfUnknownKeyShowsTheKey(t *testing.T) {
	// A missing text should be visible, not silent.
	if got := T(English, "no.such.key"); got != "no.such.key" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeFallsBackToChinese(t *testing.T) {
	for in, want := range map[string]Lang{"en": English, "zh": Chinese, "": Chinese, "de": Chinese} {
		if got := Normalize(in); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTFFillsIn(t *testing.T) {
	if got := TF(Chinese, "switcher.windows", 3); got != "3 个窗口" {
		t.Fatalf("Chinese: %q", got)
	}
	if got := TF(English, "switcher.windows", 3); got != "3 windows" {
		t.Fatalf("English: %q", got)
	}
}
