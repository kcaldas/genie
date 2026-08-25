package openaicompat

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// TurnOptions carries the provider-specific hooks of a chat turn.
type TurnOptions struct {
	// SupportsBlob reports whether a tool-result blob can be sent
	// natively. Nil means the provider is text-only: blobs are
	// described in the tool-result text instead.
	SupportsBlob func(ai.BlobContent) bool
	// BlobMessage converts a supported blob into a provider message
	// appended after the tool result. Nil disables native blobs even
	// when SupportsBlob accepts them.
	BlobMessage func(ai.BlobContent) ChatMessage
}

// Turn drives one chat turn for the shared agent loop. It owns the
// provider-native message history and appends assistant messages and
// tool results as the loop advances.
type Turn struct {
	core     *Core
	request  ChatRequest
	messages []ChatMessage
	// handlers are the prompt handlers with nil entries removed; the
	// same map is handed to the shared loop for execution.
	handlers map[string]ai.HandlerFunc
	toolUsed bool
	options  TurnOptions
}

var _ llmshared.TurnState = (*Turn)(nil)

// NewTurn starts a turn from a fully built request. Nil handler map
// entries are dropped.
func (c *Core) NewTurn(request ChatRequest, handlers map[string]ai.HandlerFunc, options TurnOptions) *Turn {
	kept := make(map[string]ai.HandlerFunc, len(handlers))
	for name, handler := range handlers {
		if handler != nil {
			kept[name] = handler
		}
	}

	return &Turn{
		core:     c,
		request:  request,
		messages: append([]ChatMessage(nil), request.Messages...),
		handlers: kept,
		options:  options,
	}
}

// Handlers exposes the turn's executable handler map for the shared loop.
func (t *Turn) Handlers() map[string]ai.HandlerFunc {
	return t.handlers
}

// Step runs one blocking model request. The streaming API is never used
// inside the tool loop (the streaming entry point either streams a
// tool-free request directly or wraps the blocking loop), so emit is
// unused.
func (t *Turn) Step(ctx context.Context, _ func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	c := t.core
	req := t.request
	req.Messages = t.messages

	response, err := c.SendChat(ctx, req)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}

	usage := c.PublishUsage(ctx, req.Model, response.Usage)

	if len(response.Choices) == 0 {
		return llmshared.StepOutcome{}, t.emptyResponseError()
	}

	assistant := response.Choices[0].Message
	if reasoning := strings.TrimSpace(assistant.ReasoningText()); reasoning != "" {
		thinkingEvent := events.ThinkingEvent{Text: reasoning}
		c.EventBus.PublishSync(thinkingEvent.Topic(), thinkingEvent)
	}
	assistantContent := strings.TrimSpace(assistant.Content.Text())

	if len(assistant.ToolCalls) == 0 {
		if assistantContent == "" {
			// An empty answer is acceptable only after tool activity.
			if t.toolUsed {
				return llmshared.StepOutcome{}, nil
			}
			return llmshared.StepOutcome{}, t.emptyResponseError()
		}
		return llmshared.StepOutcome{Text: assistantContent, Usage: usage}, nil
	}

	t.toolUsed = true
	// Interim text alongside tool calls is surfaced as a notification.
	if assistantContent != "" {
		notification := events.NotificationEvent{Message: assistantContent}
		c.EventBus.Publish(notification.Topic(), notification)
	}

	if len(t.handlers) == 0 {
		return llmshared.StepOutcome{}, ai.NonRetryable(fmt.Errorf("%s: %w", c.Provider, ErrToolCallNoHandler))
	}

	// Dedupe the requested calls the same way the shared loop will, so
	// the recorded assistant message carries exactly the calls that run.
	keptWire, converted, err := llmshared.DedupeChatToolCalls(assistant.ToolCalls, nil)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}

	// ToChatMessage drops reasoning payloads: some APIs reject them in
	// input messages.
	message := assistant.ToChatMessage()
	message.ToolCalls = keptWire
	t.messages = append(t.messages, message)

	return llmshared.StepOutcome{ToolCalls: converted, Usage: usage}, nil
}

// AddToolResults appends one tool message per executed call so the next
// step sees the results. Blobs the provider supports are appended as
// provider messages via the BlobMessage hook; everything else stays as
// textual descriptions.
func (t *Turn) AddToolResults(_ context.Context, results []llmshared.PreparedToolResult) error {
	supports := t.options.SupportsBlob
	if t.options.BlobMessage == nil {
		supports = nil
	}
	if supports == nil {
		supports = func(ai.BlobContent) bool { return false }
	}

	for _, result := range results {
		encoded := llmshared.EncodeToolResult(result, supports)
		t.messages = append(t.messages, ChatMessage{
			Role:       "tool",
			Content:    NewMessageContentFromText(encoded.Text),
			ToolCallID: result.Call.ID,
		})
		for _, blob := range encoded.Blobs {
			t.messages = append(t.messages, t.options.BlobMessage(blob))
		}
	}
	return nil
}

func (t *Turn) emptyResponseError() error {
	return ai.NonRetryable(fmt.Errorf("%s: %w", t.core.Provider, ErrEmptyResponse))
}
