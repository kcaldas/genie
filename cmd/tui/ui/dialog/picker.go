package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/sahilm/fuzzy"
)

// pickerRows is the most result rows a picker shows at once.
const pickerRows = 10

// PickerItem is one choice in a Picker.
type PickerItem struct {
	Label  string // what is matched and shown
	Detail string // shown dimmed to the right
	Value  string // what is returned when chosen
}

// PickerChosenMsg carries the chosen item. Kind is whatever the opener set.
type PickerChosenMsg struct {
	Kind string
	Item PickerItem
}

// PickerCancelledMsg says the picker closed without a choice. Query is
// what was typed into it, so the opener can keep it.
type PickerCancelledMsg struct {
	Kind  string
	Query string
}

// Picker is a fuzzy-filtered list that floats just above the prompt: the
// command palette and the file picker. Typing refines the query, arrows
// move, Enter or Tab chooses, Esc cancels. Backspace on an empty query
// cancels too, so the key that opened it can be unwound by typing.
type Picker struct {
	kind    string
	title   string
	items   []PickerItem
	labels  []string
	query   string
	matches []int // indexes into items, best first
	cursor  int
	loading bool

	w, h   int
	anchor int // screen row the picker must end above
}

// NewPicker builds a picker over items. A nil items list renders as
// loading until SetItems is called.
func NewPicker(kind, title string, items []PickerItem) *Picker {
	p := &Picker{kind: kind, title: title, loading: items == nil}
	p.SetItems(items)
	return p
}

// Kind returns the opener's tag.
func (p *Picker) Kind() string { return p.kind }

// SetTitle changes the frame title, e.g. to flag a truncated list.
func (p *Picker) SetTitle(title string) { p.title = title }

// Query returns the text typed into the picker.
func (p *Picker) Query() string { return p.query }

// SetItems replaces the choices and refilters.
func (p *Picker) SetItems(items []PickerItem) {
	p.items = items
	p.loading = items == nil
	p.labels = make([]string, len(items))
	for i, it := range items {
		p.labels[i] = it.Label
	}
	p.filter()
}

// SetAnchor sets the screen row the picker sits above (the prompt's top).
func (p *Picker) SetAnchor(row int) { p.anchor = row }

// SetSize implements Dialog.
func (p *Picker) SetSize(w, h int) { p.w, p.h = w, h }

// Fullscreen implements Dialog.
func (p *Picker) Fullscreen() bool { return false }

// ShowsStatus implements Dialog.
func (p *Picker) ShowsStatus() bool { return true }

// Cursor implements Dialog: the picker draws its own query cursor.
func (p *Picker) Cursor() *tea.Cursor { return nil }

// filter recomputes matches for the query, fuzzy when it is non-empty.
func (p *Picker) filter() {
	p.matches = p.matches[:0]
	if p.query == "" {
		for i := range p.items {
			p.matches = append(p.matches, i)
		}
	} else {
		for _, m := range fuzzy.Find(p.query, p.labels) {
			p.matches = append(p.matches, m.Index)
		}
	}
	p.cursor = min(p.cursor, max(len(p.matches)-1, 0))
}

// Update implements Dialog.
func (p *Picker) Update(msg tea.Msg) (tea.Cmd, bool) {
	if paste, ok := msg.(tea.PasteMsg); ok {
		p.query += strings.Join(strings.Fields(paste.Content), " ")
		p.cursor = 0
		p.filter()
		return nil, false
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil, false
	}
	switch k.String() {
	case "esc", "ctrl+c":
		return p.cancel(), true
	case "enter", "tab":
		if len(p.matches) == 0 {
			return p.cancel(), true
		}
		item := p.items[p.matches[p.cursor]]
		kind := p.kind
		return func() tea.Msg { return PickerChosenMsg{Kind: kind, Item: item} }, true
	case "up", "ctrl+p":
		p.cursor = max(p.cursor-1, 0)
	case "down", "ctrl+n":
		p.cursor = min(p.cursor+1, max(len(p.matches)-1, 0))
	case "pgup":
		p.cursor = max(p.cursor-pickerRows, 0)
	case "pgdown":
		p.cursor = min(p.cursor+pickerRows, max(len(p.matches)-1, 0))
	case "home":
		p.cursor = 0
	case "end":
		p.cursor = max(len(p.matches)-1, 0)
	case "backspace":
		if p.query == "" {
			return p.cancel(), true
		}
		r := []rune(p.query)
		p.query = string(r[:len(r)-1])
		p.filter()
	case "ctrl+w":
		p.query = ""
		p.filter()
	default:
		if k.Text != "" && !strings.ContainsAny(k.Text, "\n\r") {
			p.query += k.Text
			p.cursor = 0
			p.filter()
		}
	}
	return nil, false
}

func (p *Picker) cancel() tea.Cmd {
	kind, query := p.kind, p.query
	return func() tea.Msg { return PickerCancelledMsg{Kind: kind, Query: query} }
}

// Rect is where the picker sits: full width minus a margin, anchored so
// its bottom edge touches the row above the prompt.
func (p *Picker) Rect() layout.Rect {
	rows := p.visibleRows()
	h := rows + 1 + 2 // results, query line, border
	w := max(min(p.w-4, 100), 0)
	y := p.anchor - h
	if y < 0 {
		y, h = 0, min(h, p.anchor)
	}
	return layout.Rect{X: (p.w - w) / 2, Y: y, W: w, H: max(h, 0)}
}

func (p *Picker) visibleRows() int {
	if p.loading || len(p.matches) == 0 {
		return 1
	}
	return min(len(p.matches), pickerRows)
}

// View implements Dialog.
func (p *Picker) View(th *style.Theme) string {
	r := p.Rect()
	if r.Empty() {
		return ""
	}
	innerW := max(r.W-2, 1)
	lines := []string{th.Secondary.Render("› ") + p.query + th.Ghost.Render("▏")}
	switch {
	case p.loading:
		lines = append(lines, th.TextTertiary.Render("Loading…"))
	case len(p.matches) == 0:
		lines = append(lines, th.TextTertiary.Render("No matches"))
	default:
		rows := p.visibleRows()
		first := min(max(p.cursor-rows+1, 0), max(len(p.matches)-rows, 0))
		if p.cursor < first {
			first = p.cursor
		}
		for i := first; i < min(first+rows, len(p.matches)); i++ {
			lines = append(lines, p.row(th, p.items[p.matches[i]], innerW, i == p.cursor))
		}
	}
	return th.RenderFrame(r, style.Frame{Title: " " + p.title + " ", Subtitle: " ↑↓ Enter · Esc ", Border: true, Focused: true},
		strings.Join(lines, "\n"))
}

// row renders one result: label left, detail right-aligned and dimmed,
// the whole row reversed when selected.
func (p *Picker) row(th *style.Theme, it PickerItem, width int, selected bool) string {
	label := ansi.Truncate(it.Label, max(width-2, 1), "…")
	detail := ""
	if room := width - ansi.StringWidth(label) - 3; room > 8 && it.Detail != "" {
		detail = ansi.Truncate(it.Detail, room, "…")
	}
	pad := max(width-ansi.StringWidth(label)-ansi.StringWidth(detail)-2, 1)
	line := " " + label + strings.Repeat(" ", pad) + th.TextTertiary.Render(detail) + " "
	if selected {
		return th.Primary.Reverse(true).Render(ansi.Strip(line))
	}
	return line
}
