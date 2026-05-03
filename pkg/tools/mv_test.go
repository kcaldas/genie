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

func TestMvTool_Declaration(t *testing.T) {
	tool := NewMvTool(&events.NoOpPublisher{})
	d := tool.Declaration()

	assert.Equal(t, "moveFile", d.Name)
	assert.Contains(t, d.Description, "workspace")
	params := d.Parameters.Properties
	assert.Contains(t, params, "source")
	assert.Contains(t, params, "destination")
	assert.Contains(t, params, "overwrite")
	assert.Contains(t, params, "_display_message")
	assert.ElementsMatch(t, []string{"source", "destination", "_display_message"}, d.Parameters.Required)
}

func TestMvTool_RenameFile(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("payload"), 0o644))

	handler := NewMvTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "src.txt",
		"destination":      "renamed.txt",
		"_display_message": "renaming src",
	})
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])

	_, statErr := os.Stat(filepath.Join(workspace, "src.txt"))
	assert.True(t, os.IsNotExist(statErr), "source should be gone after move")

	got, err := os.ReadFile(filepath.Join(workspace, "renamed.txt"))
	require.NoError(t, err)
	assert.Equal(t, "payload", string(got))
}

func TestMvTool_RejectsSourceOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o644))

	handler := NewMvTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	result, err := handler(ctx, map[string]any{
		"source":           filepath.Join(outside, "secret.txt"),
		"destination":      "leaked.txt",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	assert.Contains(t, result["error"].(string), "outside the workspace")

	// Source must still exist outside
	_, err = os.Stat(filepath.Join(outside, "secret.txt"))
	assert.NoError(t, err, "outside source must not be touched on rejection")
}

func TestMvTool_RejectsDestinationOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("data"), 0o644))

	handler := NewMvTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "src.txt",
		"destination":      filepath.Join(outside, "leaked.txt"),
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	assert.Contains(t, result["error"].(string), "outside the workspace")

	// Source must remain in workspace because the move was rejected
	_, err = os.Stat(filepath.Join(workspace, "src.txt"))
	assert.NoError(t, err, "source must not be moved when destination is rejected")
}

func TestMvTool_RefusesOverwriteWithoutFlag(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("new"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "dst.txt"), []byte("old"), 0o644))

	handler := NewMvTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "src.txt",
		"destination":      "dst.txt",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	assert.Contains(t, result["error"].(string), "already exists")

	got, _ := os.ReadFile(filepath.Join(workspace, "dst.txt"))
	assert.Equal(t, "old", string(got))
	// Source must be intact
	_, err = os.Stat(filepath.Join(workspace, "src.txt"))
	assert.NoError(t, err)
}

func TestMvTool_OverwriteFlag(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("new"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "dst.txt"), []byte("old"), 0o644))

	handler := NewMvTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

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
	_, statErr := os.Stat(filepath.Join(workspace, "src.txt"))
	assert.True(t, os.IsNotExist(statErr), "source should be gone after overwriting move")
}

func TestMvTool_RejectsSymlinkSource(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "real.txt"), []byte("data"), 0o644))
	require.NoError(t, os.Symlink("real.txt", filepath.Join(workspace, "link.txt")))

	handler := NewMvTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	result, err := handler(ctx, map[string]any{
		"source":           "link.txt",
		"destination":      "moved.txt",
		"_display_message": "symlink should be rejected",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	// Resolver-level rejection: a path with a symlink component is
	// treated the same as a path that escapes the workspace.
	assert.Contains(t, result["error"].(string), "outside the workspace")

	// Symlink must remain in place after rejection
	_, err = os.Lstat(filepath.Join(workspace, "link.txt"))
	assert.NoError(t, err)
}

func TestMvTool_RequiresDisplayMessage(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("x"), 0o644))

	handler := NewMvTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	_, err := handler(ctx, map[string]any{
		"source":      "src.txt",
		"destination": "dst.txt",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "_display_message")
}
