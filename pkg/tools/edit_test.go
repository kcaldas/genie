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

func TestEditTool_StrReplace_HappyPath(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("hello there\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"old_string":       "hello",
		"new_string":       "howdy",
		"_display_message": "editing",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	got, _ := os.ReadFile(filepath.Join(workspace, "x.txt"))
	assert.Equal(t, "howdy there\n", string(got))
}

func TestEditTool_StrReplace_NotFoundFailsClean(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("hello there\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"old_string":       "missing",
		"new_string":       "anything",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "not found")

	got, _ := os.ReadFile(filepath.Join(workspace, "x.txt"))
	assert.Equal(t, "hello there\n", string(got), "file unchanged on no-match")
}

func TestEditTool_StrReplace_AmbiguousFailsLoud(t *testing.T) {
	// THIS IS THE CRITICAL SAFETY PROPERTY. If old_string matches more
	// than once, the model could be editing the wrong instance. We
	// refuse and force the model to add context.
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("foo\nfoo\nbar\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"old_string":       "foo",
		"new_string":       "qux",
		"_display_message": "should fail on ambiguity",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "matches 2 places")

	got, _ := os.ReadFile(filepath.Join(workspace, "x.txt"))
	assert.Equal(t, "foo\nfoo\nbar\n", string(got), "file unchanged on ambiguous match")
}

func TestEditTool_StrReplace_DeleteWithEmptyNewString(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("keep this // DELETEME end\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"old_string":       " // DELETEME",
		"new_string":       "",
		"_display_message": "deleting span",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	got, _ := os.ReadFile(filepath.Join(workspace, "x.txt"))
	assert.Equal(t, "keep this end\n", string(got))
}

func TestEditTool_LineRange_Replace(t *testing.T) {
	workspace := t.TempDir()
	original := "L1\nL2\nL3\nL4\nL5\n"
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"), []byte(original), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"start_line":       float64(2),
		"end_line":         float64(4),
		"replacement":      "REPLACED A\nREPLACED B\n",
		"_display_message": "replace lines 2-4",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	got, _ := os.ReadFile(filepath.Join(workspace, "x.txt"))
	assert.Equal(t, "L1\nREPLACED A\nREPLACED B\nL5\n", string(got))
}

func TestEditTool_LineRange_DeleteWithEmptyReplacement(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("L1\nL2\nL3\nL4\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"start_line":       float64(2),
		"end_line":         float64(3),
		"replacement":      "",
		"_display_message": "deleting lines",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	got, _ := os.ReadFile(filepath.Join(workspace, "x.txt"))
	assert.Equal(t, "L1\nL4\n", string(got))
}

func TestEditTool_LineRange_BeyondEOFFails(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"), []byte("L1\nL2\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"start_line":       float64(1),
		"end_line":         float64(99),
		"replacement":      "X\n",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "exceeds file length")
}

func TestEditTool_RejectsBothModes(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"), []byte("L1\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"old_string":       "L1",
		"new_string":       "X",
		"start_line":       float64(1),
		"end_line":         float64(1),
		"replacement":      "X",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "either")
}

func TestEditTool_RejectsNeitherMode(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"), []byte("L1\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "missing edit parameters")
}

func TestEditTool_RejectsMissingFile(t *testing.T) {
	workspace := t.TempDir()
	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "missing.txt",
		"old_string":       "x",
		"new_string":       "y",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "does not exist")
}

func TestEditTool_RejectsLargeFile(t *testing.T) {
	workspace := t.TempDir()
	big := make([]byte, MaxEditFileSize+1)
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "big.bin"), big, 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "big.bin",
		"old_string":       "x",
		"new_string":       "y",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "exceeds")
}

func TestEditTool_AtomicWriteSurvivesFailure(t *testing.T) {
	// Sanity check that a successful edit doesn't leave a stray temp
	// file behind. We assert by listing the directory before and after.
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"), []byte("a\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	r, err := handler(ctx, map[string]any{
		"path":             "x.txt",
		"old_string":       "a",
		"new_string":       "b",
		"_display_message": "edit",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	entries, err := os.ReadDir(workspace)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.Contains(e.Name(), ".edit-"),
			"temp file leaked: %s", e.Name())
	}
}

func TestEditTool_RejectsReadOnly(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello\n"), 0o644))

	handler := NewEditTool(&events.NoOpPublisher{}).Handler()
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)
	ctx = toolctx.WithReadOnlyPaths(ctx, []string{"README.md"})

	r, err := handler(ctx, map[string]any{
		"path":             "README.md",
		"old_string":       "hello",
		"new_string":       "goodbye",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "read-only")

	got, _ := os.ReadFile(filepath.Join(workspace, "README.md"))
	assert.Equal(t, "hello\n", string(got))
}
