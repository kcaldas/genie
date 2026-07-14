package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMatchGlob covers the matcher in isolation. The find tool tests
// below cover the walker integration and the policy filter.
func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		// No-slash patterns: basename match at any depth.
		{"*.go", "main.go", true},
		{"*.go", "src/main.go", true},
		{"*.go", "src/util/helpers.go", true},
		{"*.go", "src/util/helpers.txt", false},
		{"*test*", "src/util/helpers_test.go", true},
		{"helpers.go", "src/util/helpers.go", true},

		// Anchored single-component
		{"src/*.go", "src/main.go", true},
		{"src/*.go", "src/util/helpers.go", false},

		// Anchored deep with **
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/util/helpers.go", true},
		{"src/**/*.go", "src/util/sub/x.go", true},
		{"src/**/*.go", "docs/x.go", false},

		// Leading **
		{"**/*_test.go", "src/util/helpers_test.go", true},
		{"**/*_test.go", "test.go", false}, // no _ before test

		// Mid-path **
		{"docs/**/auth.md", "docs/api/auth.md", true},
		{"docs/**/auth.md", "docs/auth.md", true}, // ** matches zero components
		{"docs/**/auth.md", "src/auth.md", false},

		// Exact path
		{"pkg/util/helpers.go", "pkg/util/helpers.go", true},
		{"pkg/util/helpers.go", "pkg/util/other.go", false},

		// Edge cases
		{"", "anything", false},
	}
	for _, tc := range cases {
		t.Run(tc.pattern+"_"+tc.path, func(t *testing.T) {
			got := MatchGlob(tc.pattern, tc.path)
			assert.Equal(t, tc.want, got, "pattern=%q path=%q", tc.pattern, tc.path)
		})
	}
}

func setupFindWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	mustWrite := func(rel, content string) {
		full := filepath.Join(ws, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	mustWrite("main.go", "")
	mustWrite("src/cli.go", "")
	mustWrite("src/util/helpers.go", "")
	mustWrite("src/util/helpers_test.go", "")
	mustWrite("docs/intro.md", "")
	mustWrite("docs/api/auth.md", "")
	mustWrite(".git/HEAD", "")
	mustWrite("README.md", "")
	return ws
}

func TestFindTool_BasenameGlobIsRecursive(t *testing.T) {
	ws := setupFindWorkspace(t)
	handler := NewFindTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), ws)

	r, err := handler(ctx, map[string]any{
		"pattern":          "*.go",
		"_display_message": "go files",
	})
	require.NoError(t, err)
	out := r["results"].(string)

	assert.Contains(t, out, "main.go")
	assert.Contains(t, out, "src/cli.go")
	assert.Contains(t, out, "src/util/helpers.go")
	assert.Contains(t, out, "src/util/helpers_test.go")
	assert.NotContains(t, out, ".md")
}

func TestFindTool_SlashAnchoredSingleComponent(t *testing.T) {
	ws := setupFindWorkspace(t)
	handler := NewFindTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), ws)

	r, err := handler(ctx, map[string]any{
		"pattern":          "src/*.go",
		"_display_message": "direct children of src",
	})
	require.NoError(t, err)
	out := r["results"].(string)

	assert.Contains(t, out, "src/cli.go")
	assert.NotContains(t, out, "src/util/helpers.go", "* must not cross /")
}

func TestFindTool_DoubleStarCrossesDirs(t *testing.T) {
	ws := setupFindWorkspace(t)
	handler := NewFindTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), ws)

	r, err := handler(ctx, map[string]any{
		"pattern":          "src/**/*.go",
		"_display_message": "go files under src",
	})
	require.NoError(t, err)
	out := r["results"].(string)

	assert.Contains(t, out, "src/cli.go")
	assert.Contains(t, out, "src/util/helpers.go")
	assert.Contains(t, out, "src/util/helpers_test.go")
	assert.NotContains(t, out, "docs/")
}

func TestFindTool_TypeFilter(t *testing.T) {
	ws := setupFindWorkspace(t)
	handler := NewFindTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), ws)

	r, err := handler(ctx, map[string]any{
		"pattern":          "**",
		"type":             "directory",
		"_display_message": "directories only",
	})
	require.NoError(t, err)
	out := r["results"].(string)

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" || strings.HasPrefix(line, "...") {
			continue
		}
		assert.True(t, strings.HasSuffix(line, "/"),
			"directory result must end with /, got %q", line)
	}
}

func TestFindTool_DeniedPathsSilentlyFiltered(t *testing.T) {
	ws := setupFindWorkspace(t)
	handler := NewFindTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), ws)
	ctx = toolctx.WithDeniedPaths(ctx, []string{".git/**"})

	r, err := handler(ctx, map[string]any{
		"pattern":          "**/HEAD",
		"_display_message": "should not see denied paths",
	})
	require.NoError(t, err)
	out := r["results"].(string)
	assert.NotContains(t, out, ".git", "denied paths must not appear in results")
}

func TestFindTool_RejectsSymlinkInResults(t *testing.T) {
	ws := setupFindWorkspace(t)
	require.NoError(t, os.Symlink("README.md", filepath.Join(ws, "link.md")))

	handler := NewFindTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), ws)

	r, err := handler(ctx, map[string]any{
		"pattern":          "*.md",
		"_display_message": "md files",
	})
	require.NoError(t, err)
	out := r["results"].(string)
	assert.NotContains(t, out, "link.md", "symlinks must be filtered from results")
	assert.Contains(t, out, "README.md")
}

func TestFindTool_TruncationOnLargeResultSets(t *testing.T) {
	ws := t.TempDir()
	for i := 0; i < findFilesMaxResults+50; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(ws, "f"+intToStr(i)+".txt"), []byte("x"), 0o644))
	}

	handler := NewFindTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), ws)

	r, err := handler(ctx, map[string]any{
		"pattern":          "*.txt",
		"_display_message": "all txt",
	})
	require.NoError(t, err)
	assert.True(t, r["truncated"].(bool))
	assert.Contains(t, r["results"].(string), "(truncated")
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
