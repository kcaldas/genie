package style

import (
	"testing"

	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
)

// TestEveryThemeRendersAFrame guards against a theme table entry with a
// bad colour: every shipped theme must build styles and draw a frame.
func TestEveryThemeRendersAFrame(t *testing.T) {
	for _, name := range presentation.GetThemeNames() {
		th := New(name, 1)
		if th.Name != name {
			t.Errorf("theme %q: name not kept", name)
		}
		out := th.RenderFrame(layout.Rect{W: 20, H: 4}, Frame{Title: " T ", Border: true, Focused: true, Accent: true}, "x")
		if out == "" {
			t.Errorf("theme %q rendered nothing", name)
		}
	}
}
