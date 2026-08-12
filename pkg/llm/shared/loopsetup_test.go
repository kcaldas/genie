package shared

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
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
