package presentation

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
)

var update = flag.Bool("update", false, "rewrite golden files")

const goldenMarkdown = `# Heading

Some *emphasis* and **strong** text with ` + "`code`" + `.

- first
- second

` + "```go\nfunc main() {}\n```\n"

func goldenConfig(theme string) *types.Config {
	return &types.Config{
		Theme:             theme,
		GlamourTheme:      "auto",
		MarkdownRendering: "enabled",
		WrapMessages:      "enabled",
		UserLabel:         "○",
		AssistantLabel:    "●",
		SystemLabel:       "◆",
		ErrorLabel:        "✗",
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create): %v", path, err)
	}
	if string(want) != got {
		t.Errorf("output differs from %s\n--- want\n%q\n--- got\n%q", path, want, got)
	}
}

// TestGoldenMarkdownMessage pins the rendered form of one markdown message
// under two themes, so a glamour or theme change shows up as a diff.
func TestGoldenMarkdownMessage(t *testing.T) {
	for _, theme := range []string{"default", "dracula"} {
		t.Run(theme, func(t *testing.T) {
			f, err := NewMessageFormatter(goldenConfig(theme), GetThemeForMode(theme, "true"))
			if err != nil {
				t.Fatalf("NewMessageFormatter: %v", err)
			}
			got := f.FormatMessageWithWidth(types.Message{
				Role:        "assistant",
				Content:     goldenMarkdown,
				ContentType: "markdown",
			}, 60)
			assertGolden(t, "markdown_"+theme, got)
		})
	}
}

func TestFormatterRejectsUnknownGlamourStyle(t *testing.T) {
	cfg := goldenConfig("default")
	cfg.GlamourTheme = "no-such-style"
	if _, err := NewMessageFormatter(cfg, GetThemeForMode("default", "true")); err == nil {
		t.Fatal("expected an error for an unknown glamour style")
	}
}

func TestFormatterReusesRendererPerWidth(t *testing.T) {
	f, err := NewMessageFormatter(goldenConfig("default"), GetThemeForMode("default", "true"))
	if err != nil {
		t.Fatal(err)
	}
	msg := types.Message{Role: "assistant", Content: "hello **there**", ContentType: "markdown"}
	f.FormatMessageWithWidth(msg, 40)
	f.FormatMessageWithWidth(msg, 40)
	f.FormatMessageWithWidth(msg, 100)
	// 80 (validation) + 38 + 98
	if n := len(f.renderers); n != 3 {
		t.Fatalf("expected 3 cached renderers, got %d", n)
	}
}

func TestFormatterIsSafeForConcurrentUse(t *testing.T) {
	f, err := NewMessageFormatter(goldenConfig("default"), GetThemeForMode("default", "true"))
	if err != nil {
		t.Fatal(err)
	}
	msg := types.Message{Role: "assistant", Content: "# a\n\ntext", ContentType: "markdown"}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			out := f.FormatMessageWithWidth(msg, 30+w)
			if !strings.Contains(out, "text") {
				t.Errorf("render lost content: %q", out)
			}
		}(i)
	}
	wg.Wait()
}

func TestPlainMessageIsWrappedWhenMarkdownDisabled(t *testing.T) {
	cfg := goldenConfig("default")
	cfg.MarkdownRendering = "disabled"
	f, err := NewMessageFormatter(cfg, GetThemeForMode("default", "true"))
	if err != nil {
		t.Fatal(err)
	}
	out := f.FormatMessageWithWidth(types.Message{
		Role:    "user",
		Content: strings.Repeat("word ", 20),
	}, 30)
	for _, line := range strings.Split(out, "\n") {
		if len(stripANSI(line)) > 30 {
			t.Errorf("line longer than width: %q", line)
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		case r == '\x1b':
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
