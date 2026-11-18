package model

import "testing"

func TestAssignMovesLetter(t *testing.T) {
	s := Default()
	s.Assign("s", "com.apple.Safari", "Safari")
	if got := s.Assignments["s"]; got != "com.apple.Safari" {
		t.Fatalf("assign: %q", got)
	}
	s.Assign("f", "com.apple.Safari", "Safari")
	if _, ok := s.Assignments["s"]; ok {
		t.Fatalf("old letter kept: %v", s.Assignments)
	}
	if got := s.Assignments["f"]; got != "com.apple.Safari" {
		t.Fatalf("new letter: %q", got)
	}
}

func TestRemoveForgetsName(t *testing.T) {
	s := Default()
	s.Assign("m", "com.apple.Mail", "Mail")
	s.RemoveAssignment("m")
	if _, ok := s.Names["com.apple.Mail"]; ok {
		t.Fatal("name kept after the last letter went")
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	s := Default()
	s.Assign("t", "com.apple.Terminal", "Terminal")
	s.Ignore("com.example.hidden")
	got := Load(s.Encode())
	if got.Assignments["t"] != "com.apple.Terminal" || !got.IsIgnored("com.example.hidden") {
		t.Fatalf("round trip: %+v", got)
	}
}

func TestNormalizeLetter(t *testing.T) {
	for in, want := range map[string]string{" A ": "a", "9": "9", "": "", "!": "", "ab": "a"} {
		if got := NormalizeLetter(in); got != want {
			t.Fatalf("NormalizeLetter(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLanguageDefaultsToChinese(t *testing.T) {
	if got := Default().Language; got != "zh" {
		t.Fatalf("default language = %q", got)
	}
	// Settings saved before the UI had languages keep the original one.
	old := `{"assignments":{"t":"com.apple.Terminal"}}`
	if got := Load(old).Language; got != "zh" {
		t.Fatalf("loaded language = %q", got)
	}
}

func TestLanguageRoundTrip(t *testing.T) {
	s := Default()
	s.Language = "en"
	if got := Load(s.Encode()).Language; got != "en" {
		t.Fatalf("round trip language = %q", got)
	}
}

func TestHidePanelDefaultsOff(t *testing.T) {
	if Default().HidePanel {
		t.Fatal("blind switching should start off")
	}
	s := Default()
	s.HidePanel = true
	if !Load(s.Encode()).HidePanel {
		t.Fatal("hide panel did not round trip")
	}
}
