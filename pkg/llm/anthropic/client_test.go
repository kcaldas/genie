package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

type mockMessageClient struct {
	t             *testing.T
	mu            sync.Mutex
	requests      []anthropic_sdk.MessageNewParams
	countRequests []anthropic_sdk.MessageCountTokensParams
	responses     []*anthropic_sdk.Message
	countResponse *anthropic_sdk.MessageTokensCount
	err           error
	countErr      error
}

func (m *mockMessageClient) New(ctx context.Context, body anthropic_sdk.MessageNewParams, _ ...option.RequestOption) (*anthropic_sdk.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = append(m.requests, body)

	if m.err != nil {
		return nil, m.err
	}

	if len(m.responses) == 0 {
		require.FailNow(m.t, "mock message client received more calls than configured responses")
	}

	resp := m.responses[0]
	m.responses = m.responses[1:]
	return resp, nil
}

func (m *mockMessageClient) CountTokens(ctx context.Context, body anthropic_sdk.MessageCountTokensParams, _ ...option.RequestOption) (*anthropic_sdk.MessageTokensCount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.countRequests = append(m.countRequests, body)
	if m.countErr != nil {
		return nil, m.countErr
	}
	if m.countResponse == nil {
		require.FailNow(m.t, "mock message client has no count token response configured")
	}
	return m.countResponse, nil
}

func TestClient_GenerateContent_SimpleResponse(t *testing.T) {
	mockAPI := &mockMessageClient{
		t: t,
		responses: []*anthropic_sdk.Message{
			newTextMessage("msg-1", "Hello there!"),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithMessageClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:        "greeting",
		Instruction: "You are a helpful assistant.",
		Text:        "Say hello.",
		ModelName:   "claude-3-5-sonnet-20241022",
		MaxTokens:   256,
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", resp)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()

	require.Len(t, mockAPI.requests, 1)
	request := mockAPI.requests[0]
	assert.Equal(t, anthropic_sdk.Model("claude-3-5-sonnet-20241022"), request.Model)
	assert.Equal(t, int64(256), request.MaxTokens)
	require.Len(t, request.Messages, 1)
	require.Len(t, request.System, 1)
	assert.Equal(t, "You are a helpful assistant.", request.System[0].Text)
	require.NotNil(t, request.Messages[0].Content[0].OfText)
	assert.Equal(t, "Say hello.", request.Messages[0].Content[0].OfText.Text)
}

func TestClient_GenerateContent_WithImages(t *testing.T) {
	mockAPI := &mockMessageClient{
		t: t,
		responses: []*anthropic_sdk.Message{
			newTextMessage("vision", "Done"),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithMessageClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	imageData := []byte{0x04, 0x05, 0x06}
	prompt := ai.Prompt{
		Name:      "vision",
		Text:      "Describe the image",
		ModelName: "claude-3-5-sonnet-20241022",
		Images: []*ai.Image{
			{
				Type:     "image/png",
				Filename: "sample.png",
				Data:     imageData,
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Done", resp)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.requests, 1)
	request := mockAPI.requests[0]
	require.Len(t, request.Messages, 1)
	user := request.Messages[0]
	require.Len(t, user.Content, 2)
	require.NotNil(t, user.Content[0].OfText)
	assert.Equal(t, "Describe the image", user.Content[0].OfText.Text)
	require.NotNil(t, user.Content[1].OfImage)
	imageBlock := user.Content[1].OfImage
	require.NotNil(t, imageBlock.Source.OfBase64)
	assert.Equal(t, "image/png", string(imageBlock.Source.OfBase64.MediaType))
	expected := base64.StdEncoding.EncodeToString(imageData)
	assert.Equal(t, expected, imageBlock.Source.OfBase64.Data)
}

func TestClient_GenerateContent_WithToolCall(t *testing.T) {
	toolInput := json.RawMessage(`{"location":"Lisbon"}`)
	mockAPI := &mockMessageClient{
		t: t,
		responses: []*anthropic_sdk.Message{
			{
				ID:         "call",
				Content:    []anthropic_sdk.ContentBlockUnion{{Type: "tool_use", ID: "call_1", Name: "get_weather", Input: toolInput}},
				Model:      anthropic_sdk.Model("claude-3-5-sonnet-20241022"),
				Role:       constant.Assistant(""),
				StopReason: anthropic_sdk.StopReasonToolUse,
				Type:       constant.Message(""),
			},
			newTextMessage("final", "It is sunny and 22°C."),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithMessageClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	handlerInvoked := false
	prompt := ai.Prompt{
		Name:      "weather",
		Text:      "What's the weather like?",
		ModelName: "claude-3-5-sonnet-20241022",
		MaxTokens: 256,
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
	require.GreaterOrEqual(t, len(mockAPI.requests), 2)
	second := mockAPI.requests[1]
	require.True(t, len(second.Messages) >= 3)
	last := second.Messages[len(second.Messages)-1]
	require.Equal(t, anthropic_sdk.MessageParamRoleUser, last.Role)
	require.Len(t, last.Content, 1)
	require.NotNil(t, last.Content[0].OfToolResult)
	assert.Equal(t, "call_1", last.Content[0].OfToolResult.ToolUseID)
}

func TestClient_CountTokens(t *testing.T) {
	mockAPI := &mockMessageClient{
		t:             t,
		countResponse: &anthropic_sdk.MessageTokensCount{InputTokens: 15},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithMessageClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:        "tokens",
		Instruction: "You are concise.",
		Text:        "Provide a short summary about Go programming language.",
		ModelName:   "claude-3-5-sonnet-20241022",
		MaxTokens:   512,
	}

	count, err := client.CountTokens(context.Background(), prompt, false)
	require.NoError(t, err)
	require.NotNil(t, count)
	assert.Equal(t, int32(15), count.TotalTokens)
	assert.Equal(t, int32(15), count.InputTokens)
	assert.Equal(t, int32(0), count.OutputTokens)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.countRequests, 1)
	request := mockAPI.countRequests[0]
	assert.Equal(t, anthropic_sdk.Model("claude-3-5-sonnet-20241022"), request.Model)
	require.Len(t, request.Messages, 1)
}

func TestClient_GenerateContent_MissingAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	rawClient, err := NewClient(&events.NoOpEventBus{})
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:      "missing-key",
		Text:      "Hello?",
		ModelName: "claude-3-5-sonnet-20241022",
		MaxTokens: 64,
	}

	_, err = client.GenerateContent(context.Background(), prompt, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
}

func newTextMessage(id string, text string) *anthropic_sdk.Message {
	return &anthropic_sdk.Message{
		ID: id,
		Content: []anthropic_sdk.ContentBlockUnion{
			{
				Type: "text",
				Text: text,
			},
		},
		Model:      anthropic_sdk.Model("claude-3-5-sonnet-20241022"),
		Role:       constant.Assistant(""),
		StopReason: anthropic_sdk.StopReasonEndTurn,
		Type:       constant.Message(""),
		Usage: anthropic_sdk.Usage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}
}

func TestClient_GenerateContent_ToolOnlyEmptyResponse(t *testing.T) {
	toolInput := json.RawMessage(`{"task":"run"}`)
	mockAPI := &mockMessageClient{
		t: t,
		responses: []*anthropic_sdk.Message{
			{
				ID:         "tool",
				Content:    []anthropic_sdk.ContentBlockUnion{{Type: "tool_use", ID: "call_1", Name: "run_tool", Input: toolInput}},
				Model:      anthropic_sdk.Model("claude-3-5-sonnet-20241022"),
				Role:       constant.Assistant(""),
				StopReason: anthropic_sdk.StopReasonToolUse,
				Type:       constant.Message(""),
			},
			newTextMessage("final", ""),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithMessageClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	handlerInvoked := false
	prompt := ai.Prompt{
		Name:      "tools-only",
		Text:      "Do the task",
		ModelName: "claude-3-5-sonnet-20241022",
		Functions: []*ai.FunctionDeclaration{
			{
				Name: "run_tool",
				Parameters: &ai.Schema{
					Type: ai.TypeObject,
				},
			},
		},
		Handlers: map[string]ai.HandlerFunc{
			"run_tool": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
				handlerInvoked = true
				return map[string]any{"status": "ok"}, nil
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.True(t, handlerInvoked)
	assert.Empty(t, resp)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.requests, 2)
}

func TestClient_GenerateContent_EmptyResponseWithoutTools(t *testing.T) {
	mockAPI := &mockMessageClient{
		t: t,
		responses: []*anthropic_sdk.Message{
			newTextMessage("empty", ""),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithMessageClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:      "empty",
		Text:      "Say something",
		ModelName: "claude-3-5-sonnet-20241022",
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.Error(t, err)
	assert.Empty(t, resp)
	assert.Contains(t, err.Error(), "anthropic returned an empty response")

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.requests, 1)
}
