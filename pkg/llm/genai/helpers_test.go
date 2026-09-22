package genai

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
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

	require.Len(t, contents, 5)
	assert.Equal(t, genai.RoleUser, contents[0].Role)
	assert.Equal(t, []string{"q1"}, texts(contents[0]))
	assert.Equal(t, genai.RoleModel, contents[1].Role)
	assert.Equal(t, []string{"a1"}, texts(contents[1]))
	assert.Equal(t, genai.RoleUser, contents[2].Role)
	assert.Equal(t, []string{"q2\n\nAssistant Actions:\n- readFile a.md"}, texts(contents[2]), "the digest follows the user text")
	assert.Equal(t, genai.RoleModel, contents[3].Role)
	assert.Equal(t, []string{"a2"}, texts(contents[3]), "the reply text only")

	tail := contents[4]
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
	assert.Equal(t, []string{"be kind\n\n# AGENTS.md"}, texts(cfg.SystemInstruction))
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
	assert.Equal(t, []string{"be kind\n\n# AGENTS.md"}, texts(cfg.SystemInstruction))
	require.Len(t, contents, 5)
	assert.Equal(t, []string{"q1"}, texts(contents[0]))
}

// The Gemini API backend rejects systemInstruction on CountTokens, so the
// system text is counted as a leading user content instead.
func TestCountTokensRequest_GeminiAPIFoldsSystemIntoContents(t *testing.T) {
	contents, cfg := countTokensRequest(BackendGeminiAPI, layoutPrompt())

	assert.Nil(t, cfg)
	require.Len(t, contents, 6)
	assert.Equal(t, genai.RoleUser, contents[0].Role)
	assert.Equal(t, []string{"be kind\n\n# AGENTS.md"}, texts(contents[0]))
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
