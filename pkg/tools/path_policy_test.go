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

func contextWithPolicy(workspace string, denied, readOnly []string) context.Context {
	ctx := context.WithValue(context.Background(), "cwd", workspace)
	ctx = context.WithValue(ctx, "denied_paths", denied)
	ctx = context.WithValue(ctx, "read_only_paths", readOnly)
	return ctx
}

func TestCheckPathPolicy_DeniedBlocksReadAndMutate(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".mutiro-agent.yaml"), []byte("config"), 0o644))

	ctx := contextWithPolicy(workspace, []string{".mutiro-agent.yaml"}, nil)

	resolved, ok := ResolvePathWithWorkingDirectory(ctx, ".mutiro-agent.yaml")
	require.True(t, ok)

	assert.Error(t, CheckPathPolicy(ctx, resolved, IntentRead), "denied path must reject read")
	assert.Error(t, CheckPathPolicy(ctx, resolved, IntentMutate), "denied path must reject mutate")
}

func TestCheckPathPolicy_ReadOnlyBlocksMutateOnly(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "README.md"), []byte("docs"), 0o644))

	ctx := contextWithPolicy(workspace, nil, []string{"README.md"})

	resolved, ok := ResolvePathWithWorkingDirectory(ctx, "README.md")
	require.True(t, ok)

	assert.NoError(t, CheckPathPolicy(ctx, resolved, IntentRead), "read_only path must allow read")
	err := CheckPathPolicy(ctx, resolved, IntentMutate)
	require.Error(t, err, "read_only path must reject mutate")
	assert.Contains(t, err.Error(), "read-only")
}

func TestCheckPathPolicy_GlobAndPrefixPatterns(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".git", "objects"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "secret.yaml"), []byte("k: v"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".git", "HEAD"), []byte("ref"), 0o644))

	ctx := contextWithPolicy(workspace,
		[]string{".git/**", "*.yaml"},
		nil,
	)

	cases := []struct {
		path    string
		blocked bool
	}{
		{".git/HEAD", true},
		{".git/objects", true},
		{".git", true},
		{"secret.yaml", true},
		{"deeper/nested/secret.yaml", true},
		{"plain.txt", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			// We need a real path on disk for some of these so the
			// resolver accepts them. Touch a file when missing.
			abs := filepath.Join(workspace, tc.path)
			_ = os.MkdirAll(filepath.Dir(abs), 0o755)
			if _, err := os.Lstat(abs); os.IsNotExist(err) {
				_ = os.WriteFile(abs, []byte(""), 0o644)
			}

			resolved, ok := ResolvePathWithWorkingDirectory(ctx, tc.path)
			require.True(t, ok, "resolver must accept %q", tc.path)
			err := CheckPathPolicy(ctx, resolved, IntentRead)
			if tc.blocked {
				assert.Error(t, err, "%q should be blocked", tc.path)
			} else {
				assert.NoError(t, err, "%q should not be blocked", tc.path)
			}
		})
	}
}

func TestPolicy_PluggedIntoLsTool(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".mutiro-agent.yaml"), []byte("config"), 0o644))

	handler := NewLsTool(&events.NoOpPublisher{}).Handler()
	ctx := contextWithPolicy(workspace, []string{".mutiro-agent.yaml"}, nil)

	_, err := handler(ctx, map[string]any{
		"path":             ".mutiro-agent.yaml",
		"_display_message": "denied path test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestPolicy_PluggedIntoWriteTool(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "README.md"), []byte("docs"), 0o644))

	handler := NewWriteTool(nil, false).Handler()
	ctx := contextWithPolicy(workspace, nil, []string{"README.md"})

	result, err := handler(ctx, map[string]any{
		"path":    "README.md",
		"content": "rewritten",
	})
	require.NoError(t, err)
	assert.Equal(t, false, result["success"])
	assert.Contains(t, result["results"], "read-only")

	got, _ := os.ReadFile(filepath.Join(workspace, "README.md"))
	assert.Equal(t, "docs", string(got), "read-only file must be untouched")
}

func TestPolicy_PluggedIntoCpAndMv(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "src.txt"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "locked.txt"), []byte("locked"), 0o644))

	cp := NewCpTool(&events.NoOpPublisher{}).Handler()
	mv := NewMvTool(&events.NoOpPublisher{}).Handler()

	ctx := contextWithPolicy(workspace, nil, []string{"locked.txt"})

	// cp: writing to a read-only destination must fail
	r, err := cp(ctx, map[string]any{
		"source":           "src.txt",
		"destination":      "locked.txt",
		"overwrite":        "true",
		"_display_message": "should be blocked",
	})
	require.NoError(t, err)
	assert.Equal(t, false, r["success"])
	assert.Contains(t, r["error"].(string), "read-only")

	// mv: source is mutated (deleted) so a read-only source must fail too
	r, err = mv(ctx, map[string]any{
		"source":           "locked.txt",
		"destination":      "moved.txt",
		"_display_message": "should be blocked",
	})
	require.NoError(t, err)
	assert.Equal(t, false, r["success"])
	assert.Contains(t, r["error"].(string), "read-only")
}
