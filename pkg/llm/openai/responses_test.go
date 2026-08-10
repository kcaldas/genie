package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

type mockResponses struct {
	t         *testing.T
	mu        sync.Mutex
	requests  []responses.ResponseNewParams
	responses []*responses.Response
	streams   []string
	err       error
}

func (m *mockResponses) New(ctx context.Context, params responses.ResponseNewParams, _ ...option.RequestOption) (*responses.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = append(m.requests, params)

	if m.err != nil {
		return nil, m.err
	}
	if len(m.responses) == 0 {
		require.FailNow(m.t, "mock responses received more calls than configured responses")
	}

	resp := m.responses[0]
	m.responses = m.responses[1:]
	return resp, nil
}

func (m *mockResponses) NewStreaming(ctx context.Context, params responses.ResponseNewParams, _ ...option.RequestOption) *ssestream.Stream[responses.ResponseStreamEventUnion] {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = append(m.requests, params)

	if len(m.streams) == 0 {
		require.FailNow(m.t, "mock responses stream received more calls than configured streams")
	}

	body := m.streams[0]
	m.streams = m.streams[1:]
	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:   io.NopCloser(strings.NewReader(body)),
	}
	return ssestream.NewStream[responses.ResponseStreamEventUnion](ssestream.NewDecoder(resp), nil)
}

func responseFromJSON(t *testing.T, raw string) *responses.Response {
	t.Helper()
	var resp responses.Response
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	return &resp
}

func responseJSON(model string, output string, usage responses.ResponseUsage) string {
	return fmt.Sprintf(`{
		"id": "resp_final",
		"object": "response",
		"created_at": 0,
		"model": %q,
		"status": "completed",
		"output": [%s],
		"usage": {
			"input_tokens": %d,
			"input_tokens_details": {"cached_tokens": %d},
			"output_tokens": %d,
			"output_tokens_details": {"reasoning_tokens": %d},
			"total_tokens": %d
		}
	}`, model, output, usage.InputTokens, usage.InputTokensDetails.CachedTokens, usage.OutputTokens, usage.OutputTokensDetails.ReasoningTokens, usage.TotalTokens)
}

func outputMessageJSON(id, text string) string {
	return fmt.Sprintf(`{
		"type": "message",
		"id": %q,
		"role": "assistant",
		"status": "completed",
		"content": [{"type": "output_text", "text": %q, "annotations": []}]
	}`, id, text)
}

func outputFunctionCallJSON(id, callID, name, args string) string {
	return fmt.Sprintf(`{
		"type": "function_call",
		"id": %q,
		"call_id": %q,
		"name": %q,
		"arguments": %q,
		"status": "completed"
	}`, id, callID, name, args)
}

func outputReasoningJSON(id, encrypted string) string {
	return fmt.Sprintf(`{
		"type": "reasoning",
		"id": %q,
		"summary": [],
		"encrypted_content": %q,
		"status": "completed"
	}`, id, encrypted)
}

func sseEvent(raw string) string {
	return "data: " + strings.ReplaceAll(raw, "\n", "") + "\n\n"
}

func TestMapResponseFunctions_DisablesStrictValidation(t *testing.T) {
	tools := mapResponseFunctions([]*ai.FunctionDeclaration{{
		Name: "tool_with_optional_argument",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"required": {Type: ai.TypeString},
				"optional": {Type: ai.TypeString},
			},
			Required: []string{"required"},
		},
	}})

	require.Len(t, tools, 1)
	require.NotNil(t, tools[0].OfFunction)
	require.True(t, tools[0].OfFunction.Strict.Valid())
	assert.False(t, tools[0].OfFunction.Strict.Value)

	wire, err := json.Marshal(tools[0])
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type": "function",
		"name": "tool_with_optional_argument",
		"strict": false,
		"parameters": {
			"type": "object",
			"properties": {
				"required": {"type": "string"},
				"optional": {"type": "string"}
			},
			"required": ["required"]
		}
	}`, string(wire))
}

func TestClient_Responses_InstructionsAndInputAssembly(t *testing.T) {
	model := "gpt-5.6-luna"
	mockAPI := &mockResponses{
		t: t,
		responses: []*responses.Response{
			responseFromJSON(t, responseJSON(model, outputMessageJSON("msg_1", "Hello there!"), responses.ResponseUsage{})),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithResponsesClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	imageData := []byte{0x01, 0x02, 0x03}
	prompt := ai.Prompt{
		Name:                    "greeting",
		Instruction:             "Instruction",
		SystemPromptFiles:       "Files",
		SystemPromptUserContext: "User context",
		Text:                    "Say hello.",
		ModelName:               model,
		Images: []*ai.Image{{
			Type: "image/jpeg",
			Data: imageData,
		}},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", resp)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.requests, 1)
	request := mockAPI.requests[0]
	assert.Equal(t, shared.ResponsesModel(model), request.Model)
	require.True(t, request.Instructions.Valid())
	assert.Equal(t, "Instruction\n\nFiles\n\nUser context", request.Instructions.Value)
	require.True(t, request.Store.Valid())
	assert.False(t, request.Store.Value)
	assert.False(t, request.PreviousResponseID.Valid())
	assert.Equal(t, []responses.ResponseIncludable{responses.ResponseIncludableReasoningEncryptedContent}, request.Include)

	input := request.Input.OfInputItemList
	require.Len(t, input, 1)
	require.NotNil(t, input[0].OfInputMessage)
	assert.Equal(t, "user", input[0].OfInputMessage.Role)
	parts := input[0].OfInputMessage.Content
	require.Len(t, parts, 2)
	require.NotNil(t, parts[0].OfInputText)
	assert.Equal(t, "Say hello.", parts[0].OfInputText.Text)
	require.NotNil(t, parts[1].OfInputImage)
	expectedDataURL := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(imageData))
	require.True(t, parts[1].OfInputImage.ImageURL.Valid())
	assert.Equal(t, expectedDataURL, parts[1].OfInputImage.ImageURL.Value)
}

func TestClient_Responses_FunctionCallOutputCorrelatedByCallIDAndReasoningReplayed(t *testing.T) {
	model := "gpt-5.6-luna"
	firstOutput := strings.Join([]string{
		outputReasoningJSON("rs_1", "encrypted-reasoning"),
		outputFunctionCallJSON("fc_1", "call_1", "get_weather", `{"location":"Lisbon"}`),
	}, ",")
	mockAPI := &mockResponses{
		t: t,
		responses: []*responses.Response{
			responseFromJSON(t, responseJSON(model, firstOutput, responses.ResponseUsage{})),
			responseFromJSON(t, responseJSON(model, outputMessageJSON("msg_2", "It is sunny."), responses.ResponseUsage{})),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithResponsesClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	handlerInvoked := false
	prompt := ai.Prompt{
		Name:      "weather",
		Text:      "Weather?",
		ModelName: model,
		Functions: []*ai.FunctionDeclaration{{
			Name: "get_weather",
			Parameters: &ai.Schema{
				Type: ai.TypeObject,
			},
		}},
		Handlers: map[string]ai.HandlerFunc{
			"get_weather": func(ctx context.Context, args map[string]any) (map[string]any, error) {
				handlerInvoked = true
				require.Equal(t, "Lisbon", args["location"])
				return map[string]any{"summary": "Sunny"}, nil
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.True(t, handlerInvoked)
	assert.Equal(t, "It is sunny.", resp)

	mockAPI.mu.Lock()
	defer mockAPI.mu.Unlock()
	require.Len(t, mockAPI.requests, 2)
	secondInput := mockAPI.requests[1].Input.OfInputItemList
	require.GreaterOrEqual(t, len(secondInput), 4)
	require.NotNil(t, secondInput[1].OfReasoning)
	require.True(t, secondInput[1].OfReasoning.EncryptedContent.Valid())
	assert.Equal(t, "encrypted-reasoning", secondInput[1].OfReasoning.EncryptedContent.Value)
	require.NotNil(t, secondInput[2].OfFunctionCall)
	assert.Equal(t, "call_1", secondInput[2].OfFunctionCall.CallID)
	require.NotNil(t, secondInput[3].OfFunctionCallOutput)
	assert.Equal(t, "call_1", secondInput[3].OfFunctionCallOutput.CallID)
	assert.JSONEq(t, `{"summary":"Sunny"}`, secondInput[3].OfFunctionCallOutput.Output)
}

func TestClient_Responses_StreamingOrder(t *testing.T) {
	model := "gpt-5.6-luna"
	usage := responses.ResponseUsage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12}
	firstCompleted := responseJSON(model, outputFunctionCallJSON("fc_1", "call_1", "run_tool", `{"task":"go"}`), usage)
	secondCompleted := responseJSON(model, outputMessageJSON("msg_2", "done"), usage)
	mockAPI := &mockResponses{
		t: t,
		streams: []string{
			sseEvent(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"checking","sequence_number":1}`) +
				sseEvent(fmt.Sprintf(`{"type":"response.completed","sequence_number":2,"response":%s}`, firstCompleted)),
			sseEvent(`{"type":"response.output_text.delta","item_id":"msg_2","output_index":0,"content_index":0,"delta":"done","sequence_number":1}`) +
				sseEvent(fmt.Sprintf(`{"type":"response.completed","sequence_number":2,"response":%s}`, secondCompleted)),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithResponsesClient(mockAPI))
	require.NoError(t, err)
	client := rawClient.(*Client)

	stream, err := client.GenerateContentStream(context.Background(), ai.Prompt{
		Text:      "Run it.",
		ModelName: model,
		Functions: []*ai.FunctionDeclaration{{
			Name:       "run_tool",
			Parameters: &ai.Schema{Type: ai.TypeObject},
		}},
		Handlers: map[string]ai.HandlerFunc{
			"run_tool": func(ctx context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"ok": true}, nil
			},
		},
	}, false)
	require.NoError(t, err)
	defer stream.Close()

	var got []string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		switch {
		case chunk.Text != "":
			got = append(got, "text:"+chunk.Text)
		case chunk.TokenCount != nil:
			got = append(got, "usage")
		case len(chunk.ToolCalls) > 0:
			got = append(got, "tool:"+chunk.ToolCalls[0].ID)
		}
	}

	assert.Equal(t, []string{"text:checking", "usage", "tool:call_1", "text:done", "usage"}, got)
}

func TestClient_TransportSelectionByModelGeneration(t *testing.T) {
	chatMock := &mockChatCompletions{
		t: t,
		responses: []*openai.ChatCompletion{
			newChatCompletion("chat", shared.ChatModelGPT4oMini, newChatCompletionMessage("chat path", nil), openai.CompletionUsage{}),
		},
	}
	responsesMock := &mockResponses{
		t: t,
		responses: []*responses.Response{
			responseFromJSON(t, responseJSON("gpt-5", outputMessageJSON("msg_1", "responses path"), responses.ResponseUsage{})),
		},
	}

	rawClient, err := NewClient(&events.NoOpEventBus{}, WithChatClient(chatMock), WithResponsesClient(responsesMock))
	require.NoError(t, err)
	client := rawClient.(*Client)

	chatResp, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: string(shared.ChatModelGPT4oMini)}, false)
	require.NoError(t, err)
	assert.Equal(t, "chat path", chatResp)

	responsesResp, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "gpt-5"}, false)
	require.NoError(t, err)
	assert.Equal(t, "responses path", responsesResp)

	chatMock.mu.Lock()
	require.Len(t, chatMock.requests, 1)
	chatMock.mu.Unlock()
	responsesMock.mu.Lock()
	require.Len(t, responsesMock.requests, 1)
	responsesMock.mu.Unlock()
}
