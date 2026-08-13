package shared

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestModelInputAdmissionLimitUsesReportedInputCeiling(t *testing.T) {
	prompt := ai.Prompt{
		MaxTokens: 128_000,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit:  1_000_000,
			OutputTokenLimit: 128_000,
		},
	}

	assert.Equal(t, 1_000_000, ModelInputAdmissionLimit(prompt))
}

func TestNewLoopConfigKeepsPhysicalLimitSeparate(t *testing.T) {
	prompt := ai.Prompt{
		MaxToolIterations: 7,
		ModelCapabilities: &ai.ModelCapabilities{InputTokenLimit: 1_000_000},
	}

	cfg := NewLoopConfig(config.NewConfigManager(), nil, prompt, 20)
	assert.Equal(t, 7, cfg.MaxIterations)
	assert.Equal(t, 1_000_000, cfg.InputTokenLimit)
	assert.Equal(t, DefaultMaxBatchTextBytes, cfg.Limits.MaxBatchTextBytes)
}

func TestNeedsExactToolResultAdmission(t *testing.T) {
	textResult := PreparedToolResult{Output: ai.ContentToolOutput(nil, ai.TextContent{Text: "small result"})}
	lowUsage := &ai.TokenCount{InputTokens: 100, OutputTokens: 10, TotalTokens: 110}

	assert.False(t, NeedsExactToolResultAdmission(1_000, lowUsage, []PreparedToolResult{textResult}))
	assert.True(t, NeedsExactToolResultAdmission(1_000, nil, []PreparedToolResult{textResult}))
	assert.True(t, NeedsExactToolResultAdmission(100, &ai.TokenCount{TotalTokens: 74}, []PreparedToolResult{
		{Output: ai.ContentToolOutput(nil, ai.TextContent{Text: "abc"})},
	}))
	assert.True(t, NeedsExactToolResultAdmission(1_000, lowUsage, []PreparedToolResult{
		{Output: ai.ContentToolOutput(nil, ai.BlobContent{MIMEType: "image/png", Data: []byte("pixels")})},
	}))
}
