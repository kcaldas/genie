package openai

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// Reasoning is on by default from gpt-5 onward, and a tool-carrying
// /v1/chat/completions request is refused while it is. The rule tests the
// generation rather than the model name, so a family released tomorrow is
// covered and an older one is left alone.
func TestGptGeneration(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  float64
		isGPT bool
	}{
		{"gpt-5.6-luna", 5.6, true},
		{"gpt-5", 5, true},
		{"gpt-6-something", 6, true},
		{"gpt-4o", 4, true},
		{"gpt-4.1-mini", 4.1, true},
		{"o4-mini", 0, false},
		{"claude-sonnet-5", 0, false},
	} {
		got, ok := gptGeneration(tc.model)
		if ok != tc.isGPT || got != tc.want {
			t.Errorf("gptGeneration(%q) = %v,%v; want %v,%v", tc.model, got, ok, tc.want, tc.isGPT)
		}
	}
}

// A turn that carries tools must ask for no reasoning on those models, or
// it cannot run at all.
func TestReasoningDisabledForToolsOnModelsThatRefuseBoth(t *testing.T) {
	c := &Client{logger: logging.NewAPILogger("openai-test"), config: config.NewConfigManager()}

	withTools := ai.Prompt{Functions: []*ai.FunctionDeclaration{{Name: "glt_get_rates"}}}
	params := openai.ChatCompletionNewParams{Model: shared.ChatModel("gpt-5.6-luna")}
	c.applyGenerationConfig(&params, withTools)
	if params.ReasoningEffort != shared.ReasoningEffort("none") {
		t.Errorf("reasoning_effort = %q, want none when tools ride on gpt-5.6", params.ReasoningEffort)
	}

	// No tools: nothing to conflict with, so reasoning stays as the model
	// would default it.
	params = openai.ChatCompletionNewParams{Model: shared.ChatModel("gpt-5.6-luna")}
	c.applyGenerationConfig(&params, ai.Prompt{})
	if params.ReasoningEffort != "" {
		t.Errorf("reasoning_effort = %q, want unset without tools", params.ReasoningEffort)
	}

	// Older families and the o-series keep whatever they do by default.
	for _, model := range []string{"o4-mini", "gpt-4o", "gpt-4.1-mini"} {
		params = openai.ChatCompletionNewParams{Model: shared.ChatModel(model)}
		c.applyGenerationConfig(&params, withTools)
		if params.ReasoningEffort != "" {
			t.Errorf("reasoning_effort = %q, want untouched for %s", params.ReasoningEffort, model)
		}
	}

	// A newer family inherits the refusal without anyone editing a list.
	params = openai.ChatCompletionNewParams{Model: shared.ChatModel("gpt-7-mini")}
	c.applyGenerationConfig(&params, withTools)
	if params.ReasoningEffort != shared.ReasoningEffort("none") {
		t.Errorf("reasoning_effort = %q, want none for a future family", params.ReasoningEffort)
	}
}
