package genai

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	googlegenai "google.golang.org/genai"
)

func TestDiscoverModelCapabilitiesUsesInputOnlyLimit(t *testing.T) {
	client := &Client{
		Config: config.NewConfigManager(), Backend: BackendGeminiAPI,
		getModelFn: func(context.Context, string) (*googlegenai.Model, error) {
			return &googlegenai.Model{Name: "models/gemini-test", InputTokenLimit: 1_048_576, OutputTokenLimit: 65_536, Thinking: true}, nil
		},
	}

	caps, err := client.DiscoverModelCapabilities(context.Background(), "gemini-test")
	require.NoError(t, err)
	assert.Equal(t, "gemini-test", caps.Model)
	assert.Equal(t, 1_048_576, caps.InputTokenLimit)
	assert.Equal(t, 65_536, caps.OutputTokenLimit)
	assert.False(t, caps.SharedContextWindow)
	assert.Nil(t, caps.InputModalities)
	assert.True(t, caps.SupportsReasoning)
	assert.Equal(t, ai.CapabilitySourceProvider, caps.Source)
}
