package ctx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/toolctx"
)

func writeAgentsMD(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".genie")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0o644))
}

// TestUserTierSharedContext: $HOME/.genie/AGENTS.md is a shared-context
// tier of its own, mirroring user-tier skills ($HOME/.genie/skills). It
// loads for every session, before the agent-wide file (general before
// specific). Hosted Mutiro agents rely on this symmetry: HOME points at
// the agent state dir, so user-tier skills worked while the genie-home
// tier was broken — context must resolve through the same tiers skills do.
func TestUserTierSharedContext(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	writeAgentsMD(t, userHome, "USER-TIER-MARKER rules\n")

	agentHome := t.TempDir()
	writeAgentsMD(t, agentHome, "PROJECT-TIER-MARKER rules\n")

	provider := NewProjectCtxManager(nil)
	ctx := toolctx.WithGenieHome(context.Background(), agentHome)
	ctx = toolctx.WithWorkingDir(ctx, agentHome)

	part, err := provider.GetPart(ctx)
	require.NoError(t, err)

	assert.Contains(t, part.Content, "USER-TIER-MARKER", "user-tier shared context must load")
	assert.Contains(t, part.Content, "PROJECT-TIER-MARKER", "genie-home shared context must still load")
	assert.Less(t,
		strings.Index(part.Content, "USER-TIER-MARKER"),
		strings.Index(part.Content, "PROJECT-TIER-MARKER"),
		"user tier loads before the agent-wide tier (general before specific)")
}

// TestUserTierSharedContextDedupe: when HOME and genie home are the same
// directory (hosted pods: both /var/agent), the file must load exactly once.
func TestUserTierSharedContextDedupe(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Setenv("HOME", home)
	writeAgentsMD(t, home, "DEDUPE-MARKER rules\n")

	provider := NewProjectCtxManager(nil)
	ctx := toolctx.WithGenieHome(context.Background(), home)
	ctx = toolctx.WithWorkingDir(ctx, home)

	part, err := provider.GetPart(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(part.Content, "DEDUPE-MARKER"),
		"identical HOME and genie home must inject the shared file exactly once")
}
