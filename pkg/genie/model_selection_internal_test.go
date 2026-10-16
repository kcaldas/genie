package genie

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type catalogRunner struct {
	providers []string
	models    map[string][]string
}

func (catalogRunner) RunPrompt(context.Context, *ai.Prompt, map[string]string, events.EventBus) (string, error) {
	return "", nil
}
func (catalogRunner) RunPromptStream(context.Context, *ai.Prompt, map[string]string, events.EventBus) (string, error) {
	return "", nil
}
func (catalogRunner) CountTokens(context.Context, *ai.Prompt, map[string]string, events.EventBus) (*ai.TokenCount, error) {
	return nil, nil
}
func (catalogRunner) GetStatus() *ai.Status { return &ai.Status{} }
func (c catalogRunner) Providers() []string { return c.providers }
func (c catalogRunner) ListModels(_ context.Context, provider string) ([]string, error) {
	models, ok := c.models[provider]
	if !ok {
		return nil, ai.ErrNoModelList
	}
	return models, nil
}

func TestSetModelValidatesTheProviderAgainstTheCatalog(t *testing.T) {
	g := &core{promptRunner: catalogRunner{providers: []string{"genai", "openai"}, models: map[string][]string{"openai": {"gpt-5.5"}}}}

	require.NoError(t, g.SetModel(ModelSelection{Provider: "OpenAI", Model: "gpt-5.5"}))
	assert.Equal(t, ModelSelection{Provider: "openai", Model: "gpt-5.5"}, g.override(), "provider is canonicalised")

	err := g.SetModel(ModelSelection{Provider: "nope", Model: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "genai, openai")
	assert.Equal(t, ModelSelection{Provider: "openai", Model: "gpt-5.5"}, g.override(), "a rejected selection changes nothing")

	assert.Equal(t, []string{"genai", "openai"}, g.Providers())
	models, err := g.ListModels(context.Background(), "openai")
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-5.5"}, models)
	_, err = g.ListModels(context.Background(), "genai")
	assert.ErrorIs(t, err, ai.ErrNoModelList)
}

func TestSetModelWithoutCatalogAcceptsAnyProvider(t *testing.T) {
	g := &core{promptRunner: plainRunner{}}
	require.NoError(t, g.SetModel(ModelSelection{Provider: "anything", Model: "m"}))
	assert.Nil(t, g.Providers())
	_, err := g.ListModels(context.Background(), "")
	assert.ErrorIs(t, err, ai.ErrNoModelList)
}

type plainRunner struct{}

func (plainRunner) RunPrompt(context.Context, *ai.Prompt, map[string]string, events.EventBus) (string, error) {
	return "", nil
}
func (plainRunner) RunPromptStream(context.Context, *ai.Prompt, map[string]string, events.EventBus) (string, error) {
	return "", nil
}
func (plainRunner) CountTokens(context.Context, *ai.Prompt, map[string]string, events.EventBus) (*ai.TokenCount, error) {
	return nil, nil
}
func (plainRunner) GetStatus() *ai.Status { return &ai.Status{} }
