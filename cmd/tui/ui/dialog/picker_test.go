package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

func pickerItems() []PickerItem {
	return []PickerItem{
		{Label: "/compact", Detail: "Summarise the conversation", Value: "/compact"},
		{Label: "/review", Detail: "Review a pull request", Value: "/review"},
		{Label: ":theme", Detail: "Change the colour theme", Value: ":theme"},
		{Label: ":clear", Detail: "Clear the chat", Value: ":clear"},
	}
}

func typeQuery(p *Picker, s string) {
	for _, r := range s {
		p.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestPickerFiltersAndChooses(t *testing.T) {
	p := NewPicker("commands", "Commands", pickerItems())
	p.SetSize(100, 30)
	p.SetAnchor(26)
	th := style.New("default", 1)
	out := ansi.Strip(p.View(th))
	if !strings.Contains(out, "/compact") || !strings.Contains(out, ":clear") || !strings.Contains(out, " Commands ") {
		t.Fatalf("unfiltered view:\n%s", out)
	}
	typeQuery(p, "thm")
	out = ansi.Strip(p.View(th))
	if !strings.Contains(out, ":theme") || strings.Contains(out, "/compact") {
		t.Fatalf("fuzzy filter:\n%s", out)
	}
	cmd, done := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !done {
		t.Fatal("enter closes")
	}
	chosen, ok := cmd().(PickerChosenMsg)
	if !ok || chosen.Kind != "commands" || chosen.Item.Value != ":theme" {
		t.Fatalf("chosen = %#v", cmd())
	}
}

func TestPickerNavigationAndCancel(t *testing.T) {
	p := NewPicker("k", "T", pickerItems())
	p.SetSize(80, 24)
	p.SetAnchor(20)
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	cmd, _ := p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd().(PickerChosenMsg).Item.Value != ":theme" {
		t.Fatal("down twice then tab should pick the third item")
	}
	p = NewPicker("k", "T", pickerItems())
	p.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	cmd, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd().(PickerChosenMsg).Item.Value != ":theme" {
		t.Fatal("end then up should pick the third item")
	}
	p = NewPicker("k", "T", pickerItems())
	p.SetSize(80, 24)
	p.SetAnchor(20)
	typeQuery(p, "zzz")
	if !strings.Contains(ansi.Strip(p.View(style.New("default", 1))), "No matches") {
		t.Fatal("no-match state")
	}
	cmd, done := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !done {
		t.Fatal("enter with no matches cancels")
	}
	if _, ok := cmd().(PickerCancelledMsg); !ok {
		t.Fatal("expected cancel")
	}
	p = NewPicker("k", "T", pickerItems())
	typeQuery(p, "c")
	p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if _, done := p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace}); !done {
		t.Fatal("backspace on an empty query cancels")
	}
	if _, done := p.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); !done {
		t.Fatal("esc cancels")
	}
}

func TestPickerLoadingAndGeometry(t *testing.T) {
	p := NewPicker("files", "Files", nil)
	p.SetSize(120, 40)
	p.SetAnchor(35)
	th := style.New("default", 1)
	if !strings.Contains(ansi.Strip(p.View(th)), "Loading") {
		t.Fatal("nil items render as loading")
	}
	r := p.Rect()
	if r.Y+r.H != 35 || r.W != 100 || r.X != 10 {
		t.Fatalf("loading rect = %+v", r)
	}
	var many []PickerItem
	for i := 0; i < 30; i++ {
		many = append(many, PickerItem{Label: "file" + string(rune('a'+i%26)) + ".go", Value: "x"})
	}
	p.SetItems(many)
	r = p.Rect()
	if r.H != pickerRows+3 || r.Y+r.H != 35 {
		t.Fatalf("full rect = %+v", r)
	}
	for i := 0; i < 15; i++ {
		p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	out := ansi.Strip(p.View(th))
	if lines := strings.Split(out, "\n"); len(lines) != pickerRows+3 {
		t.Fatalf("view rows = %d", len(lines))
	}
	p.SetAnchor(5)
	if r := p.Rect(); r.Y != 0 || r.H != 5 {
		t.Fatalf("picker clamps to the space above the anchor: %+v", r)
	}
}
