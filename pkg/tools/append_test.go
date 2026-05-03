package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendTool_CreatesFileIfMissing(t *testing.T) {
	workspace := t.TempDir()
	handler := NewAppendTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "log.txt",
		"content":          "line 1\n",
		"_display_message": "appending",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	got, err := os.ReadFile(filepath.Join(workspace, "log.txt"))
	require.NoError(t, err)
	assert.Equal(t, "line 1\n", string(got))
}

func TestAppendTool_AppendsToExistingFile(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "log.txt"), []byte("first\n"), 0o644))

	handler := NewAppendTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "log.txt",
		"content":          "second\n",
		"_display_message": "appending",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	got, err := os.ReadFile(filepath.Join(workspace, "log.txt"))
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", string(got))
}

func TestAppendTool_DoesNotInsertNewlineImplicitly(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"), []byte("a"), 0o644))

	handler := NewAppendTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	_, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"content":          "b",
		"_display_message": "appending",
	})
	require.NoError(t, err)

	got, _ := os.ReadFile(filepath.Join(workspace, "x.txt"))
	assert.Equal(t, "ab", string(got))
}

func TestAppendTool_RejectsDirectoryTarget(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "d"), 0o755))

	handler := NewAppendTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "d",
		"content":          "data",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "directory")
}

func TestAppendTool_RejectsReadOnly(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "README.md"), []byte("docs"), 0o644))

	handler := NewAppendTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)
	ctx = context.WithValue(ctx, "read_only_paths", []string{"README.md"})

	r, err := handler(ctx, map[string]any{
		"path":             "README.md",
		"content":          "more\n",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "read-only")
	got, _ := os.ReadFile(filepath.Join(workspace, "README.md"))
	assert.Equal(t, "docs", string(got))
}

func TestAppendTool_AutoCreatesParentDirs(t *testing.T) {
	workspace := t.TempDir()
	handler := NewAppendTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "logs/2026-05/today.txt",
		"content":          "hello\n",
		"_display_message": "appending nested",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	got, err := os.ReadFile(filepath.Join(workspace, "logs", "2026-05", "today.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(got))
}
