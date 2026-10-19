package genie

import (
	"context"
	"errors"
	"strings"
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
func (c catalogRunner) Canonical(provider string) (string, error) {
	provider = strings.ToLower(provider)
	if provider == "gemini" {
		provider = "genai"
	}
	for _, p := range c.providers {
		if p == provider {
			return p, nil
		}
	}
	return "", errors.New("unsupported")
}
func (c catalogRunner) ListModels(_ context.Context, provider string) ([]string, error) {
	models, ok := c.models[provider]
	if !ok {
		return nil, ai.ErrNoModelList
	}
	return models, nil
}

func TestSetModelValidatesTheProviderAgainstTheCatalog(t *testing.T) {
	// started is false, so SetModel never reaches initContextBudget and the
	// nil config/context managers are safe; the fixture test covers started.
	g := &core{promptRunner: catalogRunner{providers: []string{"genai", "openai"}, models: map[string][]string{"openai": {"gpt-5.5"}}}}

	require.NoError(t, g.SetModel(ModelSelection{Provider: "OpenAI", Model: "gpt-5.5"}))
	assert.Equal(t, ModelSelection{Provider: "openai", Model: "gpt-5.5"}, g.override(), "provider is canonicalised")

	err := g.SetModel(ModelSelection{Provider: "nope", Model: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "genai, openai")
	assert.Equal(t, ModelSelection{Provider: "openai", Model: "gpt-5.5"}, g.override(), "a rejected selection changes nothing")

	require.NoError(t, g.SetModel(ModelSelection{Provider: "Gemini", Model: "gemini-3.7-pro"}))
	assert.Equal(t, ModelSelection{Provider: "genai", Model: "gemini-3.7-pro"}, g.override(), "aliases resolve to the canonical provider")

	err = g.SetModel(ModelSelection{Provider: "openai"})
	require.Error(t, err, "a provider without a model would send the persona's model elsewhere")
	assert.Contains(t, err.Error(), "model is required")

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
