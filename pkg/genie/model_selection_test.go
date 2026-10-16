package genie_test

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetModelOverridesThePersonaPin(t *testing.T) {
	f := genietest.NewTestFixture(t)
	defer f.Cleanup()
	f.UsePrompt(&ai.Prompt{Name: "pinned", Instruction: "x", Text: "{{.message}}", LLMProvider: "genai", ModelName: "gemini-3.7-flash"})
	f.StartAndGetSession()
	ctx := context.Background()

	assert.Equal(t, genie.ModelSelection{Provider: "genai", Model: "gemini-3.7-flash"}, f.Genie.Model(ctx), "before override the persona's pin is reported")

	require.NoError(t, f.Genie.SetModel(genie.ModelSelection{Provider: "openai", Model: "gpt-5.5"}))
	assert.Equal(t, genie.ModelSelection{Provider: "openai", Model: "gpt-5.5"}, f.Genie.Model(ctx))
	assert.Equal(t, "openai/gpt-5.5", f.Genie.GetStatus().Model)

	f.ExpectSimpleMessage("hi", "hello")
	require.NoError(t, f.StartChat("hi"))
	f.WaitForResponseOrFail(2 * time.Second)
	prompts := f.MockPromptRunner.CapturedPrompts()
	require.Len(t, prompts, 1)
	assert.Equal(t, "openai", prompts[0].LLMProvider)
	assert.Equal(t, "gpt-5.5", prompts[0].ModelName)

	// Model only: the persona's provider stays.
	require.NoError(t, f.Genie.SetModel(genie.ModelSelection{Model: "gemini-3.7-pro"}))
	assert.Equal(t, genie.ModelSelection{Provider: "genai", Model: "gemini-3.7-pro"}, f.Genie.Model(ctx))

	// Clearing goes back to the persona.
	require.NoError(t, f.Genie.SetModel(genie.ModelSelection{}))
	assert.Equal(t, genie.ModelSelection{Provider: "genai", Model: "gemini-3.7-flash"}, f.Genie.Model(ctx))
}

func TestParseModelSpec(t *testing.T) {
	cases := map[string]genie.ModelSelection{
		"gpt-5.5":              {Model: "gpt-5.5"},
		"openai/gpt-5.5":       {Provider: "openai", Model: "gpt-5.5"},
		" Anthropic/claude-x ": {Provider: "anthropic", Model: "claude-x"},
		"openai/":              {Provider: "openai"},
		"openai/org/gpt-5.5":   {Provider: "openai", Model: "org/gpt-5.5"},
		"":                     {},
	}
	for in, want := range cases {
		assert.Equal(t, want, genie.ParseModelSpec(in), in)
	}
	assert.Equal(t, "openai/gpt-5.5", genie.ModelSelection{Provider: "openai", Model: "gpt-5.5"}.String())
	assert.Equal(t, "gpt-5.5", genie.ModelSelection{Model: "gpt-5.5"}.String())
	assert.True(t, genie.ModelSelection{}.IsZero())
}
