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

func TestCpTool_Declaration(t *testing.T) {
	tool := NewCpTool(&events.NoOpPublisher{})
	d := tool.Declaration()

	assert.Equal(t, "copyFile", d.Name)
	assert.Contains(t, d.Description, "workspace")
	params := d.Parameters.Properties
	assert.Contains(t, params, "source")
	assert.Contains(t, params, "destination")
	assert.Contains(t, params, "overwrite")
	assert.Contains(t, params, "_display_message")
	assert.ElementsMatch(t, []string{"source", "destination", "_display_message"}, d.Parameters.Required)
}

func TestCpTool_CopyFile(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("hello"), 0o644))

	handler := NewCpTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "src.txt",
		"destination":      "dst.txt",
		"_display_message": "copying src to dst",
	})
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])

	got, err := os.ReadFile(filepath.Join(workspace, "dst.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestCpTool_CopyDirectoryRecursive(t *testing.T) {
	workspace := t.TempDir()
	srcDir := filepath.Join(workspace, "src")
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("A"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("B"), 0o644))

	handler := NewCpTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "src",
		"destination":      "dst",
		"_display_message": "copying src tree",
	})
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])

	a, err := os.ReadFile(filepath.Join(workspace, "dst", "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "A", string(a))
	b, err := os.ReadFile(filepath.Join(workspace, "dst", "sub", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "B", string(b))
}

func TestCpTool_RejectsSourceOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o644))

	handler := NewCpTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	result, err := handler(ctx, map[string]any{
		"source":           filepath.Join(outside, "secret.txt"),
		"destination":      "leaked.txt",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	assert.Contains(t, result["error"].(string), "outside the workspace")

	_, statErr := os.Stat(filepath.Join(workspace, "leaked.txt"))
	assert.True(t, os.IsNotExist(statErr), "leaked.txt should not exist in workspace")
}

func TestCpTool_RejectsDestinationOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("data"), 0o644))

	handler := NewCpTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "src.txt",
		"destination":      filepath.Join(outside, "leaked.txt"),
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	assert.Contains(t, result["error"].(string), "outside the workspace")
}

func TestCpTool_RefusesOverwriteWithoutFlag(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("new"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "dst.txt"), []byte("old"), 0o644))

	handler := NewCpTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "src.txt",
		"destination":      "dst.txt",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	assert.Contains(t, result["error"].(string), "already exists")

	got, _ := os.ReadFile(filepath.Join(workspace, "dst.txt"))
	assert.Equal(t, "old", string(got), "destination must be untouched without overwrite")
}

func TestCpTool_OverwriteFlag(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("new"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "dst.txt"), []byte("old"), 0o644))

	handler := NewCpTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "src.txt",
		"destination":      "dst.txt",
		"overwrite":        "true",
		"_display_message": "overwriting",
	})
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])

	got, _ := os.ReadFile(filepath.Join(workspace, "dst.txt"))
	assert.Equal(t, "new", string(got))
}

func TestCpTool_RejectsSymlinkSource(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "real.txt"), []byte("data"), 0o644))
	require.NoError(t, os.Symlink("real.txt", filepath.Join(workspace, "link.txt")))

	handler := NewCpTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "link.txt",
		"destination":      "out.txt",
		"_display_message": "symlink should be rejected",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	// Resolver-level rejection: a path with a symlink component is
	// treated the same as a path that escapes the workspace.
	assert.Contains(t, result["error"].(string), "outside the workspace")
}

func TestCpTool_RequiresDisplayMessage(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("x"), 0o644))

	handler := NewCpTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	_, err := handler(ctx, map[string]any{
		"source":      "src.txt",
		"destination": "dst.txt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "_display_message")
}
