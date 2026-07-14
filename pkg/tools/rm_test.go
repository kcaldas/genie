package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRmTool_RemovesFile(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"), []byte("data"), 0o644))

	handler := NewRmTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"_display_message": "removing x.txt",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	_, statErr := os.Stat(filepath.Join(workspace, "x.txt"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestRmTool_RefusesDirectoryWithoutRecursive(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "d", "nested"), 0o755))

	handler := NewRmTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "d",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "recursive")

	_, statErr := os.Stat(filepath.Join(workspace, "d", "nested"))
	assert.NoError(t, statErr, "directory must be untouched without recursive")
}

func TestRmTool_RecursiveDeletesTree(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "d", "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "d", "a.txt"), []byte("a"), 0o644))

	handler := NewRmTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "d",
		"recursive":        "true",
		"_display_message": "removing tree",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	_, statErr := os.Stat(filepath.Join(workspace, "d"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestRmTool_RefusesToRemoveWorkspaceRoot(t *testing.T) {
	workspace := t.TempDir()

	handler := NewRmTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             ".",
		"recursive":        "true",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "workspace root")
}

func TestRmTool_NonExistentPath(t *testing.T) {
	workspace := t.TempDir()
	handler := NewRmTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "missing.txt",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "does not exist")
}

func TestRmTool_RejectsSymlink(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "real.txt"), []byte("d"), 0o644))
	require.NoError(t, os.Symlink("real.txt", filepath.Join(workspace, "link.txt")))

	handler := NewRmTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "link.txt",
		"_display_message": "symlink should be rejected",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	// The resolver rejects with "outside the workspace"; that's the
	// hardened policy talking. The symlink is still in place.
	_, lstatErr := os.Lstat(filepath.Join(workspace, "link.txt"))
	assert.NoError(t, lstatErr, "symlink must remain in place after rejection")
}

func TestRmTool_RespectsDeniedAndReadOnly(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".mutiro-agent.yaml"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "README.md"), []byte("docs"), 0o644))

	handler := NewRmTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)
	ctx = toolctx.WithDeniedPaths(ctx, []string{".mutiro-agent.yaml"})
	ctx = toolctx.WithReadOnlyPaths(ctx, []string{"README.md"})

	r, err := handler(ctx, map[string]any{
		"path":             ".mutiro-agent.yaml",
		"_display_message": "denied",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "denied")

	r, err = handler(ctx, map[string]any{
		"path":             "README.md",
		"_display_message": "read-only",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "read-only")
}
