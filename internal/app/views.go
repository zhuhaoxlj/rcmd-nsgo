package app

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/mark/rcmd-nsgo/internal/helper"
	"github.com/mark/rcmd-nsgo/internal/i18n"
	"github.com/mark/rcmd-nsgo/internal/model"
)

const accessibilitySettings = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"

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

// SettingsView is the window the menu bar item opens. Switching the
// language builds it again, so its text follows at once.
func (s *State) SettingsView(cx *core.Scope) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		_ = s.lang.Get()
		return s.settingsBody(cx)
	})
}

func (s *State) settingsBody(cx *core.Scope) core.Node {
	c := ui.ColorsOf(cx)
	// draftIgnore is the bundle field of the ignored-apps card; the
	// assignment picker keeps its own state internally.
	draftIgnore := core.NewSignal(cx, "")
	loginOn := core.NewSignal(cx, false)
	desktop.LaunchAtLogin(cx, func(on bool) { cx.Post(func() { loginOn.Set(on) }) })
	s.refreshPermissions()
	s.refreshPickableApps()

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
					return s.tr("permissions.granted")
				}
				return s.tr("permissions.needed")
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
			ui.Text(s.tr("settings.subtitle"), 13, ui.Regular, c.Muted),
		}},
		card(
			ui.Text(s.tr("permissions.title"), 13, ui.Semibold, c.Text),
			ui.Text(s.tr("permissions.detail"), 11, ui.Regular, c.Muted),
			permRow(s.tr("permissions.accessibility"), func() bool { return s.permission.Get().Accessibility }),
			permRow(s.tr("permissions.inputMonitoring"), func() bool { return s.permission.Get().InputMonitoring }),
			core.View{Style: core.Style{Direction: core.Row, Gap: 8, Wrap: core.Wrap}, Children: []core.Node{
				ui.Button{Text: s.tr("permissions.request"), Variant: ui.ButtonPrimary, OnPress: func() {
					if s.bridge != nil {
						go func() { _ = s.bridge.RequestPermissions() }()
					}
				}},
				ui.Button{Text: s.tr("permissions.open"), Variant: ui.ButtonSecondary, OnPress: func() {
					cx.OpenURL(accessibilitySettings, nil)
				}},
				ui.Button{Text: s.tr("permissions.recheck"), Variant: ui.ButtonGhost, OnPress: s.refreshPermissions},
			}},
			ui.Textf(func() string { return s.status.Get() }, 11, ui.Regular, c.Muted),
			core.Dynamic(func(*core.Scope) core.Node {
				p := s.permission.Get()
				if p.Accessibility && p.InputMonitoring {
					return core.Group(nil)
				}
				return ui.Text(s.tr("permissions.stale"), 10, ui.Regular, c.Danger)
			}),
		),
		card(
			ui.Text(s.tr("settings.behaviour.title"), 13, ui.Semibold, c.Text),
			behaviorRow(s.tr("behaviour.assignWithOption"), s.tr("behaviour.assignWithOptionDetail"),
				func() bool { return s.settings.AssignWithOption }, func(on bool) {
					s.settings.AssignWithOption = on
					s.save()
				}),
			behaviorRow(s.tr("behaviour.cycleWindows"), s.tr("behaviour.cycleWindowsDetail"),
				func() bool { return s.settings.CycleWindows }, func(on bool) {
					s.settings.CycleWindows = on
					s.save()
				}),
			behaviorRow(s.tr("behaviour.launchAtLogin"), s.tr("behaviour.launchAtLoginDetail"),
				func() bool { return loginOn.Get() }, func(on bool) {
					desktop.SetLaunchAtLogin(cx, on)
					loginOn.Set(on)
				}),
		),
	}

	languageVariant := func(lang string) ui.ButtonVariant {
		if s.language() == i18n.Lang(lang) {
			return ui.ButtonPrimary
		}
		return ui.ButtonSecondary
	}
	children = append(children, card(
		ui.Text(s.tr("settings.language.title"), 13, ui.Semibold, c.Text),
		ui.Text(s.tr("settings.language.detail"), 11, ui.Regular, c.Muted),
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Button{Text: s.tr("settings.language.chinese"), Variant: languageVariant("zh"), OnPress: func() { s.setLanguage("zh") }},
			ui.Button{Text: s.tr("settings.language.english"), Variant: languageVariant("en"), OnPress: func() { s.setLanguage("en") }},
		}},
	))

	assignCard := card(
		ui.Text(s.tr("assign.title"), 13, ui.Semibold, c.Text),
		core.Dynamic(func(cx *core.Scope) core.Node {
			_ = s.version.Get() // rebuild rows after assign/remove
			assignments := s.SortedAssignments()
			rows := []core.Node{}
			if len(assignments) == 0 {
				rows = append(rows, ui.Text(s.tr("assign.empty"), 11, ui.Regular, c.Muted))
			}
			for _, pair := range assignments {
				letter, bundle := pair[0], pair[1]
				name := s.settings.Name(bundle)
				if name == "" {
					name = bundle
				}
				// The letter field is editable in place, as rcmd's
				// assignments list is: type a new key to move the app to
				// it, and whichever app held that key gives it up.
				//
				// The key typed is read from OnKeyPress rather than from
				// the text: the caret lands wherever the click put it, so
				// reading the edited text meant a new key could end up
				// next to the old one.
				keyField := core.Input{
					Value: func() string { return strings.ToUpper(letter) },
					OnKeyPress: func(key string) {
						if key == "Backspace" {
							s.settings.RemoveAssignment(letter)
							s.save()
							return
						}
						next := lastLetter(key)
						if next == "" || next == letter {
							return
						}
						s.settings.Assign(next, bundle, name)
						s.save()
					},
					TestID: "assign-key-" + bundle,
					Style:  core.Style{Width: core.Pt(38), TextAlign: core.TextAlignCenter},
				}
				rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(4, 0)}, Children: []core.Node{
					keyField,
					core.View{Style: core.Style{FlexGrow: 1, Gap: 1}, Children: []core.Node{
						ui.Text(name, 13, ui.Medium, c.Text),
						ui.Text(bundle, 10, ui.Regular, c.Muted),
					}},
					ui.Button{Text: s.tr("common.remove"), Variant: ui.ButtonGhost, OnPress: func() {
						s.settings.RemoveAssignment(letter)
						s.save()
					}},
				}})
			}
			return core.View{Style: core.Style{Gap: 4}, Children: rows}
		}),
		s.assignPicker(cx, c),
	)
	children = append(children, assignCard)

	ignoredCard := card(
		ui.Text(s.tr("ignored.title"), 13, ui.Semibold, c.Text),
		ui.Text(s.tr("ignored.detail"), 11, ui.Regular, c.Muted),
		core.Dynamic(func(cx *core.Scope) core.Node {
			_ = s.version.Get()
			rows := []core.Node{}
			if len(s.settings.Ignored) == 0 {
				rows = append(rows, ui.Text(s.tr("ignored.empty"), 11, ui.Regular, c.Muted))
			}
			for _, bundle := range s.settings.Ignored {
				b := bundle
				rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
					ui.Text(b, 12, ui.Regular, c.Text),
					core.View{Style: core.Style{FlexGrow: 1}},
					ui.Button{Text: s.tr("common.remove"), Variant: ui.ButtonGhost, OnPress: func() {
						s.settings.Unignore(b)
						s.save()
					}},
				}})
			}
			return core.View{Style: core.Style{Gap: 4}, Children: rows}
		}),
		core.View{Style: core.Style{Direction: core.Row, Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.Input{Value: draftIgnore.Get, OnChange: draftIgnore.Set, Placeholder: "com.example.app", Style: core.Style{FlexGrow: 1}},
			ui.Button{Text: s.tr("common.ignore"), Variant: ui.ButtonSecondary, OnPress: func() {
				bundle := strings.TrimSpace(draftIgnore.Get())
				if bundle == "" {
					return
				}
				s.settings.Ignore(bundle)
				s.save()
				s.refreshPickableApps()
				draftIgnore.Set("")
			}},
		}},
	)
	children = append(children, ignoredCard)

	children = append(children, ui.Text(s.tr("footer"), 10, ui.Regular, c.Muted))

	return core.Scroll{Style: core.Style{FlexGrow: 1, Background: c.Background()}, Children: []core.Node{
		core.View{Style: core.Style{Padding: core.All(24), Gap: 14}, Children: children},
	}}
}

// pickRow is one running app the assign picker offers.
type pickRow struct {
	name     string
	bundle   string
	icon     string // base64 PNG, as the helper sends it
	windows  int
	assigned bool
	letter   string // the assigned letter, or the next free one for the name
}

// assignPicker is the "add an assignment" part of the Letter
// assignments card: a search field over the running apps and one row
// per match. Each row's key slot is editable, so typing a key there sets
// that app's key, and pressing the rest of the row gives it the next
// free letter (or removes it when it already has one). The whole picker
// reacts to saves, so it never needs the window rebuilt.
func (s *State) assignPicker(cx *core.Scope, c ui.Colors) core.Node {
	query := core.NewSignal(cx, "")
	note := core.NewSignal(cx, "")
	rows := core.NewMemoFunc(cx, func() []pickRow {
		_ = s.version.Get() // re-run when an assignment saves
		return suggestRows(s.PickableApps(), s.settings, query.Get())
	}, func(x, y []pickRow) bool {
		if len(x) != len(y) {
			return false
		}
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
		return true
	})

	help := ui.Text(s.tr("assign.help"), 11, ui.Regular, c.Muted)
	fieldRow := core.View{Style: core.Style{Direction: core.Row, Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{
		core.Input{Value: query.Get, OnChange: query.Set, Placeholder: s.tr("assign.search"), Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0)}},
		ui.Button{Text: s.tr("common.refresh"), Variant: ui.ButtonGhost, OnPress: s.refreshPickableApps},
	}}
	empty := core.Dynamic(func(*core.Scope) core.Node {
		if len(rows.Get()) == 0 {
			return ui.Text(s.tr("assign.noMatch"), 11, ui.Regular, c.Muted)
		}
		return core.Group(nil)
	})

	return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
		help,
		fieldRow,
		empty,
		core.Scroll{Style: core.Style{MaxHeight: core.Pt(224), Gap: 2}, Children: []core.Node{
			core.Each(rows.Get, func(r pickRow) string { return r.bundle }, func(cx *core.Scope, item func() pickRow, index func() int) core.Node {
				return core.Dynamic(func(*core.Scope) core.Node { return s.pickableRow(cx, c, item, note) })
			}),
		}},
		noteView(note, c),
	}}
}

// noteView shows what the picker last did, beside it rather than in the
// permissions card the shared status lands in.
func noteView(note *core.Signal[string], c ui.Colors) core.Node {
	return core.Dynamic(func(*core.Scope) core.Node {
		if v := note.Get(); v != "" {
			return ui.Text(v, 11, ui.Medium, c.Accent)
		}
		return core.Group(nil)
	})
}

// pickableRow is one app in the assign picker. Its key slot edits that
// app's key directly; pressing the rest of the row assigns the next
// free letter, or removes a key the app already has.
func (s *State) pickableRow(cx *core.Scope, c ui.Colors, item func() pickRow, note *core.Signal[string]) core.Node {
	r := item()
	press := func() {
		r := item()
		// Pressing a row that has a key removes it; an unassigned row gets
		// the next free letter of its name.
		if r.assigned {
			s.settings.RemoveAssignment(r.letter)
			s.save()
			note.Set(s.trf("assign.removed", strings.ToUpper(r.letter), r.name))
			return
		}
		used := s.AssignNext("", r.bundle, r.name)
		if used == "" {
			note.Set(s.tr("assign.noKeysLeft"))
			return
		}
		note.Set(s.trf("assign.set", strings.ToUpper(used), r.name))
	}

	var icon core.Node
	if r.icon != "" {
		if data, err := base64.StdEncoding.DecodeString(r.icon); err == nil {
			icon = core.Image{Source: core.ImageSource{Data: data}, Style: core.Style{Width: core.Pt(22), Height: core.Pt(22)}}
		}
	}

	// The key slot shows the assigned letter, or the one the app would
	// get; typing into it sets that key, and Backspace removes it.
	keySlot := core.Input{
		Value: func() string {
			if r.assigned {
				return strings.ToUpper(r.letter)
			}
			return ""
		},
		Placeholder: strings.ToUpper(r.letter),
		OnKeyPress: func(key string) {
			r := item()
			if key == "Backspace" {
				if r.assigned {
					s.settings.RemoveAssignment(r.letter)
					s.save()
					note.Set(s.trf("assign.removed", strings.ToUpper(r.letter), r.name))
				}
				return
			}
			next := lastLetter(key)
			if next == "" {
				return
			}
			if r.assigned && next == r.letter {
				return
			}
			s.settings.Assign(next, r.bundle, r.name)
			s.save()
			note.Set(s.trf("assign.set", strings.ToUpper(next), r.name))
		},
		TestID: "pick-key-" + r.bundle,
		Style:  core.Style{Width: core.Pt(40), TextAlign: core.TextAlignCenter},
	}

	tail := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		keySlot,
		ui.Textf(func() string {
			if r.assigned {
				return s.tr("assign.removeHint")
			}
			return s.tr("assign.assign")
		}, 10, ui.Regular, c.Muted),
	}}
	return core.View{
		StyleFunc: func() core.Style {
			base := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(6, 8), CornerRadius: 8,
				Background: c.Inset(), BorderWidth: 1, BorderColor: c.Border()}
			if r.assigned {
				base.Background = c.AccentFill()
				base.BorderColor = c.Accent()
			}
			return base
		},
		OnPress: func(core.PressEvent) { press() },
		Children: []core.Node{
			icon,
			core.View{Style: core.Style{FlexGrow: 1, Gap: 1, MinWidth: core.Pt(0)}, Children: []core.Node{
				ui.Text(r.name, 13, ui.Regular, c.Text),
				ui.Textf(func() string {
					if r.windows > 1 {
						return fmt.Sprintf("%s · %s", r.bundle, s.trf("switcher.windows", r.windows))
					}
					return r.bundle
				}, 9, ui.Regular, c.Muted),
			}},
			tail,
		},
	}
}

// SwitcherView is the overlay the trigger key shows. Switching the
// language builds it again, so its text follows at once.
func (s *State) SwitcherView(cx *core.Scope) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		_ = s.lang.Get()
		return s.switcherBody(cx)
	})
}

func (s *State) switcherBody(cx *core.Scope) core.Node {
	c := ui.ColorsOf(cx)
	return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(18), Gap: 12, Background: c.Background()}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			ui.Text(s.tr("switcher.title"), 20, ui.Semibold, c.Text),
			ui.Textf(func() string {
				if v := s.status.Get(); v != "" {
					return v
				}
				return s.tr("switcher.hint")
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
			Empty:         ui.Text(s.tr("switcher.empty"), 12, ui.Regular, c.Muted),
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				// A list builds a cell's content once and then only updates
				// its bindings, so the card has to be built inside a
				// Dynamic: otherwise a letter changed while the app runs
				// stayed as it was on the card, and the panel disagreed
				// with the settings window.
				return core.Dynamic(func(cx *core.Scope) core.Node {
					list := s.entries.Get()
					i := row.Index()
					if i < 0 || i >= len(list) {
						return core.View{}
					}
					return s.entryCard(cx, list[i])
				})
			},
		},
		core.View{Style: core.Style{Direction: core.Row, Gap: 12, Wrap: core.Wrap}, Children: []core.Node{
			ui.Text(s.tr("switcher.optionHint"), 10, ui.Regular, c.Muted),
			ui.Text(s.tr("switcher.shiftHint"), 10, ui.Regular, c.Muted),
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
		kids = append(kids, ui.Text(s.tr("switcher.notRunning"), 10, ui.Regular, muted))
	} else if e.Windows > 1 {
		kids = append(kids, ui.Textf(func() string { return s.trf("switcher.windows", e.Windows) }, 10, ui.Regular, muted))
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

// lastLetter is the last key in v: what a key field keeps when the user
// types over the letter already there, or appends to it by accident.
// An empty v is "", which the caller reads as clearing the key.
func lastLetter(v string) string {
	runes := []rune(strings.ToLower(v))
	for i := len(runes) - 1; i >= 0; i-- {
		if r := runes[i]; r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return string(r)
		}
	}
	return ""
}

// suggestRows builds the picker's rows: the letters assigned already
// stay, the rest get the next free letter of their name in the
// switcher's order, so the letter a row offers is the letter pressing
// that app's key would use. Apps with windows come first, matching
// rcmd's assignment panel; the frontmost app and assigned apps stay
// even without a window, and when no app reports a window at all the
// list falls back to every running app, as it must while Accessibility
// is still refused. query filters by name and bundle id after the
// letters are settled, so searching never changes them.
func suggestRows(apps []helper.App, settings model.Settings, query string) []pickRow {
	used := map[string]bool{}
	assigned := map[string]string{}
	for l, b := range settings.Assignments {
		used[l] = true
		assigned[b] = l
	}
	hasWindows := false
	for _, a := range apps {
		if a.Windows > 0 {
			hasWindows = true
			break
		}
	}
	q := strings.ToLower(query)
	var out []pickRow
	for _, a := range apps {
		if a.BundleID == "" {
			continue
		}
		r := pickRow{name: a.Name, bundle: a.BundleID, icon: a.Icon, windows: a.Windows}
		if l, ok := assigned[a.BundleID]; ok {
			r.assigned, r.letter = true, l
		} else {
			r.letter = nextLetter(a.Name, used)
			if r.letter != "" {
				used[r.letter] = true // reserve, so suggestions do not collide
			}
		}
		if hasWindows && a.Windows == 0 && !a.Active && !r.assigned {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(a.Name), q) && !strings.Contains(strings.ToLower(a.BundleID), q) {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].windows > 0) != (out[j].windows > 0) {
			return out[i].windows > 0
		}
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	return out
}
