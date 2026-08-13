package genai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestDiscoverModelCapabilitiesUsesModelsAPI(t *testing.T) {
	client := &Client{
		Config:      config.NewConfigManager(),
		EventBus:    &events.NoOpEventBus{},
		initialized: true,
		Backend:     BackendGeminiAPI,
		getModelFn: func(context.Context, string) (*genai.Model, error) {
			return &genai.Model{
				Name:             "models/gemini-test",
				InputTokenLimit:  1_048_576,
				OutputTokenLimit: 65_536,
				Thinking:         true,
			}, nil
		},
	}

	caps, err := client.DiscoverModelCapabilities(context.Background(), "gemini-test")
	require.NoError(t, err)
	assert.Equal(t, "gemini-test", caps.Model)
	assert.Equal(t, 1_048_576, caps.InputTokenLimit)
	assert.Equal(t, 65_536, caps.OutputTokenLimit)
	assert.True(t, caps.SupportsReasoning)
	assert.Equal(t, ai.CapabilitySourceProvider, caps.Source)
}

func TestGeminiAdmissionReservesFallbackButNotInputOnlyLimit(t *testing.T) {
	client := &Client{Config: config.NewConfigManager(), EventBus: &events.NoOpEventBus{}}
	prompt := ai.Prompt{
		MaxTokens: 200,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit: 1_000,
			Source:          ai.CapabilitySourceFallback,
		},
	}

	assert.Equal(t, 800, client.loopConfig(prompt).InputTokenLimit)
	prompt.ModelCapabilities.Source = ai.CapabilitySourceProvider
	assert.Equal(t, 1_000, client.loopConfig(prompt).InputTokenLimit)
}

func TestGeminiBlobMIMEAllowlistRejectsArbitraryBinary(t *testing.T) {
	assert.True(t, supportsGeminiBlob(ai.BlobContent{MIMEType: "image/png"}))
	assert.True(t, supportsGeminiBlob(ai.BlobContent{MIMEType: "application/pdf"}))
	assert.True(t, supportsGeminiBlob(ai.BlobContent{MIMEType: "audio/wav"}))
	assert.False(t, supportsGeminiBlob(ai.BlobContent{MIMEType: "image/bmp"}))
	assert.False(t, supportsGeminiBlob(ai.BlobContent{MIMEType: "application/zip"}))
	assert.False(t, supportsGeminiBlob(ai.BlobContent{MIMEType: "application/octet-stream"}))
}

func TestGeminiToolResultAdmissionCountFailureFailsOpen(t *testing.T) {
	client := &Client{Config: config.NewConfigManager(), EventBus: &events.NoOpEventBus{}}
	client.countTokensFn = func(context.Context, string, []*genai.Content, *genai.CountTokensConfig) (*genai.CountTokensResponse, error) {
		return nil, errors.New("count unavailable")
	}
	turn := client.newTurn(ai.Prompt{
		Text:              "search",
		ModelCapabilities: &ai.ModelCapabilities{InputTokenLimit: 1_000},
	})
	result := llmshared.PreparedToolResult{
		Call:   llmshared.ToolCall{ID: "call-1", Name: "search"},
		Output: ai.ContentToolOutput(nil, ai.TextContent{Text: "bounded result"}),
	}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, nil))
	require.Len(t, turn.contents, 2)
}

func TestGeminiToolResultAdmissionSkipsRoutineTextCount(t *testing.T) {
	countCalls := 0
	client := &Client{Config: config.NewConfigManager(), EventBus: &events.NoOpEventBus{}}
	client.countTokensFn = func(context.Context, string, []*genai.Content, *genai.CountTokensConfig) (*genai.CountTokensResponse, error) {
		countCalls++
		return &genai.CountTokensResponse{TotalTokens: 10}, nil
	}
	turn := client.newTurn(ai.Prompt{
		Text:              "search",
		ModelCapabilities: &ai.ModelCapabilities{InputTokenLimit: 1_000},
	})
	result := llmshared.PreparedToolResult{
		Call:   llmshared.ToolCall{ID: "call-1", Name: "search"},
		Output: ai.ContentToolOutput(nil, ai.TextContent{Text: "small result"}),
	}
	usage := &ai.TokenCount{InputTokens: 100, OutputTokens: 10, TotalTokens: 110}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, usage))
	assert.Zero(t, countCalls)
}

func TestGeminiToolResultAdmissionDropsMediaBeforeSending(t *testing.T) {
	client := &Client{Config: config.NewConfigManager(), EventBus: &events.NoOpEventBus{}}
	client.countTokensFn = func(_ context.Context, _ string, contents []*genai.Content, _ *genai.CountTokensConfig) (*genai.CountTokensResponse, error) {
		for _, content := range contents {
			for _, part := range content.Parts {
				if part.InlineData != nil {
					return &genai.CountTokensResponse{TotalTokens: 110}, nil
				}
			}
		}
		return &genai.CountTokensResponse{TotalTokens: 20}, nil
	}
	prompt := ai.Prompt{
		Text:      "inspect",
		ModelName: "gemini-test",
		MaxTokens: 20,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit:  100,
			OutputTokenLimit: 20,
		},
	}
	turn := client.newTurn(prompt)
	result := llmshared.PreparedToolResult{
		Call: llmshared.ToolCall{ID: "call-1", Name: "screenshot"},
		Output: ai.ContentToolOutput(nil,
			ai.TextContent{Text: "screenshot metadata"},
			ai.BlobContent{MIMEType: "image/png", Data: []byte("pixels"), Name: "screen.png"},
		),
	}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, nil))
	for _, content := range turn.contents {
		for _, part := range content.Parts {
			assert.Nil(t, part.InlineData)
			if part.FunctionResponse != nil {
				assert.Contains(t, part.FunctionResponse.Response["output"], "omitted: model input budget exhausted")
			}
		}
	}
}

func TestGeminiToolResultAdmissionUsesMinimalCorrelatedResult(t *testing.T) {
	client := &Client{Config: config.NewConfigManager(), EventBus: &events.NoOpEventBus{}}
	client.countTokensFn = func(_ context.Context, _ string, contents []*genai.Content, _ *genai.CountTokensConfig) (*genai.CountTokensResponse, error) {
		for _, content := range contents {
			for _, part := range content.Parts {
				if part.FunctionResponse != nil {
					output, _ := part.FunctionResponse.Response["output"].(string)
					if strings.Contains(output, "narrow the tool call") {
						return &genai.CountTokensResponse{TotalTokens: 20}, nil
					}
				}
			}
		}
		return &genai.CountTokensResponse{TotalTokens: 110}, nil
	}
	turn := client.newTurn(ai.Prompt{
		Text:      "search",
		ModelName: "gemini-test",
		MaxTokens: 20,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit:  100,
			OutputTokenLimit: 20,
		},
	})
	result := llmshared.PreparedToolResult{
		Call:   llmshared.ToolCall{ID: "call-7", Name: "search"},
		Output: ai.ContentToolOutput(nil, ai.TextContent{Text: strings.Repeat("result", 100)}),
	}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, nil))
	last := turn.contents[len(turn.contents)-1].Parts[0].FunctionResponse
	require.NotNil(t, last)
	assert.Equal(t, "call-7", last.ID)
	assert.Contains(t, last.Response["output"], "tool output omitted")
}
