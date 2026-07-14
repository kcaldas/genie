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

func TestMkdirTool_CreatesNewDir(t *testing.T) {
	workspace := t.TempDir()
	handler := NewMkdirTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "new/subdir",
		"_display_message": "creating subdir",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	info, err := os.Stat(filepath.Join(workspace, "new", "subdir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestMkdirTool_IdempotentForExistingDir(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "existing"), 0o755))

	handler := NewMkdirTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "existing",
		"_display_message": "should be no-op",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Contains(t, r["results"].(string), "already exists")
}

func TestMkdirTool_RejectsExistingFileAtPath(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "blocker"), []byte("x"), 0o644))

	handler := NewMkdirTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "blocker",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "is a file")
}

func TestMkdirTool_OutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	handler := NewMkdirTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             filepath.Join(outside, "nope"),
		"_display_message": "outside",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "outside the workspace")
}
