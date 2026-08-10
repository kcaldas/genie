package openai

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// gpt-5.6 refuses function tools while reasoning is on in
// /v1/chat/completions, and says so: reasoning_effort "none", or the
// Responses API. Until the Responses API is wired, a turn that carries
// tools must ask for no reasoning or it cannot run at all.
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

	// Other models keep whatever reasoning they do by default, tools or not.
	params = openai.ChatCompletionNewParams{Model: shared.ChatModel("o4-mini")}
	c.applyGenerationConfig(&params, withTools)
	if params.ReasoningEffort != "" {
		t.Errorf("reasoning_effort = %q, want untouched for o4-mini", params.ReasoningEffort)
	}
}
