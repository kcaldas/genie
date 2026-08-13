package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func anthropicMetadata(t *testing.T, raw string) *anthropic_sdk.ModelInfo {
	t.Helper()
	var metadata anthropic_sdk.ModelInfo
	require.NoError(t, json.Unmarshal([]byte(raw), &metadata))
	return &metadata
}

func TestDiscoverModelCapabilitiesUsesCompleteCapabilityShape(t *testing.T) {
	client := &Client{config: config.NewConfigManager(), getModelFn: func(context.Context, string) (*anthropic_sdk.ModelInfo, error) {
		return anthropicMetadata(t, `{
          "id":"claude-test","type":"model","display_name":"Claude Test",
          "created_at":"2026-08-01T00:00:00Z","max_input_tokens":1000000,
          "max_tokens":128000,"capabilities":{"image_input":{"supported":true},"pdf_input":{"supported":true},"thinking":{"supported":true}}
        }`), nil
	}}

	caps, err := client.DiscoverModelCapabilities(context.Background(), "claude-test")
	require.NoError(t, err)
	assert.Equal(t, 1_000_000, caps.InputTokenLimit)
	assert.Equal(t, 128_000, caps.OutputTokenLimit)
	assert.True(t, caps.SharedContextWindow)
	assert.True(t, caps.SupportsInput(ai.ModalityImage))
	assert.True(t, caps.SupportsInput(ai.ModalityDocument))
	assert.True(t, caps.SupportsReasoning)
}

func TestDiscoverModelCapabilitiesLeavesPartialModalitiesUnknown(t *testing.T) {
	client := &Client{config: config.NewConfigManager(), getModelFn: func(context.Context, string) (*anthropic_sdk.ModelInfo, error) {
		return anthropicMetadata(t, `{
          "id":"claude-test","type":"model","display_name":"Claude Test",
          "created_at":"2026-08-01T00:00:00Z","max_input_tokens":200000,
          "max_tokens":64000,"capabilities":{"image_input":{"supported":true}}
        }`), nil
	}}

	caps, err := client.DiscoverModelCapabilities(context.Background(), "claude-test")
	require.NoError(t, err)
	assert.Nil(t, caps.InputModalities)
}
