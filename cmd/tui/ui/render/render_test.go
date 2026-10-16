package render

import (
	"strings"
	"testing"

	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type cfg struct{ c types.Config }

func (f *cfg) GetConfig() *types.Config { return &f.c }
func (f *cfg) GetTheme() *types.Theme   { return presentation.GetThemeForMode(f.c.Theme, "true") }

// TestEveryThemeCombinationRenders runs markdown and a diff through every
// main theme, glamour style and diff theme the config can name.
func TestEveryThemeCombinationRenders(t *testing.T) {
	const md = "# Title\n\nSome `code` and a list:\n\n- one\n- two\n\n```go\nfunc x() {}\n```\n"
	const diff = "--- a\n+++ b\n@@ -1,2 +1,2 @@\n-old\n+new\n context\n"
	for _, theme := range presentation.GetThemeNames() {
		for _, glamour := range append(presentation.GetAllAvailableGlamourStyles(), "auto") {
			for _, dt := range append(presentation.GetDiffThemeNames(), "auto") {
				c := &cfg{c: types.Config{Theme: theme, GlamourTheme: glamour, DiffTheme: dt}}
				if out := Markdown(c, md, 60); !strings.Contains(out, "Title") || !strings.Contains(out, "func") {
					t.Errorf("theme=%s glamour=%s: markdown lost content: %q", theme, glamour, out)
				}
				if out := Diff(c, diff); !strings.Contains(out, "new") || !strings.Contains(out, "old") {
					t.Errorf("theme=%s diff=%s: diff lost content: %q", theme, dt, out)
				}
			}
		}
	}
}

func TestBodySplitsByType(t *testing.T) {
	c := &cfg{c: types.Config{Theme: "default", GlamourTheme: "auto", DiffTheme: "auto"}}
	if lines := Body(c, "a\nb", "text", 40); len(lines) != 2 || lines[1] != "b" {
		t.Fatalf("text body = %q", lines)
	}
	if lines := Body(c, "", "markdown", 40); len(lines) != 1 || lines[0] != "" {
		t.Fatalf("empty markdown body = %q", lines)
	}
	if lines := Body(c, "-x\n+y", "diff", 40); len(lines) != 2 {
		t.Fatalf("diff body = %q", lines)
	}
	if out := Markdown(c, strings.Repeat("word ", 20000), 60); !strings.Contains(out, "word") {
		t.Fatal("large markdown falls back to ascii but keeps content")
	}
}
