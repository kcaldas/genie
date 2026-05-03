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

func TestReadFileTool_LineRange(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("L1\nL2\nL3\nL4\nL5\n"), 0o644))

	handler := NewReadFileTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"file_path":        "x.txt",
		"start_line":       float64(2),
		"end_line":         float64(4),
		"_display_message": "reading L2-L4",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Equal(t, "L2\nL3\nL4", r["results"].(string))
}

func TestReadFileTool_LineRangeWithLineNumbers(t *testing.T) {
	// When asking for a slice with line numbers, the original numbers
	// should be preserved (not renumbered from 1) so the model can
	// refer back to them in subsequent edits.
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("L1\nL2\nL3\nL4\n"), 0o644))

	handler := NewReadFileTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"file_path":        "x.txt",
		"start_line":       float64(2),
		"end_line":         float64(3),
		"line_numbers":     true,
		"_display_message": "reading numbered slice",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Contains(t, r["results"].(string), "     2\tL2")
	assert.Contains(t, r["results"].(string), "     3\tL3")
}

func TestReadFileTool_PartialRangeRejected(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("L1\nL2\n"), 0o644))

	handler := NewReadFileTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	// start without end → reject. Don't silently read more than the model expected.
	r, err := handler(ctx, map[string]any{
		"file_path":        "x.txt",
		"start_line":       float64(1),
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "both start_line and end_line")
}

func TestReadFileTool_RangeBeyondEOFTruncatesGracefully(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("L1\nL2\nL3\n"), 0o644))

	handler := NewReadFileTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"file_path":        "x.txt",
		"start_line":       float64(2),
		"end_line":         float64(99),
		"_display_message": "reading past end",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Equal(t, "L2\nL3", r["results"].(string))
}

func TestReadFileTool_StartBeyondEOFFails(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("L1\n"), 0o644))

	handler := NewReadFileTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"file_path":        "x.txt",
		"start_line":       float64(99),
		"end_line":         float64(100),
		"_display_message": "out of range",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "exceeds file length")
}

func TestReadFileTool_FullReadStillWorks(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "x.txt"),
		[]byte("L1\nL2\n"), 0o644))

	handler := NewReadFileTool(&events.NoOpPublisher{}).Handler()
	ctx := context.WithValue(context.Background(), "cwd", workspace)

	r, err := handler(ctx, map[string]any{
		"file_path":        "x.txt",
		"_display_message": "no range",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Equal(t, "L1\nL2", r["results"].(string))
}
