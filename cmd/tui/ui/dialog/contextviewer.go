package dialog

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

// Width of the context-part list on the left.
const contextKeysWidth = 25

// ContextRefreshMsg asks the owner to reload the context parts; the
// dialog gets them back through SetData.
type ContextRefreshMsg struct{}

// ContextViewer is the full-screen LLM context inspector: a list of
// context parts on the left, the selected part on the right, and a line
// of navigation tips at the bottom.
type ContextViewer struct {
	data     map[string]string
	keys     []string
	selected int
	offset   int
	w, h     int
}

// NewContextViewer builds the viewer over the given context parts.
func NewContextViewer(data map[string]string) *ContextViewer {
	v := &ContextViewer{}
	v.SetData(data)
	return v
}

// SetData replaces the context parts, keeping the selection when the key
// still exists.
func (v *ContextViewer) SetData(data map[string]string) {
	selectedKey := v.selectedKey()
	v.data = data
	v.keys = v.keys[:0]
	for k := range data {
		v.keys = append(v.keys, k)
	}
	sort.Strings(v.keys)
	v.selected = 0
	for i, k := range v.keys {
		if k == selectedKey {
			v.selected = i
		}
	}
	v.offset = 0
}

// SetSize implements Dialog.
func (v *ContextViewer) SetSize(w, h int) { v.w, v.h = w, h }

// Fullscreen implements Dialog.
func (v *ContextViewer) Fullscreen() bool { return true }

// ShowsStatus implements Dialog: the viewer covers the whole screen.
func (v *ContextViewer) ShowsStatus() bool { return false }

// Cursor implements Dialog: the viewer hides the cursor.
func (v *ContextViewer) Cursor() *tea.Cursor { return nil }

// Update implements Dialog.
func (v *ContextViewer) Update(msg tea.Msg) (tea.Cmd, bool) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil, false
	}
	switch k.String() {
	case "esc", "q":
		return nil, true
	case "up", "k":
		if v.selected > 0 {
			v.selected--
			v.offset = 0
		}
	case "down", "j":
		if v.selected < len(v.keys)-1 {
			v.selected++
			v.offset = 0
		}
	case "pgup", "ctrl+u":
		v.offset = max(v.offset-v.contentHeight(), 0)
	case "pgdown", "ctrl+d":
		v.offset = min(v.offset+v.contentHeight(), v.maxOffset())
	case "home":
		v.offset = 0
	case "end":
		v.offset = v.maxOffset()
	case "r":
		return func() tea.Msg { return ContextRefreshMsg{} }, false
	}
	return nil, false
}

func (v *ContextViewer) selectedKey() string {
	if v.selected >= 0 && v.selected < len(v.keys) {
		return v.keys[v.selected]
	}
	return ""
}

func (v *ContextViewer) contentLines() []string {
	key := v.selectedKey()
	if key == "" {
		return nil
	}
	return strings.Split(v.data[key], "\n")
}

func (v *ContextViewer) contentHeight() int { return max(v.h-1-2, 1) }

func (v *ContextViewer) maxOffset() int {
	return max(len(v.contentLines())-v.contentHeight(), 0)
}

// View implements Dialog.
func (v *ContextViewer) View(th *style.Theme) string {
	if v.w <= 0 || v.h <= 0 {
		return ""
	}
	bodyH := v.h - 1
	keysRect := layout.Rect{X: 0, Y: 0, W: min(contextKeysWidth, v.w), H: bodyH}
	contentRect := layout.Rect{X: keysRect.W, Y: 0, W: v.w - keysRect.W, H: bodyH}

	keys := th.RenderFrame(keysRect, style.Frame{Title: " Context Parts ", Border: true}, v.keysView())
	content := th.RenderFrame(contentRect, style.Frame{Title: v.contentTitle(), Border: true}, v.contentView(th))
	tips := style.Fit(th.Secondary.Render(" ↑↓ Navigate | PgUp/PgDn Scroll | Home/End Jump | r Refresh | Esc/q Close"), v.w, 1)

	body := lipgloss.JoinHorizontal(lipgloss.Top, keys, content)
	return lipgloss.JoinVertical(lipgloss.Left, body, tips)
}

func (v *ContextViewer) keysView() string {
	if len(v.keys) == 0 {
		return "  No context data available\n  Press 'r' to refresh"
	}
	var sb strings.Builder
	for i, k := range v.keys {
		marker := "  "
		if i == v.selected {
			marker = "► "
		}
		fmt.Fprintf(&sb, "%s%-20s\n", marker, k)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (v *ContextViewer) contentTitle() string {
	if len(v.keys) == 0 {
		return " No Content "
	}
	return fmt.Sprintf(" {%s} ", v.selectedKey())
}

func (v *ContextViewer) contentView(th *style.Theme) string {
	if len(v.keys) == 0 {
		return th.TextTertiary.Render("No context data available.") + "\n" +
			th.TextTertiary.Render("Press 'r' to refresh context data.")
	}
	lines := v.contentLines()
	if len(lines) == 1 && lines[0] == "" {
		return th.TextTertiary.Render(fmt.Sprintf("No content available for '%s'", v.selectedKey()))
	}
	end := min(v.offset+v.contentHeight(), len(lines))
	out := make([]string, 0, end-v.offset)
	for _, l := range lines[v.offset:end] {
		out = append(out, th.TextTertiary.Render(l))
	}
	return strings.Join(out, "\n")
}
