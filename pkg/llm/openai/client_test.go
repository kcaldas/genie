package openai

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"testing"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

type mockChatCompletions struct {
	t         *testing.T
	mu        sync.Mutex
	requests  []openai.ChatCompletionNewParams
	responses []*openai.ChatCompletion
	err       error
}

func (m *mockChatCompletions) New(ctx context.Context, params openai.ChatCompletionNewParams, _ ...option.RequestOption) (*openai.ChatCompletion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = append(m.requests, params)

	if m.err != nil {
		return nil, m.err
	}

	if len(m.responses) == 0 {
		require.FailNow(m.t, "mock chat completions received more calls than configured responses")
	}

	resp := m.responses[0]
	m.responses = m.responses[1:]
	return resp, nil
}

func newChatCompletionMessage(content string, toolCalls []openai.ChatCompletionMessageToolCall) openai.ChatCompletionChoice {
	return openai.ChatCompletionChoice{
		Index:        0,
		FinishReason: finishReason(toolCalls),
		Logprobs: openai.ChatCompletionChoiceLogprobs{
			Content: []openai.ChatCompletionTokenLogprob{},
			Refusal: []openai.ChatCompletionTokenLogprob{},
		},
		Message: openai.ChatCompletionMessage{
			Role:      constant.Assistant(""),
			Content:   content,
			Refusal:   "",
			ToolCalls: toolCalls,
		},
	}
}

func finishReason(calls []openai.ChatCompletionMessageToolCall) string {
	if len(calls) > 0 {
		return "tool_calls"
	}
	return "stop"
}

func newChatCompletion(id string, model shared.ChatModel, choice openai.ChatCompletionChoice, usage openai.CompletionUsage) *openai.ChatCompletion {
	return &openai.ChatCompletion{
		ID:      id,
		Object:  constant.ChatCompletion(""),
		Created: 0,
		Model:   string(model),
		Choices: []openai.ChatCompletionChoice{choice},
		Usage:   usage,
	}
}

func TestClient_GenerateContent_SimpleResponse(t *testing.T) {
	mockAPI := &mockChatCompletions{
		t: t,
		responses: []*openai.ChatCompletion{
			newChatCompletion(
				"test",
				shared.ChatModelGPT4oMini,
				newChatCompletionMessage("Hello there!", nil),
				openai.CompletionUsage{},
			),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithChatClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:        "greeting",
		Instruction: "You are a helpful assistant.",
		Text:        "Say hello.",
		ModelName:   string(shared.ChatModelGPT4oMini),
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", resp)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.requests, 1)
	request := mockAPI.requests[0]
	assert.Equal(t, shared.ChatModelGPT4oMini, request.Model)
	require.Len(t, request.Messages, 2)
	require.NotNil(t, request.Messages[0].OfSystem)
	require.True(t, request.Messages[0].OfSystem.Content.OfString.Valid())
	assert.Equal(t, "You are a helpful assistant.", request.Messages[0].OfSystem.Content.OfString.Value)
	require.NotNil(t, request.Messages[1].OfUser)
	require.True(t, request.Messages[1].OfUser.Content.OfString.Valid())
	assert.Equal(t, "Say hello.", request.Messages[1].OfUser.Content.OfString.Value)
}

func TestClient_GenerateContent_WithImages(t *testing.T) {
	mockAPI := &mockChatCompletions{
		t: t,
		responses: []*openai.ChatCompletion{
			newChatCompletion(
				"vision",
				shared.ChatModelGPT4oMini,
				newChatCompletionMessage("Looks good!", nil),
				openai.CompletionUsage{},
			),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithChatClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	imageData := []byte{0x01, 0x02, 0x03}
	prompt := ai.Prompt{
		Name:      "vision",
		Text:      "Describe this image",
		ModelName: string(shared.ChatModelGPT4oMini),
		Images: []*ai.Image{
			{
				Type:     "image/jpeg",
				Filename: "sample.jpg",
				Data:     imageData,
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Looks good!", resp)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.requests, 1)
	request := mockAPI.requests[0]
	require.Len(t, request.Messages, 1)
	user := request.Messages[0].OfUser
	require.NotNil(t, user)
	require.True(t, user.Content.OfArrayOfContentParts != nil)
	parts := user.Content.OfArrayOfContentParts
	require.Len(t, parts, 2)
	require.NotNil(t, parts[0].OfText)
	assert.Equal(t, "Describe this image", parts[0].OfText.Text)
	require.NotNil(t, parts[1].OfImageURL)

	expectedDataURL := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(imageData))
	assert.Equal(t, expectedDataURL, parts[1].OfImageURL.ImageURL.URL)
}

func TestClient_GenerateContent_WithFunctionCall(t *testing.T) {
	toolCall := openai.ChatCompletionMessageToolCall{
		ID: "call_1",
		Function: openai.ChatCompletionMessageToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"location":"Lisbon"}`,
		},
		Type: constant.Function(""),
	}

	mockAPI := &mockChatCompletions{
		t: t,
		responses: []*openai.ChatCompletion{
			newChatCompletion(
				"tool-call",
				shared.ChatModelGPT4oMini,
				newChatCompletionMessage("", []openai.ChatCompletionMessageToolCall{toolCall}),
				openai.CompletionUsage{PromptTokens: 12, CompletionTokens: 4, TotalTokens: 16},
			),
			newChatCompletion(
				"final",
				shared.ChatModelGPT4oMini,
				newChatCompletionMessage("It is sunny and 22°C.", nil),
				openai.CompletionUsage{PromptTokens: 18, CompletionTokens: 6, TotalTokens: 24},
			),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithChatClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	handlerInvoked := false
	prompt := ai.Prompt{
		Name:      "weather",
		Text:      "What's the weather like?",
		ModelName: string(shared.ChatModelGPT4oMini),
		Functions: []*ai.FunctionDeclaration{
			{
				Name:        "get_weather",
				Description: "Look up the weather for a location",
				Parameters: &ai.Schema{
					Type: ai.TypeObject,
					Properties: map[string]*ai.Schema{
						"location": {
							Type:        ai.TypeString,
							Description: "City to query",
						},
					},
					Required: []string{"location"},
				},
			},
		},
		Handlers: map[string]ai.HandlerFunc{
			"get_weather": func(ctx context.Context, args map[string]any) (map[string]any, error) {
				handlerInvoked = true
				require.Equal(t, "Lisbon", args["location"])
				return map[string]any{
					"summary": "Sunny",
					"temp":    22,
				}, nil
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.True(t, handlerInvoked)
	assert.Equal(t, "It is sunny and 22°C.", resp)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.requests, 2)
	second := mockAPI.requests[1]
	require.NotNil(t, second.Messages[len(second.Messages)-1].OfTool)
	assert.Equal(t, "call_1", second.Messages[len(second.Messages)-1].OfTool.ToolCallID)
}

func TestClient_CountTokens(t *testing.T) {
	rawClient, err := NewClient(&events.NoOpEventBus{})
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:        "tokens",
		Instruction: "You are concise.",
		Text:        "Provide a short summary about Go programming language.",
		ModelName:   string(shared.ChatModelGPT4oMini),
		MaxTokens:   42,
	}

	count, err := client.CountTokens(context.Background(), prompt, false)
	require.NoError(t, err)
	require.NotNil(t, count)
	assert.Greater(t, count.TotalTokens, int32(0))
	assert.Equal(t, count.TotalTokens, count.InputTokens)
}

func TestClient_GenerateContent_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	rawClient, err := NewClient(&events.NoOpEventBus{})
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:      "missing-key",
		Text:      "Hello?",
		ModelName: string(shared.ChatModelGPT4oMini),
	}

	_, err = client.GenerateContent(context.Background(), prompt, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}
