package ollama

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// turnState drives one chat turn against the Ollama API for the shared
// agent loop. It owns the provider-native message history and appends
// assistant messages and tool results as the loop advances.
type turnState struct {
	client   *Client
	request  chatRequest
	messages []chatMessage
	// handlers are the prompt handlers with nil entries removed; the
	// same map is handed to the shared loop for execution.
	handlers map[string]ai.HandlerFunc
	// normalized maps normalized tool names onto registered handler
	// names so sloppy model output (case, whitespace, zero-width runes)
	// still resolves to the right tool.
	normalized map[string]string
	toolUsed   bool
}

func (c *Client) newTurn(prompt ai.Prompt) (*turnState, error) {
	request, err := c.buildChatRequest(prompt, normalMode)
	if err != nil {
		return nil, err
	}

	handlers := make(map[string]ai.HandlerFunc, len(prompt.Handlers))
	normalized := make(map[string]string, len(prompt.Handlers))
	for name, handler := range prompt.Handlers {
		if handler == nil {
			continue
		}
		handlers[name] = handler
		if norm := normalizeToolName(name); norm != "" {
			if _, exists := normalized[norm]; !exists {
				normalized[norm] = name
			}
		}
	}

	return &turnState{
		client:     c,
		request:    request,
		messages:   append([]chatMessage(nil), request.Messages...),
		handlers:   handlers,
		normalized: normalized,
	}, nil
}

// Step runs one model request. With emit set it streams; otherwise it
// performs a single blocking chat call.
func (t *turnState) Step(ctx context.Context, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	if emit != nil {
		return t.stepStreaming(ctx, emit)
	}
	return t.stepBlocking(ctx)
}

func (t *turnState) stepBlocking(ctx context.Context) (llmshared.StepOutcome, error) {
	c := t.client
	req := t.request
	req.Messages = t.messages
	req.Stream = false

	response, err := c.sendChat(ctx, req)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}

	c.PublishTokenCount(c.buildTokenCount(response))

	assistant := response.Message
	assistantContent := strings.TrimSpace(assistant.Content.Text())

	if len(assistant.ToolCalls) == 0 {
		if assistantContent == "" {
			// An empty answer is acceptable only after tool activity.
			if t.toolUsed {
				return llmshared.StepOutcome{}, nil
			}
			return llmshared.StepOutcome{}, ai.NonRetryable(errEmptyResponse)
		}
		return llmshared.StepOutcome{Text: assistantContent}, nil
	}

	t.toolUsed = true
	// Interim text alongside tool calls is surfaced as a notification;
	// in streaming mode it already reached the consumer through emit.
	if assistantContent != "" {
		notification := events.NotificationEvent{Message: assistantContent}
		c.EventBus.Publish(notification.Topic(), notification)
	}

	return t.recordToolCallStep(assistant.toChatMessage(), assistant.ToolCalls)
}

func (t *turnState) stepStreaming(ctx context.Context, emit func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	c := t.client
	req := t.request
	req.Messages = t.messages
	req.Stream = true

	var accumulatedText strings.Builder
	var accumulatedToolCalls []toolCall

	err := c.sendChatStream(ctx, req, func(resp *chatResponse) error {
		if resp.Error != "" {
			return fmt.Errorf("ollama error: %s", resp.Error)
		}

		if text := resp.Message.Content.Text(); text != "" {
			accumulatedText.WriteString(text)
			emit(&ai.StreamChunk{Text: text})
		}

		if len(resp.Message.ToolCalls) > 0 {
			accumulatedToolCalls = append(accumulatedToolCalls, resp.Message.ToolCalls...)
			chunk, err := toolCallsChunk(resp.Message.ToolCalls)
			if err != nil {
				return err
			}
			emit(chunk)
		}

		if resp.Done {
			tokenCount := c.buildTokenCount(resp)
			c.PublishTokenCount(tokenCount)
			if tokenCount != nil {
				emit(&ai.StreamChunk{TokenCount: tokenCount})
			}
		}

		return nil
	})
	if err != nil {
		return llmshared.StepOutcome{}, err
	}
	if err := ctx.Err(); err != nil {
		return llmshared.StepOutcome{}, err
	}

	if len(accumulatedToolCalls) == 0 {
		// The text already reached the consumer via emit.
		return llmshared.StepOutcome{Text: accumulatedText.String()}, nil
	}

	t.toolUsed = true
	assistantMessage := chatMessage{
		Role:    "assistant",
		Content: newMessageContentFromText(accumulatedText.String()),
	}
	return t.recordToolCallStep(assistantMessage, accumulatedToolCalls)
}

// recordToolCallStep dedupes the requested calls the same way the
// shared loop will, appends the assistant message carrying exactly the
// calls that will run, and converts them for the loop.
func (t *turnState) recordToolCallStep(assistantMessage chatMessage, calls []toolCall) (llmshared.StepOutcome, error) {
	if len(t.handlers) == 0 {
		return llmshared.StepOutcome{}, ai.NonRetryable(errToolCallNoHandler)
	}

	keptWire, converted, err := llmshared.DedupeChatToolCalls(calls, t.resolveHandlerName)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}

	assistantMessage.ToolCalls = keptWire
	t.messages = append(t.messages, assistantMessage)

	return llmshared.StepOutcome{ToolCalls: converted}, nil
}

// AddToolResults appends one tool message per executed call (plus any
// extracted media payloads) so the next step sees the results.
func (t *turnState) AddToolResults(_ context.Context, results []llmshared.ToolResult) error {
	messages, err := llmshared.BuildToolResultMessages(
		t.client.EventBus,
		results,
		func(callID, payload string) chatMessage {
			return chatMessage{
				Role:       "tool",
				Content:    newMessageContentFromText(payload),
				ToolCallID: callID,
			}
		},
		buildOllamaImageMessage,
		buildOllamaDocumentMessage,
	)
	if err != nil {
		return err
	}
	t.messages = append(t.messages, messages...)
	return nil
}

// resolveHandlerName maps the model's tool name onto a registered
// handler name, tolerating sloppy formatting via normalization.
func (t *turnState) resolveHandlerName(name string) string {
	if _, ok := t.handlers[name]; ok {
		return name
	}
	if registered, ok := t.normalized[normalizeToolName(name)]; ok {
		return registered
	}
	return name
}

// toolCallsChunk converts wire tool calls into a consumer-facing stream
// chunk. Argument parse failures abort the turn, as they always have.
func toolCallsChunk(calls []toolCall) (*ai.StreamChunk, error) {
	chunks := make([]*ai.ToolCallChunk, 0, len(calls))
	for _, call := range calls {
		args, err := call.Function.ArgumentsAsMap()
		if err != nil {
			return nil, ai.NonRetryable(err)
		}
		chunks = append(chunks, &ai.ToolCallChunk{
			ID:         call.ID,
			Name:       call.Function.Name,
			Parameters: args,
		})
	}
	return &ai.StreamChunk{ToolCalls: chunks}, nil
}
