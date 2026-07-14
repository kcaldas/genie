package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
	openai_constant "github.com/openai/openai-go/shared/constant"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
)

// turnState drives one chat turn against the OpenAI Chat Completions
// API for the shared agent loop. It owns the provider-native message
// history ([]openai.ChatCompletionMessageParamUnion) and appends
// assistant messages and tool results as the loop advances.
type turnState struct {
	client   *Client
	params   openai.ChatCompletionNewParams
	messages []openai.ChatCompletionMessageParamUnion
	toolUsed bool
}

func (c *Client) newTurn(prompt ai.Prompt) (*turnState, error) {
	modelName := c.resolveModelName(prompt.ModelName)
	messages, _, err := c.buildMessages(prompt)
	if err != nil {
		return nil, err
	}

	params := openai.ChatCompletionNewParams{
		Model: shared.ChatModel(modelName),
	}
	c.applyGenerationConfig(&params, prompt)

	return &turnState{client: c, params: params, messages: messages}, nil
}

// Step runs one model request. With emit set it streams; otherwise it
// performs a single blocking completion call.
func (t *turnState) Step(ctx context.Context, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	params := t.params
	params.Messages = t.messages

	if emit != nil {
		return t.stepStreaming(ctx, params, emit)
	}
	return t.stepBlocking(ctx, params)
}

func (t *turnState) stepBlocking(ctx context.Context, params openai.ChatCompletionNewParams) (llmshared.StepOutcome, error) {
	c := t.client

	resp, err := c.chatCompletions.New(ctx, params)
	if err != nil {
		return llmshared.StepOutcome{}, fmt.Errorf("openai chat completion: %w", err)
	}

	c.publishUsage(string(params.Model), resp.Usage)

	if len(resp.Choices) == 0 {
		if t.toolUsed {
			return llmshared.StepOutcome{}, nil
		}
		return llmshared.StepOutcome{}, errors.New("openai chat completion returned no choices")
	}

	assistantMessage := resp.Choices[0].Message
	content := strings.TrimSpace(assistantMessage.Content)
	hasToolCalls := len(assistantMessage.ToolCalls) > 0

	// Interim assistant text before tool calls is surfaced as a
	// notification in blocking mode; in streaming mode the text reaches
	// the consumer through emit instead.
	if hasToolCalls && content != "" {
		notification := events.NotificationEvent{Message: content}
		c.eventBus.Publish(notification.Topic(), notification)
	}

	t.messages = append(t.messages, assistantMessage.ToParam())

	if !hasToolCalls {
		if content == "" {
			if t.toolUsed {
				return llmshared.StepOutcome{}, nil
			}
			return llmshared.StepOutcome{}, errors.New("openai returned an empty response")
		}
		return llmshared.StepOutcome{Text: content}, nil
	}

	t.toolUsed = true
	calls, err := toSharedToolCalls(assistantMessage.ToolCalls)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}
	return llmshared.StepOutcome{ToolCalls: calls}, nil
}

// toolCallState accumulates one tool call across streaming deltas.
type toolCallState struct {
	id        string
	name      string
	arguments strings.Builder
}

func (t *turnState) stepStreaming(ctx context.Context, params openai.ChatCompletionNewParams, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	c := t.client

	stream := c.chatCompletions.NewStreaming(ctx, params)
	defer stream.Close()

	var assistantBuilder strings.Builder
	toolStates := make(map[int64]*toolCallState)
	toolOrder := make([]int64, 0, 2)
	seenToolIndex := make(map[int64]bool)
	var finishReason string
	var lastUsage openai.CompletionUsage

	for stream.Next() {
		chunk := stream.Current()
		if chunk.Usage.TotalTokens != 0 || chunk.Usage.PromptTokens != 0 || chunk.Usage.CompletionTokens != 0 {
			lastUsage = chunk.Usage
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			emit(&ai.StreamChunk{Text: choice.Delta.Content})
			assistantBuilder.WriteString(choice.Delta.Content)
		}
		// Some OpenAI-compatible servers still stream legacy `function_call`
		// deltas (replaced by `tool_calls`). The SDK only exposes them through a
		// deprecated field, so read the raw JSON metadata instead.
		if fcField := choice.Delta.JSON.FunctionCall; fcField.Valid() {
			var functionCall struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}
			if err := json.Unmarshal([]byte(fcField.Raw()), &functionCall); err == nil &&
				(functionCall.Name != "" || functionCall.Arguments != "") {
				index := int64(0)
				state := toolStates[index]
				if state == nil {
					state = &toolCallState{}
					toolStates[index] = state
				}
				if !seenToolIndex[index] {
					toolOrder = append(toolOrder, index)
					seenToolIndex[index] = true
				}
				if functionCall.Name != "" {
					state.name = functionCall.Name
				}
				if functionCall.Arguments != "" {
					state.arguments.WriteString(functionCall.Arguments)
				}
			}
		}
		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				state := toolStates[tc.Index]
				if state == nil {
					state = &toolCallState{}
					toolStates[tc.Index] = state
				}
				if !seenToolIndex[tc.Index] {
					toolOrder = append(toolOrder, tc.Index)
					seenToolIndex[tc.Index] = true
				}
				if tc.ID != "" {
					state.id = tc.ID
				}
				if tc.Function.Name != "" {
					state.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					state.arguments.WriteString(tc.Function.Arguments)
				}
			}
		}
		if choice.FinishReason != "" {
			finishReason = choice.FinishReason
		}
	}

	if err := stream.Err(); err != nil {
		return llmshared.StepOutcome{}, fmt.Errorf("openai chat completion stream: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return llmshared.StepOutcome{}, err
	}

	if lastUsage.TotalTokens != 0 || lastUsage.PromptTokens != 0 || lastUsage.CompletionTokens != 0 {
		c.publishUsage(string(params.Model), lastUsage)
		emit(&ai.StreamChunk{
			TokenCount: &ai.TokenCount{
				TotalTokens:  int32(lastUsage.TotalTokens),
				InputTokens:  int32(lastUsage.PromptTokens),
				OutputTokens: int32(lastUsage.CompletionTokens),
			},
		})
	}

	assistantParam := openai.ChatCompletionAssistantMessageParam{
		Content: openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(assistantBuilder.String()),
		},
	}

	var toolCalls []openai.ChatCompletionMessageToolCall
	if len(toolStates) > 0 {
		toolCalls = buildToolCalls(toolOrder, toolStates)
		assistantParam.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(toolCalls))
		for i, call := range toolCalls {
			// call.ToParam() relies on raw JSON captured from the API response.
			// Since we construct these tool calls ourselves while streaming, the raw
			// JSON is empty and would fail to marshal. Build the param struct
			// directly to ensure valid JSON encoding.
			assistantParam.ToolCalls[i] = openai.ChatCompletionMessageToolCallParam{
				ID: call.ID,
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
				Type: call.Type,
			}
		}
		emit(toolCallChunk(toolCalls))
	}

	t.messages = append(t.messages, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistantParam})

	if len(toolCalls) == 0 {
		text := assistantBuilder.String()
		if strings.TrimSpace(text) == "" && finishReason == "" {
			return llmshared.StepOutcome{}, errors.New("openai returned an empty response")
		}
		// The text already reached the consumer via emit.
		return llmshared.StepOutcome{Text: text}, nil
	}

	t.toolUsed = true
	calls, err := toSharedToolCalls(toolCalls)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}
	return llmshared.StepOutcome{ToolCalls: calls}, nil
}

// AddToolResults converts executed tool results into tool-role messages
// correlated by tool_call_id, plus follow-up user messages for media
// payloads (images, documents).
func (t *turnState) AddToolResults(ctx context.Context, results []llmshared.ToolResult) error {
	c := t.client

	for _, result := range results {
		handlerResp := result.Result
		if result.Err != nil {
			// Return the error to the model as a tool result so it can
			// recover (apologise, retry with different args, escalate)
			// instead of aborting the conversation. We still log the
			// failure for ops visibility.
			c.eventBus.Publish(events.NotificationEvent{}.Topic(), events.NotificationEvent{
				Message: fmt.Sprintf("tool %s returned error: %v", result.Call.Name, result.Err),
			})
			handlerResp = map[string]any{
				"error": fmt.Sprintf("function %q returned an error: %v", result.Call.Name, result.Err),
			}
		}

		var media *toolpayload.Payload
		switch result.Call.Name {
		case "viewImage", "viewDocument":
			extracted, sanitized, err := toolpayload.Extract(handlerResp)
			if err != nil {
				return fmt.Errorf("invalid %s response: %w", result.Call.Name, err)
			}
			handlerResp = sanitized
			media = extracted
		}

		payload, err := json.Marshal(handlerResp)
		if err != nil {
			return fmt.Errorf("unable to marshal response for function %q: %w", result.Call.Name, err)
		}
		t.messages = append(t.messages, openai.ToolMessage(string(payload), result.Call.ID))

		if media != nil {
			switch result.Call.Name {
			case "viewImage":
				t.messages = append(t.messages, buildImageUserMessage(media))
			case "viewDocument":
				t.messages = append(t.messages, buildDocumentUserMessage(media))
			}
		}
	}

	return nil
}

// buildToolCalls assembles the tool calls accumulated while streaming
// in the order their indices first appeared.
func buildToolCalls(order []int64, states map[int64]*toolCallState) []openai.ChatCompletionMessageToolCall {
	calls := make([]openai.ChatCompletionMessageToolCall, 0, len(states))
	for _, idx := range order {
		state := states[idx]
		if state == nil {
			continue
		}
		calls = append(calls, openai.ChatCompletionMessageToolCall{
			ID: state.id,
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      state.name,
				Arguments: state.arguments.String(),
			},
			Type: openai_constant.Function("function"),
		})
	}
	return calls
}

// toSharedToolCalls converts provider tool calls into the shared loop's
// neutral representation, parsing the argument JSON strictly: a call
// whose arguments cannot be parsed fails the step, as it did in the
// per-provider loop.
func toSharedToolCalls(calls []openai.ChatCompletionMessageToolCall) ([]llmshared.ToolCall, error) {
	out := make([]llmshared.ToolCall, 0, len(calls))
	for _, call := range calls {
		args := map[string]any{}
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("invalid arguments for function %q: %w", call.Function.Name, err)
			}
		}
		out = append(out, llmshared.ToolCall{ID: call.ID, Name: call.Function.Name, Args: args})
	}
	return out, nil
}

// toolCallChunk converts the step's tool calls into the stream chunk
// surfaced to consumers, parsing arguments leniently.
func toolCallChunk(calls []openai.ChatCompletionMessageToolCall) *ai.StreamChunk {
	toolChunks := make([]*ai.ToolCallChunk, 0, len(calls))
	for _, call := range calls {
		var params map[string]any
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &params); err != nil {
				params = map[string]any{
					"raw": call.Function.Arguments,
				}
			}
		}
		toolChunks = append(toolChunks, &ai.ToolCallChunk{
			ID:         call.ID,
			Name:       call.Function.Name,
			Parameters: params,
		})
	}
	return &ai.StreamChunk{ToolCalls: toolChunks}
}
