package shared

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
)

func TestModelInputAdmissionLimitReservesRequestedOutput(t *testing.T) {
	prompt := ai.Prompt{
		MaxTokens: 20_000,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit: 100_000, SharedContextWindow: true,
		},
	}
	assert.Equal(t, 80_000, ModelInputAdmissionLimit(prompt))
}

func TestModelInputAdmissionLimitClampsReserveToModelMaximum(t *testing.T) {
	prompt := ai.Prompt{
		MaxTokens: 20_000,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit: 100_000, OutputTokenLimit: 8_000, SharedContextWindow: true,
		},
	}
	assert.Equal(t, 92_000, ModelInputAdmissionLimit(prompt))
}

func TestModelInputAdmissionLimitUnknownModelIsDisabled(t *testing.T) {
	assert.Zero(t, ModelInputAdmissionLimit(ai.Prompt{}))
}

func TestModelInputAdmissionLimitCapsOversizedReserveAtHalfWindow(t *testing.T) {
	prompt := ai.Prompt{
		MaxTokens: 200_000,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit: 100_000, SharedContextWindow: true,
		},
	}
	assert.Equal(t, 50_000, ModelInputAdmissionLimit(prompt))
}
