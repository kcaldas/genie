package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolvePath_RejectsSymlinkParent demonstrates that the
// parent-symlink bypass is closed. Without the symlink-component
// check, a path string-validated as inside the workspace could escape
// to a real location outside it via a symlink directory, and any tool
// using ResolvePathWithWorkingDirectory would happily operate there.
func TestResolvePath_RejectsSymlinkParent(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	// Plant a symlink inside the workspace pointing to an outside dir.
	require.NoError(t, os.Symlink(outside, filepath.Join(workspace, "escape")))

	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	// A path under the symlink directory must be rejected even though
	// the path string is inside the workspace.
	_, valid := ResolvePathWithWorkingDirectory(ctx, "escape/secret.txt")
	assert.False(t, valid, "path traversal through symlink directory must be rejected")
}

func TestResolvePath_RejectsSymlinkLeaf(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "real.txt"), []byte("data"), 0o644))
	require.NoError(t, os.Symlink("real.txt", filepath.Join(workspace, "link.txt")))

	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	_, valid := ResolvePathWithWorkingDirectory(ctx, "link.txt")
	assert.False(t, valid, "symlink leaf must be rejected by the resolver")
}

func TestResolvePath_AcceptsRealPathsAndNonExistentLeaves(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "sub", "real.txt"), []byte("data"), 0o644))

	ctx := toolctx.WithWorkingDir(context.Background(), workspace)

	// Real existing file
	_, valid := ResolvePathWithWorkingDirectory(ctx, "sub/real.txt")
	assert.True(t, valid, "real file must resolve")

	// Non-existent leaf with real ancestor (typical write destination)
	_, valid = ResolvePathWithWorkingDirectory(ctx, "sub/new.txt")
	assert.True(t, valid, "non-existent leaf with real ancestors must resolve")

	// Non-existent intermediate path (writeFile auto-creates parents)
	_, valid = ResolvePathWithWorkingDirectory(ctx, "newsub/new.txt")
	assert.True(t, valid, "non-existent intermediate dir must resolve")
}
