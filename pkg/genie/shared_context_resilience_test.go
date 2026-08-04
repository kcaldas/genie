package genie_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/genie/genietest"
)

// TestSharedAgentContextSurvivesReadFailureAfterStart pins the resilience
// contract of the agent-wide shared context file (<genie home>/.genie/AGENTS.md):
// it must be read and cached at Start — the same moment skills are discovered —
// so that a read failure at turn time (permissions, a stale FUSE mount, cwd
// tricks) cannot silently strip agent-wide rules from the prompt. Skills
// already behave this way (boot-time discovery, cached); the shared file must
// match, or a hosted agent can lose its entire rulebook with no error anywhere.
func TestSharedAgentContextSurvivesReadFailureAfterStart(t *testing.T) {
	f := genietest.NewTestFixture(t)

	genieDir := filepath.Join(f.TestDir, ".genie")
	require.NoError(t, os.MkdirAll(genieDir, 0o755))
	marker := "SHARED-CONTEXT-MARKER: no client quotation goes out without approval"
	require.NoError(t, os.WriteFile(
		filepath.Join(genieDir, "AGENTS.md"),
		[]byte("# Desk Manual\n\n"+marker+"\n"), 0o644))

	f.StartAndGetSession()

	// Block reads after Start — the production suspicion for hosted pods.
	sharedFile := filepath.Join(genieDir, "AGENTS.md")
	require.NoError(t, os.Chmod(sharedFile, 0o000))
	t.Cleanup(func() { _ = os.Chmod(sharedFile, 0o644) })

	parts, err := f.Genie.GetContext(context.Background())
	require.NoError(t, err)
	assert.Contains(t, parts["project"], marker,
		"shared agent context read at Start must survive later read failures")
}
