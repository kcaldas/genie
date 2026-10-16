package style

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
)

func plainLines(s string) []string {
	return strings.Split(ansi.Strip(s), "\n")
}

func TestRenderFrameHasExactSize(t *testing.T) {
	th := New("default", 1)
	out := th.RenderFrame(layout.Rect{W: 13, H: 4}, Frame{Border: true, Title: " Chat ", Subtitle: " F4 "}, "hello\nworld\nthird\nfourth")
	lines := plainLines(out)
	if len(lines) != 4 {
		t.Fatalf("height = %d, want 4:\n%s", len(lines), ansi.Strip(out))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 13 {
			t.Errorf("line %d width = %d, want 13: %q", i, w, l)
		}
	}
	if lines[0] != "╭ Chat ─ F4 ╮" {
		t.Errorf("top edge = %q", lines[0])
	}
	if lines[1] != "│hello      │" || lines[2] != "│world      │" {
		t.Errorf("content rows = %q %q", lines[1], lines[2])
	}
	if lines[3] != "╰───────────╯" {
		t.Errorf("bottom edge = %q", lines[3])
	}
}

func TestRenderFrameTruncatesLongLinesAndTitles(t *testing.T) {
	th := New("dracula", 1)
	out := th.RenderFrame(layout.Rect{W: 8, H: 3}, Frame{Border: true, Title: " A very long title "}, "0123456789")
	lines := plainLines(out)
	if lines[0] != "╭ A ver╮" {
		t.Errorf("title not truncated: %q", lines[0])
	}
	if lines[1] != "│012345│" {
		t.Errorf("content not truncated: %q", lines[1])
	}
}

func TestRenderFrameWithoutBorderClampsContent(t *testing.T) {
	th := New("default", 1)
	out := th.RenderFrame(layout.Rect{W: 5, H: 2}, Frame{}, "ab")
	if out != "ab   \n     " {
		t.Fatalf("got %q", out)
	}
}

func TestRenderFrameDegenerateRects(t *testing.T) {
	th := New("default", 1)
	if out := th.RenderFrame(layout.Rect{}, Frame{Border: true}, "x"); out != "" {
		t.Fatalf("empty rect rendered %q", out)
	}
	out := th.RenderFrame(layout.Rect{W: 1, H: 1}, Frame{Border: true}, "x")
	if ansi.Strip(out) != " " {
		t.Fatalf("1x1 bordered rect = %q", out)
	}
}

func TestRenderFrameKeepsAnsiInContent(t *testing.T) {
	th := New("default", 1)
	red := "\x1b[31mred\x1b[0m"
	out := th.RenderFrame(layout.Rect{W: 7, H: 3}, Frame{Border: true}, red)
	if !strings.Contains(out, "\x1b[31mred") {
		t.Fatal("content styling was lost")
	}
	if l := plainLines(out)[1]; l != "│red  │" {
		t.Fatalf("content row = %q", l)
	}
}

func TestNewFallsBackForUnknownTheme(t *testing.T) {
	a, b := New("no-such-theme", 3), New("default", 3)
	if a.Border.String() != b.Border.String() {
		t.Fatal("unknown theme should resolve to the default colours")
	}
}
