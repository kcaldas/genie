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
