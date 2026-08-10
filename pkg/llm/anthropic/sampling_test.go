package anthropic

import (
	"testing"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/logging"
)

func newTestParams(model string) anthropic_sdk.MessageNewParams {
	return anthropic_sdk.MessageNewParams{Model: anthropic_sdk.Model(model)}
}

func testPrompt(temp, topP float32) ai.Prompt {
	return ai.Prompt{Temperature: temp, TopP: topP}
}

// Anthropic retired temperature and top_p on newer generations: sending
// either is a 400, not a warning. The gate lists the generations known to
// accept them, so a model nobody has taught this code about omits the
// fields and still answers.
func TestAllowsSamplingParams(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  bool
	}{
		{"claude-3-5-sonnet-20241022", true},
		{"claude-3-haiku-20240307", true},
		{"claude-haiku-4-5-20251001", true},
		{"claude-sonnet-4-20250514", true},
		{"claude-opus-4-1", true},
		{"claude-sonnet-5", false},
		{"claude-opus-5", false},
		{"claude-fable-5", false},
		{"  CLAUDE-SONNET-5  ", false},
		{"something-unreleased", false},
		{"", false},
	} {
		if got := allowsSamplingParams(tc.model); got != tc.want {
			t.Errorf("allowsSamplingParams(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

// A model that refuses the fields must have neither set, whatever the
// prompt asks for.
func TestApplyGenerationConfigOmitsSamplingForNewModels(t *testing.T) {
	c := &Client{logger: logging.NewAPILogger("anthropic-test")}

	params := newTestParams("claude-sonnet-5")
	c.applyGenerationConfig(&params, testPrompt(0.7, 0.9))
	if params.Temperature.Valid() {
		t.Error("temperature was sent to a model that rejects it")
	}
	if params.TopP.Valid() {
		t.Error("top_p was sent to a model that rejects it")
	}

	params = newTestParams("claude-haiku-4-5-20251001")
	c.applyGenerationConfig(&params, testPrompt(0.7, 0))
	if !params.Temperature.Valid() {
		t.Error("temperature was dropped for a model that accepts it")
	}
}
