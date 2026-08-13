package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeModelClient struct {
	metadata *anthropic_sdk.ModelInfo
}

func (f fakeModelClient) Get(context.Context, string, anthropic_sdk.ModelGetParams, ...option.RequestOption) (*anthropic_sdk.ModelInfo, error) {
	return f.metadata, nil
}

func TestDiscoverModelCapabilitiesUsesModelsAPI(t *testing.T) {
	var metadata anthropic_sdk.ModelInfo
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":"claude-test","type":"model","display_name":"Claude Test",
		"created_at":"2026-08-01T00:00:00Z","max_input_tokens":1000000,
		"max_tokens":128000,"capabilities":{"image_input":{"supported":true},"pdf_input":{"supported":true},"thinking":{"supported":true}}
	}`), &metadata))
	client := &Client{
		messages:    &mockMessageClient{t: t},
		models:      fakeModelClient{metadata: &metadata},
		initialized: true,
	}

	caps, err := client.DiscoverModelCapabilities(context.Background(), "claude-test")
	require.NoError(t, err)
	assert.Equal(t, "claude-test", caps.Model)
	assert.Equal(t, 1_000_000, caps.InputTokenLimit)
	assert.Equal(t, 128_000, caps.OutputTokenLimit)
	assert.True(t, caps.SupportsInput(ai.ModalityImage))
	assert.True(t, caps.SupportsInput(ai.ModalityDocument))
	assert.True(t, caps.SupportsReasoning)
}

func TestDiscoverModelCapabilitiesLeavesPartialModalitiesUnknown(t *testing.T) {
	var metadata anthropic_sdk.ModelInfo
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":"claude-test","type":"model","display_name":"Claude Test",
		"created_at":"2026-08-01T00:00:00Z","max_input_tokens":200000,
		"max_tokens":64000,"capabilities":{"image_input":{"supported":true}}
	}`), &metadata))
	client := &Client{
		messages:    &mockMessageClient{t: t},
		models:      fakeModelClient{metadata: &metadata},
		initialized: true,
	}

	caps, err := client.DiscoverModelCapabilities(context.Background(), "claude-test")
	require.NoError(t, err)
	assert.Nil(t, caps.InputModalities)
	assert.False(t, caps.SupportsReasoning)
}

func TestAnthropicAdmissionReservesRequestedOutput(t *testing.T) {
	client := &Client{messages: &mockMessageClient{t: t}, config: config.NewConfigManager(), eventBus: &events.NoOpEventBus{}}
	prompt := ai.Prompt{
		Text:      "inspect",
		ModelName: "claude-test",
		MaxTokens: 20,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit:  100,
			OutputTokenLimit: 50,
		},
	}

	turn, err := client.newTurn(prompt)
	require.NoError(t, err)
	assert.Equal(t, 80, turn.inputLimit)
	assert.Equal(t, 80, client.loopConfig(prompt).InputTokenLimit)
	prompt.MaxTokens = 200
	assert.Equal(t, 50, anthropicInputAdmissionLimit(prompt, int64(prompt.MaxTokens)),
		"the model output ceiling should cap an oversized reservation")
}

func TestAnthropicToolResultAdmissionCountFailureFailsOpen(t *testing.T) {
	mockAPI := &mockMessageClient{t: t, countErr: errors.New("count unavailable")}
	client := &Client{messages: mockAPI, config: config.NewConfigManager(), eventBus: &events.NoOpEventBus{}}
	turn, err := client.newTurn(ai.Prompt{
		Text:              "search",
		ModelCapabilities: &ai.ModelCapabilities{InputTokenLimit: 1_000, OutputTokenLimit: 100},
	})
	require.NoError(t, err)
	result := llmshared.PreparedToolResult{
		Call:   llmshared.ToolCall{ID: "call-1", Name: "search"},
		Output: ai.ContentToolOutput(nil, ai.TextContent{Text: "bounded result"}),
	}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, nil))
	require.Len(t, turn.messages, 2)
	require.Len(t, mockAPI.countRequests, 1)
}

func TestAnthropicToolResultAdmissionSkipsRoutineTextCount(t *testing.T) {
	mockAPI := &mockMessageClient{t: t}
	client := &Client{messages: mockAPI, config: config.NewConfigManager(), eventBus: &events.NoOpEventBus{}}
	turn, err := client.newTurn(ai.Prompt{
		Text:              "search",
		ModelCapabilities: &ai.ModelCapabilities{InputTokenLimit: 1_000, OutputTokenLimit: 100},
	})
	require.NoError(t, err)
	result := llmshared.PreparedToolResult{
		Call:   llmshared.ToolCall{ID: "call-1", Name: "search"},
		Output: ai.ContentToolOutput(nil, ai.TextContent{Text: "small result"}),
	}
	usage := &ai.TokenCount{InputTokens: 100, OutputTokens: 10, TotalTokens: 110}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, usage))
	require.Len(t, turn.messages, 2)
	assert.Empty(t, mockAPI.countRequests)
}

func TestAnthropicToolResultAdmissionFailsOpenAfterMediaDegradation(t *testing.T) {
	mockAPI := &mockMessageClient{t: t, countErrAt: 2}
	mockAPI.countFn = func(params anthropic_sdk.MessageCountTokensParams) *anthropic_sdk.MessageTokensCount {
		wire, _ := json.Marshal(params.Messages)
		if strings.Contains(string(wire), `"type":"image"`) {
			return &anthropic_sdk.MessageTokensCount{InputTokens: 110}
		}
		return &anthropic_sdk.MessageTokensCount{InputTokens: 20}
	}
	client := &Client{messages: mockAPI, config: config.NewConfigManager(), eventBus: &events.NoOpEventBus{}}
	turn, err := client.newTurn(ai.Prompt{
		Text:              "inspect",
		ModelCapabilities: &ai.ModelCapabilities{InputTokenLimit: 100, OutputTokenLimit: 10},
	})
	require.NoError(t, err)
	result := llmshared.PreparedToolResult{
		Call: llmshared.ToolCall{ID: "call-1", Name: "screenshot"},
		Output: ai.ContentToolOutput(nil,
			ai.TextContent{Text: "metadata"},
			ai.BlobContent{MIMEType: "image/png", Data: []byte("pixels")},
		),
	}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, nil))
	require.Len(t, mockAPI.countRequests, 2)
	require.Len(t, turn.messages, 2)
	wire, err := json.Marshal(turn.messages[1])
	require.NoError(t, err)
	assert.NotContains(t, string(wire), `"type":"image"`)
	assert.Contains(t, string(wire), "omitted: model input budget exhausted")
}

func TestAnthropicToolResultAdmissionDropsMediaBeforeSending(t *testing.T) {
	mockAPI := &mockMessageClient{t: t}
	mockAPI.countFn = func(params anthropic_sdk.MessageCountTokensParams) *anthropic_sdk.MessageTokensCount {
		wire, _ := json.Marshal(params.Messages)
		if strings.Contains(string(wire), `"type":"image"`) {
			return &anthropic_sdk.MessageTokensCount{InputTokens: 110}
		}
		return &anthropic_sdk.MessageTokensCount{InputTokens: 20}
	}
	client := &Client{messages: mockAPI, config: config.NewConfigManager(), eventBus: &events.NoOpEventBus{}}
	prompt := ai.Prompt{
		Text:      "inspect",
		ModelName: "claude-test",
		MaxTokens: 20,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit:  100,
			OutputTokenLimit: 20,
		},
	}
	turn, err := client.newTurn(prompt)
	require.NoError(t, err)
	result := llmshared.PreparedToolResult{
		Call: llmshared.ToolCall{ID: "call-1", Name: "screenshot"},
		Output: ai.ContentToolOutput(nil,
			ai.TextContent{Text: "metadata"},
			ai.BlobContent{MIMEType: "image/png", Data: []byte("pixels"), Name: "screen.png"},
		),
	}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, nil))
	require.Len(t, turn.messages, 2)
	wire, err := json.Marshal(turn.messages[1])
	require.NoError(t, err)
	assert.NotContains(t, string(wire), `"type":"image"`)
	assert.Contains(t, string(wire), "omitted: model input budget exhausted")
}

func TestAnthropicToolResultAdmissionUsesMinimalCorrelatedResult(t *testing.T) {
	mockAPI := &mockMessageClient{t: t}
	mockAPI.countFn = func(params anthropic_sdk.MessageCountTokensParams) *anthropic_sdk.MessageTokensCount {
		wire, _ := json.Marshal(params.Messages)
		if strings.Contains(string(wire), "narrow the tool call") {
			return &anthropic_sdk.MessageTokensCount{InputTokens: 20}
		}
		return &anthropic_sdk.MessageTokensCount{InputTokens: 110}
	}
	client := &Client{messages: mockAPI, config: config.NewConfigManager(), eventBus: &events.NoOpEventBus{}}
	turn, err := client.newTurn(ai.Prompt{
		Text:      "search",
		ModelName: "claude-test",
		MaxTokens: 20,
		ModelCapabilities: &ai.ModelCapabilities{
			InputTokenLimit:  100,
			OutputTokenLimit: 20,
		},
	})
	require.NoError(t, err)
	result := llmshared.PreparedToolResult{
		Call:   llmshared.ToolCall{ID: "call-7", Name: "search"},
		Output: ai.ContentToolOutput(nil, ai.TextContent{Text: strings.Repeat("result", 100)}),
	}

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result}, nil))
	last := turn.messages[len(turn.messages)-1]
	require.Len(t, last.Content, 1)
	require.NotNil(t, last.Content[0].OfToolResult)
	assert.Equal(t, "call-7", last.Content[0].OfToolResult.ToolUseID)
	wire, err := json.Marshal(last)
	require.NoError(t, err)
	assert.Contains(t, string(wire), "tool output omitted")
}
