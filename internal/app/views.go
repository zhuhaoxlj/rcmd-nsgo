package app

import (
	"strings"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/mark/rcmd-nsgo/internal/model"
)

const accessibilitySettings = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"

// reloadSettingsWindow rebuilds the settings window after a change: the
// window's views read the settings when they are built.
func (s *State) reloadSettingsWindow() {
	if s.settingsWindow == nil {
		return
	}
	s.settingsWindow.Close()
	s.settingsWindow = desktop.OpenWindow(s.cx, desktop.WindowOptions{
		Title: "rcmd-nsgo", Key: "settings", Width: 680, Height: 760,
	}, func(cx *core.Scope) core.Node { return s.SettingsView(cx) })
	s.settingsWindow.Show()
}

func letterBadge(cx *core.Scope, letter string, filled bool) core.Node {
	c := ui.ColorsOf(cx)
	return core.View{
		Style: core.Style{
			Width: core.Pt(26), Height: core.Pt(26), CornerRadius: 7,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Background: c.Inset(), BorderWidth: 1, BorderColor: c.Border(),
		},
		StyleFunc: func() core.Style {
			if filled {
				return core.Style{Width: core.Pt(26), Height: core.Pt(26), CornerRadius: 7,
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
					Background: c.AccentFill(), BorderWidth: 1, BorderColor: c.Accent()}
			}
			return core.Style{Width: core.Pt(26), Height: core.Pt(26), CornerRadius: 7,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Background: c.Inset(), BorderWidth: 1, BorderColor: c.Border()}
		},
		Children: []core.Node{core.Text{Content: strings.ToUpper(letter), Style: core.Style{FontSize: 12, FontWeight: core.FontWeightSemibold, Color: c.Text()}}},
	}
}

// SettingsView is the window the menu bar item opens.
func (s *State) SettingsView(cx *core.Scope) core.Node {
	c := ui.ColorsOf(cx)
	draftLetter := core.NewSignal(cx, "")
	draftBundle := core.NewSignal(cx, "")
	draftIgnore := core.NewSignal(cx, "")
	loginOn := core.NewSignal(cx, false)
	desktop.LaunchAtLogin(cx, func(on bool) { cx.Post(func() { loginOn.Set(on) }) })
	s.refreshPermissions()

	card := func(children ...core.Node) core.Node {
		return core.View{Style: core.Style{
			Direction: core.Column, Gap: 10, Padding: core.All(16), CornerRadius: 12,
			Background: c.Surface(), BorderWidth: 1, BorderColor: c.Border(),
		}, Children: children}
	}

	permRow := func(name string, ok func() bool) core.Node {
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			ui.Text(name, 13, ui.Regular, c.Text),
			ui.Textf(func() string {
				if ok() {
					return "Granted"
				}
				return "Needed"
			}, 12, ui.Medium, func() core.Color {
				if ok() {
					return c.Accent()
				}
				return c.Danger()
			}),
		}}
	}

	behaviorRow := func(label, detail string, checked func() bool, onChange func(bool)) core.Node {
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, Gap: 2}, Children: []core.Node{
				ui.Text(label, 13, ui.Regular, c.Text),
				ui.Text(detail, 11, ui.Regular, c.Muted),
			}},
			ui.Switch{Label: label, Checked: checked, OnChange: onChange},
		}}
	}

	children := []core.Node{
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			ui.Text("rcmd-nsgo", 26, ui.Bold, c.Text),
			ui.Text("Hold the right Command key, then press the letter of the app you want.", 13, ui.Regular, c.Muted),
		}},
		card(
			ui.Text("Permissions", 13, ui.Semibold, c.Text),
			ui.Text("The helper needs Accessibility to watch keys and move windows, and Input Monitoring on macOS 10.15 and later.", 11, ui.Regular, c.Muted),
			permRow("Accessibility", func() bool { return s.permission.Get().Accessibility }),
			permRow("Input Monitoring", func() bool { return s.permission.Get().InputMonitoring }),
			core.View{Style: core.Style{Direction: core.Row, Gap: 8, Wrap: core.Wrap}, Children: []core.Node{
				ui.Button{Text: "Request Access", Variant: ui.ButtonPrimary, OnPress: func() {
					if s.bridge != nil {
						go func() { _ = s.bridge.RequestPermissions() }()
					}
				}},
				ui.Button{Text: "Open System Settings", Variant: ui.ButtonSecondary, OnPress: func() {
					cx.OpenURL(accessibilitySettings, nil)
				}},
				ui.Button{Text: "Recheck", Variant: ui.ButtonGhost, OnPress: s.refreshPermissions},
			}},
			ui.Textf(func() string { return s.status.Get() }, 11, ui.Regular, c.Muted),
		),
		card(
			ui.Text("Behaviour", 13, ui.Semibold, c.Text),
			behaviorRow("Assign with Option", "Hold the trigger key and press Option plus a letter to assign the app in front.",
				func() bool { return s.settings.AssignWithOption }, func(on bool) {
					s.settings.AssignWithOption = on
					s.save()
				}),
			behaviorRow("Cycle windows", "Pressing a letter a second time moves to the app's next window.",
				func() bool { return s.settings.CycleWindows }, func(on bool) {
					s.settings.CycleWindows = on
					s.save()
				}),
			behaviorRow("Launch at login", "Start rcmd-nsgo when you log in. macOS only takes this for a built app.",
				func() bool { return loginOn.Get() }, func(on bool) {
					desktop.SetLaunchAtLogin(cx, on)
					loginOn.Set(on)
				}),
		),
	}

	assignments := s.SortedAssignments()
	assignRows := []core.Node{}
	if len(assignments) == 0 {
		assignRows = append(assignRows, ui.Text("No letters assigned yet. Hold right \u2318 and press Option + a letter over an app, or add one below.", 11, ui.Regular, c.Muted))
	}
	for _, pair := range assignments {
		letter, bundle := pair[0], pair[1]
		name := s.settings.Name(bundle)
		if name == "" {
			name = bundle
		}
		assignRows = append(assignRows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(4, 0)}, Children: []core.Node{
			letterBadge(cx, letter, false),
			core.View{Style: core.Style{FlexGrow: 1, Gap: 1}, Children: []core.Node{
				ui.Text(name, 13, ui.Medium, c.Text),
				ui.Text(bundle, 10, ui.Regular, c.Muted),
			}},
			ui.Button{Text: "Remove", Variant: ui.ButtonGhost, OnPress: func() {
				s.settings.RemoveAssignment(letter)
				s.save()
				s.reloadSettingsWindow()
			}},
		}})
	}
	assignChildren := []core.Node{
		ui.Text("Letter assignments", 13, ui.Semibold, c.Text),
	}
	assignChildren = append(assignChildren, assignRows...)
	assignChildren = append(assignChildren, core.View{Style: core.Style{Direction: core.Row, Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{
		core.Input{Value: draftLetter.Get, OnChange: draftLetter.Set, Placeholder: "A", MaxLength: 1, Style: core.Style{Width: core.Pt(44)}},
		core.Input{Value: draftBundle.Get, OnChange: draftBundle.Set, Placeholder: "com.apple.Safari", Style: core.Style{FlexGrow: 1}},
		ui.Button{Text: "Add", Variant: ui.ButtonSecondary, OnPress: func() {
			letter := model.NormalizeLetter(draftLetter.Get())
			bundle := strings.TrimSpace(draftBundle.Get())
			if letter == "" || bundle == "" {
				return
			}
			s.settings.Assign(letter, bundle, "")
			s.save()
			draftLetter.Set("")
			draftBundle.Set("")
			s.reloadSettingsWindow()
		}},
	}})
	children = append(children, card(assignChildren...))

	ignoredRows := []core.Node{}
	if len(s.settings.Ignored) == 0 {
		ignoredRows = append(ignoredRows, ui.Text("No ignored apps.", 11, ui.Regular, c.Muted))
	}
	for _, bundle := range s.settings.Ignored {
		b := bundle
		ignoredRows = append(ignoredRows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
			ui.Text(b, 12, ui.Regular, c.Text),
			core.View{Style: core.Style{FlexGrow: 1}},
			ui.Button{Text: "Remove", Variant: ui.ButtonGhost, OnPress: func() {
				s.settings.Unignore(b)
				s.save()
				s.reloadSettingsWindow()
			}},
		}})
	}
	ignoredChildren := []core.Node{
		ui.Text("Ignored apps", 13, ui.Semibold, c.Text),
		ui.Text("Ignored apps never appear in the switcher.", 11, ui.Regular, c.Muted),
	}
	ignoredChildren = append(ignoredChildren, ignoredRows...)
	ignoredChildren = append(ignoredChildren, core.View{Style: core.Style{Direction: core.Row, Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{
		core.Input{Value: draftIgnore.Get, OnChange: draftIgnore.Set, Placeholder: "com.example.app", Style: core.Style{FlexGrow: 1}},
		ui.Button{Text: "Ignore", Variant: ui.ButtonSecondary, OnPress: func() {
			bundle := strings.TrimSpace(draftIgnore.Get())
			if bundle == "" {
				return
			}
			s.settings.Ignore(bundle)
			s.save()
			draftIgnore.Set("")
			s.reloadSettingsWindow()
		}},
	}})
	children = append(children, card(ignoredChildren...))

	children = append(children, ui.Text("rcmd-nsgo 0.1.0 · a reverse-engineered rcmd, built with NSGo.", 10, ui.Regular, c.Muted))

	return core.Scroll{Style: core.Style{FlexGrow: 1, Background: c.Background()}, Children: []core.Node{
		core.View{Style: core.Style{Padding: core.All(24), Gap: 14}, Children: children},
	}}
}

// SwitcherView is the overlay the trigger key shows.
func (s *State) SwitcherView(cx *core.Scope) core.Node {
	c := ui.ColorsOf(cx)
	return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(18), Gap: 12, Background: c.Background()}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			ui.Text("Switch to app", 20, ui.Semibold, c.Text),
			ui.Textf(func() string {
				if v := s.status.Get(); v != "" {
					return v
				}
				return "Release \u2318 to switch · Esc cancels"
			}, 11, ui.Regular, c.Muted),
		}},
		core.List{
			Count:         func() int { return len(s.entries.Get()) },
			Key:           func(i int) any { return s.entries.Get()[i].Bundle },
			NumColumns:    4,
			Gap:           8,
			ColumnGap:     8,
			EstimatedSize: 86,
			Style:         core.Style{FlexGrow: 1},
			Empty:         ui.Text("No apps yet. Grant Accessibility and Input Monitoring above, then hold the right \u2318 again.", 12, ui.Regular, c.Muted),
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				list := s.entries.Get()
				i := row.Index()
				if i < 0 || i >= len(list) {
					return core.View{}
				}
				return s.entryCard(cx, list[i])
			},
		},
		core.View{Style: core.Style{Direction: core.Row, Gap: 12, Wrap: core.Wrap}, Children: []core.Node{
			ui.Text("Option + letter assigns the app in front", 10, ui.Regular, c.Muted),
			ui.Text("Right Shift + letter cycles apps that share a letter", 10, ui.Regular, c.Muted),
		}},
	}}
}

func (s *State) entryCard(cx *core.Scope, e Entry) core.Node {
	c := ui.ColorsOf(cx)
	selected := func() bool { return s.selected.Get() == e.Letter }
	style := func() core.Style {
		if selected() {
			return core.Style{Direction: core.Column, Gap: 6, Padding: core.All(10), CornerRadius: 10,
				Background: c.AccentFill(), BorderWidth: 1, BorderColor: c.Accent()}
		}
		return core.Style{Direction: core.Column, Gap: 6, Padding: core.All(10), CornerRadius: 10,
			Background: c.Surface(), BorderWidth: 1, BorderColor: c.Border()}
	}
	fg := func() core.Color {
		if selected() {
			return c.OnAccent()
		}
		return c.Text()
	}
	muted := func() core.Color {
		if selected() {
			return c.OnAccent()
		}
		return c.Muted()
	}

	head := []core.Node{letterBadge(cx, e.Letter, selected())}
	if len(e.Icon) > 0 {
		head = append(head, core.Image{Source: core.ImageSource{Data: e.Icon}, Style: core.Style{Width: core.Pt(30), Height: core.Pt(30)}})
	}

	kids := []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: head},
		ui.Textf(func() string { return e.Name }, 13, ui.Medium, fg),
	}
	if !e.Running {
		kids = append(kids, ui.Text("not running", 10, ui.Regular, muted))
	} else if e.Windows > 1 {
		kids = append(kids, ui.Textf(func() string { return itoa(e.Windows) + " windows" }, 10, ui.Regular, muted))
	}

	return core.View{
		StyleFunc: style,
		OnPress: func(core.PressEvent) {
			s.activate(e)
		},
		Children: kids,
	}
}

func (s *State) activate(e Entry) {
	if s.bridge == nil || e.Bundle == "" {
		return
	}
	s.selected.Set(e.Letter)
	go func() {
		if err := s.bridge.Activate(e.Bundle); err != nil {
			s.cx.Post(func() { s.status.Set("could not open " + e.Name + ": " + err.Error()) })
		}
	}()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [8]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
