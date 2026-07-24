package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiskJSONL_AppendAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.session.jsonl")

	storage, err := NewDiskJSONL(path)
	require.NoError(t, err)

	require.NoError(t, storage.WriteHeader([]byte(`{"type":"session","version":1,"id":"s1"}`)))
	require.NoError(t, storage.AppendEntry([]byte(`{"type":"custom","id":"e1"}`)))
	require.NoError(t, storage.Checkpoint())
	require.NoError(t, storage.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "{\"type\":\"session\",\"version\":1,\"id\":\"s1\"}\n{\"type\":\"custom\",\"id\":\"e1\"}\n", string(data))
}

func TestDiskJSONL_NoDuplicateHeaderOnReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.session.jsonl")

	first, err := NewDiskJSONL(path)
	require.NoError(t, err)
	require.NoError(t, first.WriteHeader([]byte(`{"type":"session","version":1,"id":"s1"}`)))
	require.NoError(t, first.AppendEntry([]byte(`{"type":"custom","id":"e1"}`)))
	require.NoError(t, first.Close())

	second, err := NewDiskJSONL(path)
	require.NoError(t, err)
	require.NoError(t, second.WriteHeader([]byte(`{"type":"session","version":1,"id":"s1"}`)))
	require.NoError(t, second.AppendEntry([]byte(`{"type":"custom","id":"e2"}`)))
	require.NoError(t, second.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := nonEmptyLines(string(data))
	require.Len(t, lines, 3, "header must not be written twice on reopen")
	assert.Contains(t, lines[0], `"type":"session"`)
	assert.Contains(t, lines[1], `"e1"`)
	assert.Contains(t, lines[2], `"e2"`)
}

func TestDiskJSONL_CreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deep", "s.session.jsonl")
	storage, err := NewDiskJSONL(path)
	require.NoError(t, err)
	require.NoError(t, storage.AppendEntry([]byte(`{"type":"custom"}`)))
	require.NoError(t, storage.Close())

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestSpoolCheckpoint_AppendsWithoutCheckpointLeaveTargetUntouched(t *testing.T) {
	spoolDir := t.TempDir()
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "conv.session.jsonl")

	storage, err := NewSpoolCheckpoint(spoolDir, target)
	require.NoError(t, err)

	require.NoError(t, storage.WriteHeader([]byte(`{"type":"session","version":1,"id":"s1"}`)))
	require.NoError(t, storage.AppendEntry([]byte(`{"type":"custom","id":"e1"}`)))

	// Crash-sim: no checkpoint issued. Target must not exist.
	_, err = os.Stat(target)
	assert.True(t, os.IsNotExist(err), "target must be untouched before first checkpoint")
	assertNoTmpResidue(t, targetDir)
}

func TestSpoolCheckpoint_CheckpointPublishesWholeFileAtomically(t *testing.T) {
	spoolDir := t.TempDir()
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "conv.session.jsonl")

	storage, err := NewSpoolCheckpoint(spoolDir, target)
	require.NoError(t, err)

	require.NoError(t, storage.WriteHeader([]byte(`{"type":"session","version":1,"id":"s1"}`)))
	require.NoError(t, storage.AppendEntry([]byte(`{"type":"custom","id":"e1"}`)))
	require.NoError(t, storage.Checkpoint())

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	lines := nonEmptyLines(string(data))
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"type":"session"`)
	assert.Contains(t, lines[1], `"e1"`)
	assertNoTmpResidue(t, targetDir)

	// Second turn: appends land in the target only after the next checkpoint.
	require.NoError(t, storage.AppendEntry([]byte(`{"type":"custom","id":"e2"}`)))
	data, err = os.ReadFile(target)
	require.NoError(t, err)
	assert.Len(t, nonEmptyLines(string(data)), 2, "un-checkpointed appends must not reach the target")

	require.NoError(t, storage.Checkpoint())
	data, err = os.ReadFile(target)
	require.NoError(t, err)
	assert.Len(t, nonEmptyLines(string(data)), 3)
	assertNoTmpResidue(t, targetDir)
	require.NoError(t, storage.Close())
}

func TestSpoolCheckpoint_ReseedsSpoolFromExistingTarget(t *testing.T) {
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "conv.session.jsonl")

	// First pod life: header + one entry, checkpointed.
	firstSpool := t.TempDir()
	first, err := NewSpoolCheckpoint(firstSpool, target)
	require.NoError(t, err)
	require.NoError(t, first.WriteHeader([]byte(`{"type":"session","version":1,"id":"s1"}`)))
	require.NoError(t, first.AppendEntry([]byte(`{"type":"custom","id":"e1"}`)))
	require.NoError(t, first.Checkpoint())
	require.NoError(t, first.Close())

	// Pod restart: fresh spool dir, same target on the mount.
	secondSpool := t.TempDir()
	second, err := NewSpoolCheckpoint(secondSpool, target)
	require.NoError(t, err)

	// Header must be skipped: spool was seeded from the non-empty target.
	require.NoError(t, second.WriteHeader([]byte(`{"type":"session","version":1,"id":"s1"}`)))
	require.NoError(t, second.AppendEntry([]byte(`{"type":"custom","id":"e2"}`)))
	require.NoError(t, second.Checkpoint())
	require.NoError(t, second.Close())

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	lines := nonEmptyLines(string(data))
	require.Len(t, lines, 3, "continuation must preserve prior content and skip duplicate header")
	assert.Contains(t, lines[0], `"type":"session"`)
	assert.Contains(t, lines[1], `"e1"`)
	assert.Contains(t, lines[2], `"e2"`)
}

func TestMemoryStorage_CheckpointCount(t *testing.T) {
	storage := NewMemoryStorage()
	require.NoError(t, storage.WriteHeader([]byte(`{"type":"session"}`)))
	require.NoError(t, storage.AppendEntry([]byte(`{"type":"custom"}`)))
	assert.Equal(t, 0, storage.CheckpointCount())

	require.NoError(t, storage.Checkpoint())
	require.NoError(t, storage.Checkpoint())
	assert.Equal(t, 2, storage.CheckpointCount())
}

func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range splitLines(s) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func assertNoTmpResidue(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	require.NoError(t, err)
	assert.Empty(t, matches, "no temp files may remain next to the target")
}
