package genai

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func layoutClient() *Client {
	return &Client{Config: config.NewConfigManager()}
}

func layoutPrompt() ai.Prompt {
	return ai.Prompt{
		Instruction: "be kind",
		Context:     ai.TurnContext{Project: "# AGENTS.md", Files: "File: a.md", Host: "[Memory]"},
		History: []ai.HistoryTurn{
			{User: "q1", Assistant: "a1"},
			{User: "q2", Actions: []ai.HistoryAction{{Tool: "readFile", Args: "a.md"}}, Assistant: "a2"},
		},
		Text:   "User: q3",
		Images: []*ai.Image{{Type: "image/png", Data: []byte{1, 2}}},
	}
}

func texts(c *genai.Content) []string {
	var out []string
	for _, p := range c.Parts {
		if p.Text != "" {
			out = append(out, p.Text)
		}
	}
	return out
}

func TestBuildInitialContents_OneContentPerTurnThenTail(t *testing.T) {
	contents := layoutClient().buildInitialContents(layoutPrompt())

	require.Len(t, contents, 7)
	assert.Equal(t, genai.RoleUser, contents[0].Role)
	assert.Equal(t, []string{"q1"}, texts(contents[0]))
	assert.Equal(t, genai.RoleModel, contents[1].Role)
	assert.Equal(t, []string{"a1"}, texts(contents[1]))
	assert.Equal(t, genai.RoleUser, contents[2].Role)
	assert.Equal(t, []string{"q2"}, texts(contents[2]), "the user's own words only")

	call := contents[3]
	assert.Equal(t, genai.RoleModel, call.Role)
	require.Len(t, call.Parts, 1)
	require.NotNil(t, call.Parts[0].FunctionCall, "the digest replays as a function call")
	assert.Equal(t, "hist_1_0", call.Parts[0].FunctionCall.ID)
	assert.Equal(t, "readFile", call.Parts[0].FunctionCall.Name)
	assert.Equal(t, map[string]any{"args": "a.md"}, call.Parts[0].FunctionCall.Args)

	result := contents[4]
	assert.Equal(t, genai.RoleUser, result.Role)
	require.Len(t, result.Parts, 1)
	require.NotNil(t, result.Parts[0].FunctionResponse)
	assert.Equal(t, "hist_1_0", result.Parts[0].FunctionResponse.ID)
	assert.Equal(t, "readFile", result.Parts[0].FunctionResponse.Name)
	assert.Equal(t, map[string]any{"output": "[recorded] (no summary)", "is_error": false}, result.Parts[0].FunctionResponse.Response)

	assert.Equal(t, genai.RoleModel, contents[5].Role)
	assert.Equal(t, []string{"a2"}, texts(contents[5]), "the reply text only")

	tail := contents[6]
	assert.Equal(t, genai.RoleUser, tail.Role)
	assert.Equal(t, []string{"File: a.md\n\n[Memory]\n\nUser: q3"}, texts(tail))
	require.Len(t, tail.Parts, 2, "image rides in the tail after the text")
	assert.Equal(t, []byte{1, 2}, tail.Parts[1].InlineData.Data)
}

func TestBuildInitialContents_SystemNeverDuplicatedIntoContents(t *testing.T) {
	contents := layoutClient().buildInitialContents(ai.Prompt{Instruction: "be kind", Text: "hi"})

	require.Len(t, contents, 1)
	assert.Equal(t, []string{"hi"}, texts(contents[0]))
}

func TestBuildInitialContents_SkipsEmptySidesOfATurn(t *testing.T) {
	prompt := ai.Prompt{History: []ai.HistoryTurn{{User: "", Assistant: "seeded answer"}, {User: "q", Assistant: ""}}, Text: "hi"}

	contents := layoutClient().buildInitialContents(prompt)

	require.Len(t, contents, 3)
	assert.Equal(t, genai.RoleModel, contents[0].Role)
	assert.Equal(t, genai.RoleUser, contents[1].Role)
	assert.Equal(t, []string{"hi"}, texts(contents[2]))
}

func TestBuildGenerateConfig_SystemInstructionIsStableBlocksOnly(t *testing.T) {
	cfg := layoutClient().buildGenerateConfig(layoutPrompt())

	require.NotNil(t, cfg)
	require.NotNil(t, cfg.SystemInstruction)
	assert.Equal(t, []string{"be kind\n\n# AGENTS.md\n\n" + shared.RecordedNote}, texts(cfg.SystemInstruction))
}

func TestBuildGenerateConfig_NoSystemInstructionWithoutInstruction(t *testing.T) {
	cfg := layoutClient().buildGenerateConfig(ai.Prompt{Text: "hi", Context: ai.TurnContext{Host: "[Memory]"}})

	if cfg != nil {
		assert.Nil(t, cfg.SystemInstruction)
	}
}

func TestCountTokensRequest_VertexCarriesSystemInstructionInConfig(t *testing.T) {
	contents, cfg := countTokensRequest(BackendVertexAI, layoutPrompt())

	require.NotNil(t, cfg)
	assert.Equal(t, []string{"be kind\n\n# AGENTS.md\n\n" + shared.RecordedNote}, texts(cfg.SystemInstruction))
	require.Len(t, contents, 7)
	assert.Equal(t, []string{"q1"}, texts(contents[0]))
}

// The Gemini API backend rejects systemInstruction on CountTokens, so the
// system text is counted as a leading user content instead.
func TestCountTokensRequest_GeminiAPIFoldsSystemIntoContents(t *testing.T) {
	contents, cfg := countTokensRequest(BackendGeminiAPI, layoutPrompt())

	assert.Nil(t, cfg)
	require.Len(t, contents, 8)
	assert.Equal(t, genai.RoleUser, contents[0].Role)
	assert.Equal(t, []string{"be kind\n\n# AGENTS.md\n\n" + shared.RecordedNote}, texts(contents[0]))
	assert.Equal(t, []string{"q1"}, texts(contents[1]))
}

func TestCountTokensRequest_NoSystemNoConfig(t *testing.T) {
	contents, cfg := countTokensRequest(BackendVertexAI, ai.Prompt{Text: "hi"})

	assert.Nil(t, cfg)
	require.Len(t, contents, 1)
}

// An empty message (GetContext counts the prompt with no message) must not
// yield an empty text part, which the API rejects.
func TestBuildInitialContents_EmptyTailTextIsNotSentAsAnEmptyPart(t *testing.T) {
	withImage := ai.Prompt{Images: []*ai.Image{{Type: "image/png", Data: []byte{1}}}}
	contents := layoutClient().buildInitialContents(withImage)
	require.Len(t, contents, 1)
	require.Len(t, contents[0].Parts, 1, "only the image, no empty text part")
	assert.NotNil(t, contents[0].Parts[0].InlineData)

	withHistory := ai.Prompt{History: []ai.HistoryTurn{{User: "q1", Assistant: "a1"}}}
	contents = layoutClient().buildInitialContents(withHistory)
	require.Len(t, contents, 2, "history only; an empty tail adds no content")
}

func TestCountTokensRequest_SystemOnlyPromptCountsAsAUserContent(t *testing.T) {
	prompt := ai.Prompt{Instruction: "be kind"}

	for _, backend := range []Backend{BackendVertexAI, BackendGeminiAPI} {
		contents, cfg := countTokensRequest(backend, prompt)
		assert.Nil(t, cfg, string(backend))
		require.Len(t, contents, 1, string(backend))
		assert.Equal(t, []string{"be kind"}, texts(contents[0]), string(backend))
	}
}

func TestCountTokensRequest_VertexKeepsSystemInConfigWhenContentsExist(t *testing.T) {
	prompt := ai.Prompt{Instruction: "be kind", History: []ai.HistoryTurn{{User: "q1", Assistant: "a1"}}}

	contents, cfg := countTokensRequest(BackendVertexAI, prompt)

	require.NotNil(t, cfg)
	require.Len(t, contents, 2)
	for _, c := range contents {
		for _, p := range c.Parts {
			assert.NotEmpty(t, p.Text)
		}
	}
}

// Parallel calls share one model content and one function-response
// content, which is the pairing Gemini validates.
func TestBuildInitialContents_ParallelCallsSharePairedContents(t *testing.T) {
	contents := layoutClient().buildInitialContents(ai.Prompt{History: []ai.HistoryTurn{{User: "q", Actions: []ai.HistoryAction{
		{Tool: "bash", Args: `command="go test"`, Summary: "Failed: TestX", Failed: true},
		{Tool: "readFile", Args: `path="a.go"`, Summary: "read a.go"},
	}, Assistant: "a"}}, Text: "n"})

	require.Len(t, contents, 5)
	require.Len(t, contents[1].Parts, 2)
	assert.Equal(t, map[string]any{"command": "go test"}, contents[1].Parts[0].FunctionCall.Args)
	require.Len(t, contents[2].Parts, 2)
	assert.Equal(t, map[string]any{"output": "[recorded] Failed: TestX", "is_error": true}, contents[2].Parts[0].FunctionResponse.Response)
	assert.Equal(t, "hist_0_1", contents[2].Parts[1].FunctionResponse.ID)
}

// Responses group per turn only, and every call has exactly one response
// even when omission markers sit between the actions.
func TestBuildInitialContents_CallsAndResponsesPairPerTurn(t *testing.T) {
	contents := layoutClient().buildInitialContents(ai.Prompt{History: []ai.HistoryTurn{
		{User: "q1", Actions: []ai.HistoryAction{{Tool: "bash", Summary: "ok"}, {Tool: ai.OmittedActions, Summary: "3 more actions omitted"}, {Tool: "readFile", Summary: "read"}}},
		{User: "q2", Actions: []ai.HistoryAction{{Tool: "bash", Summary: "ok2"}}, Assistant: "a2"},
	}, Text: "q3"})

	require.Len(t, contents, 8)
	require.Len(t, contents[1].Parts, 2, "two calls in turn 0")
	require.Len(t, contents[2].Parts, 2, "two responses in turn 0")
	assert.Equal(t, "[recorded] [3 more actions omitted before this]\n\nread", contents[2].Parts[1].FunctionResponse.Response["output"])
	assert.Equal(t, genai.RoleUser, contents[3].Role)
	assert.Equal(t, []string{"q2"}, texts(contents[3]))
	require.Len(t, contents[5].Parts, 1, "turn 1's response is its own content")
	assert.Equal(t, "hist_1_0", contents[5].Parts[0].FunctionResponse.ID)
}
