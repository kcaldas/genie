package genie

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blockingCapabilityGen struct {
	ai.Gen
}

func (g *blockingCapabilityGen) ModelCapabilities(ctx context.Context, _ ai.Prompt) (ai.ModelCapabilities, error) {
	<-ctx.Done()
	return ai.ModelCapabilities{}, ctx.Err()
}

func TestAttachModelCapabilitiesUsesConfiguredTimeout(t *testing.T) {
	t.Setenv("GENIE_CAPABILITY_DISCOVERY_TIMEOUT", "10ms")
	runner, ok := NewDefaultPromptRunner(&blockingCapabilityGen{}, false).(*DefaultPromptRunner)
	require.True(t, ok)
	prompt := &ai.Prompt{ModelName: "slow-model"}

	started := time.Now()
	runner.attachModelCapabilities(context.Background(), prompt)

	assert.Less(t, time.Since(started), 500*time.Millisecond)
	assert.Nil(t, prompt.ModelCapabilities)
}
