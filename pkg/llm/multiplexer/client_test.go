package multiplexer

import (
	"context"
	"errors"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGen struct {
	name          string
	generateCalls int
}

func (f *fakeGen) GenerateContent(ctx context.Context, p ai.Prompt, debug bool, args ...string) (string, error) {
	f.generateCalls++
	return f.name, nil
}

func (f *fakeGen) GenerateContentAttr(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	f.generateCalls++
	return f.name, nil
}

func (f *fakeGen) CountTokens(ctx context.Context, p ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	return &ai.TokenCount{TotalTokens: 1}, nil
}

func (f *fakeGen) CountTokensAttr(ctx context.Context, p ai.Prompt, debug bool, attrs []ai.Attr) (*ai.TokenCount, error) {
	return &ai.TokenCount{TotalTokens: 1}, nil
}

func (f *fakeGen) GetStatus() *ai.Status {
	return &ai.Status{Backend: f.name, Connected: true}
}

func TestMultiplexer_DefaultProviderUsedWhenPromptOmitted(t *testing.T) {
	genaiStub := &fakeGen{name: "genai"}
	openaiStub := &fakeGen{name: "openai"}

	client, err := NewClient("genai", map[string]Factory{
		"genai":  func() (ai.Gen, error) { return genaiStub, nil },
		"openai": func() (ai.Gen, error) { return openaiStub, nil },
	}, map[string]string{})
	require.NoError(t, err)

	err = client.WarmUp("genai")
	require.NoError(t, err)

	resp, err := client.GenerateContent(context.Background(), ai.Prompt{}, false)
	require.NoError(t, err)
	assert.Equal(t, "genai", resp)
	assert.Equal(t, 1, genaiStub.generateCalls)
	assert.Equal(t, 0, openaiStub.generateCalls)

	status := client.GetStatus()
	require.NotNil(t, status)
	assert.Equal(t, "genai", status.Backend)
}

func TestMultiplexer_RoutesBasedOnPromptProvider(t *testing.T) {
	genaiStub := &fakeGen{name: "genai"}
	openaiStub := &fakeGen{name: "openai"}

	client, err := NewClient("gemini", map[string]Factory{
		"genai":  func() (ai.Gen, error) { return genaiStub, nil },
		"openai": func() (ai.Gen, error) { return openaiStub, nil },
	}, map[string]string{
		"gemini":      "genai",
		"openai-chat": "openai",
	})
	require.NoError(t, err)

	// Warm up default (alias should resolve)
	require.NoError(t, client.WarmUp("gemini"))
	assert.Equal(t, "genai", client.DefaultProvider())

	prompt := ai.Prompt{LLMProvider: "openai-chat"}
	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "openai", resp)
	assert.Equal(t, 0, genaiStub.generateCalls)
	assert.Equal(t, 1, openaiStub.generateCalls)

	status := client.GetStatus()
	require.NotNil(t, status)
	assert.Equal(t, "openai", status.Backend)
}

func TestMultiplexer_StatusReflectsPersonaModel(t *testing.T) {
	openaiStub := &fakeGen{name: "openai"}

	client, err := NewClient("openai", map[string]Factory{
		"openai": func() (ai.Gen, error) { return openaiStub, nil },
	}, map[string]string{})
	require.NoError(t, err)

	prompt := ai.Prompt{LLMProvider: "openai", ModelName: "gpt-4o-mini"}
	_, err = client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)

	status := client.GetStatus()
	require.NotNil(t, status)
	assert.Equal(t, "openai", status.Backend)
	assert.Equal(t, "gpt-4o-mini (persona)", status.Model)
}

func TestMultiplexer_ErrorOnUnknownProvider(t *testing.T) {
	client, err := NewClient("genai", map[string]Factory{
		"genai": func() (ai.Gen, error) { return &fakeGen{name: "genai"}, nil },
	}, map[string]string{})
	require.NoError(t, err)

	_, err = client.GenerateContent(context.Background(), ai.Prompt{LLMProvider: "anthropic"}, false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported LLM provider")
}

func TestMultiplexer_PropagatesFactoryErrors(t *testing.T) {
	client, err := NewClient("genai", map[string]Factory{
		"genai": func() (ai.Gen, error) { return nil, errors.New("boom") },
	}, map[string]string{})
	require.NoError(t, err)

	_, err = client.GenerateContent(context.Background(), ai.Prompt{}, false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}
