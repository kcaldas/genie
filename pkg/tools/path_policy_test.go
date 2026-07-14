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

func contextWithPolicy(workspace string, denied, readOnly []string) context.Context {
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)
	ctx = toolctx.WithDeniedPaths(ctx, denied)
	ctx = toolctx.WithReadOnlyPaths(ctx, readOnly)
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

// TestCheckPathPolicy_AllowedDirRootedPattern_OwnerAndUser is the
// load-bearing test for the "owner-managed shared folder is read-only
// for users" hosted-agent shape. The pattern "shared/**" must match
// files in an allowed_dir whose basename is "shared", regardless of
// whether the active workspace is the owner's root or a per-user nested
// workspace that has to walk up to reach the same allowed_dir.
//
// Without the allowed-dir-rooted matching candidate, filepath.Rel for
// the user case produces "../../shared/<file>" which the workspace-
// relative pattern "shared/**" cannot match.
func TestCheckPathPolicy_AllowedDirRootedPattern_OwnerAndUser(t *testing.T) {
	root := t.TempDir()
	sharedDir := filepath.Join(root, "shared")
	require.NoError(t, os.MkdirAll(filepath.Join(sharedDir, "library"), 0o755))
	sharedFile := filepath.Join(sharedDir, "library", "lesson.md")
	require.NoError(t, os.WriteFile(sharedFile, []byte("x"), 0o644))

	withAllowedDir := func(workspace string, readOnly []string) context.Context {
		ctx := contextWithPolicy(workspace, nil, readOnly)
		return toolctx.WithAllowedDirs(ctx, []string{sharedDir})
	}

	// Owner: workspace is the root. shared/** matches via the
	// workspace-relative form (existing behavior, regression check).
	ownerCtx := withAllowedDir(root, []string{"shared/**"})
	err := CheckPathPolicy(ownerCtx, sharedFile, IntentMutate)
	require.Error(t, err, "owner mutate on shared file must be rejected")
	assert.Contains(t, err.Error(), "read-only")
	assert.NoError(t, CheckPathPolicy(ownerCtx, sharedFile, IntentRead))

	// User: workspace is a nested per-user dir. Without the allowed-dir-
	// rooted candidate this would silently allow the mutate.
	userWorkspace := filepath.Join(root, "users", "alice")
	require.NoError(t, os.MkdirAll(userWorkspace, 0o755))
	userCtx := withAllowedDir(userWorkspace, []string{"shared/**"})
	err = CheckPathPolicy(userCtx, sharedFile, IntentMutate)
	require.Error(t, err, "user mutate on shared file must be rejected")
	assert.Contains(t, err.Error(), "read-only")
	assert.NoError(t, CheckPathPolicy(userCtx, sharedFile, IntentRead))
}

// TestCheckPathPolicy_DeniedAppliesAcrossAllowedDir verifies the
// allowed-dir-rooted candidate is consulted for denied_paths too, not
// just read_only_paths.
func TestCheckPathPolicy_DeniedAppliesAcrossAllowedDir(t *testing.T) {
	root := t.TempDir()
	sharedDir := filepath.Join(root, "shared")
	require.NoError(t, os.MkdirAll(filepath.Join(sharedDir, "secrets"), 0o755))
	secretFile := filepath.Join(sharedDir, "secrets", "creds.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("x"), 0o644))

	userWorkspace := filepath.Join(root, "users", "alice")
	require.NoError(t, os.MkdirAll(userWorkspace, 0o755))

	ctx := contextWithPolicy(userWorkspace, []string{"shared/secrets/**"}, nil)
	ctx = toolctx.WithAllowedDirs(ctx, []string{sharedDir})

	err := CheckPathPolicy(ctx, secretFile, IntentRead)
	require.Error(t, err, "denied pattern must apply via allowed-dir-rooted form")
	assert.Contains(t, err.Error(), "denied")
}

// TestCheckPathPolicy_AllowedDirCandidateDoesNotShadowUnrelatedPaths
// guards against false positives: paths that have no relation to the
// allowed_dir must keep passing through unchanged.
func TestCheckPathPolicy_AllowedDirCandidateDoesNotShadowUnrelatedPaths(t *testing.T) {
	root := t.TempDir()
	sharedDir := filepath.Join(root, "shared")
	require.NoError(t, os.MkdirAll(sharedDir, 0o755))
	userWorkspace := filepath.Join(root, "users", "alice", "src")
	require.NoError(t, os.MkdirAll(userWorkspace, 0o755))
	unrelated := filepath.Join(userWorkspace, "main.go")
	require.NoError(t, os.WriteFile(unrelated, []byte(""), 0o644))

	ctx := contextWithPolicy(userWorkspace, nil, []string{"shared/**"})
	ctx = toolctx.WithAllowedDirs(ctx, []string{sharedDir})

	assert.NoError(t, CheckPathPolicy(ctx, unrelated, IntentMutate),
		"file outside the allowed_dir must not be flagged by shared/** pattern")
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
