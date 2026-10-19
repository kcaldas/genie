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
	assert.Equal(t, "openai", genie.ModelSelection{Provider: "openai"}.String())
	assert.True(t, genie.ModelSelection{}.IsZero())
}

// swappablePromptFactory serves a different pinned model per persona.
type swappablePromptFactory struct{ byPersona map[string]*ai.Prompt }

func (f swappablePromptFactory) GetPrompt(_ context.Context, name string) (*ai.Prompt, error) {
	if p, ok := f.byPersona[name]; ok {
		return p, nil
	}
	return f.byPersona["genie"], nil
}

func (f swappablePromptFactory) GetPromptFromBytes(context.Context, []byte) (*ai.Prompt, error) {
	return f.byPersona["genie"], nil
}

type namedPersona string

func (p namedPersona) GetID() string     { return string(p) }
func (p namedPersona) GetName() string   { return string(p) }
func (p namedPersona) GetSource() string { return "test" }

func TestModelFollowsTheSessionPersona(t *testing.T) {
	f := genietest.NewTestFixture(t)
	defer f.Cleanup()
	factory := swappablePromptFactory{byPersona: map[string]*ai.Prompt{
		"genie": {Name: "genie", Instruction: "x", Text: "{{.message}}", LLMProvider: "genai", ModelName: "gemini-3.7-flash"},
		"coder": {Name: "coder", Instruction: "x", Text: "{{.message}}", LLMProvider: "anthropic", ModelName: "claude-sonnet-5-5"},
	}}
	f.UsePromptFactory(factory)
	sess := f.StartAndGetSession()
	ctx := context.Background()

	assert.Equal(t, "genai/gemini-3.7-flash", f.Genie.Model(ctx).String())
	sess.SetPersona(namedPersona("coder"))
	assert.Equal(t, "anthropic/claude-sonnet-5-5", f.Genie.Model(ctx).String(), "the active session persona wins over the default")

	require.NoError(t, f.Genie.SetModel(genie.ModelSelection{Model: "claude-opus-5-5"}))
	assert.Equal(t, "anthropic/claude-opus-5-5", f.Genie.Model(ctx).String(), "a model-only override keeps the active persona's provider")
	sess.SetPersona(namedPersona("genie"))
	assert.Equal(t, "genai/claude-opus-5-5", f.Genie.Model(ctx).String(), "the override survives a persona swap")
}
