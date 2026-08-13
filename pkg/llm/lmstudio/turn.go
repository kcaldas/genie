package lmstudio

import (
	"context"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// turnState drives one chat turn against the LM Studio API for the
// shared agent loop. It owns the provider-native message history and
// appends assistant messages and tool results as the loop advances.
type turnState struct {
	client   *Client
	request  chatRequest
	messages []chatMessage
	// handlers are the prompt handlers with nil entries removed; the
	// same map is handed to the shared loop for execution.
	handlers     map[string]ai.HandlerFunc
	toolUsed     bool
	supportsBlob func(ai.BlobContent) bool
}

func (c *Client) newTurn(prompt ai.Prompt) (*turnState, error) {
	request, err := c.buildChatRequest(prompt, normalMode)
	if err != nil {
		return nil, err
	}

	handlers := make(map[string]ai.HandlerFunc, len(prompt.Handlers))
	for name, handler := range prompt.Handlers {
		if handler != nil {
			handlers[name] = handler
		}
	}

	return &turnState{
		client:       c,
		request:      request,
		messages:     append([]chatMessage(nil), request.Messages...),
		handlers:     handlers,
		supportsBlob: llmshared.SupportsBlobForModel(prompt.ModelCapabilities, llmshared.SupportsImagesOnly),
	}, nil
}

// Step runs one blocking model request. LM Studio's streaming API is
// never used inside the tool loop (the streaming entry point either
// streams a tool-free request directly or wraps this blocking loop),
// so emit is unused.
func (t *turnState) Step(ctx context.Context, _ func(*ai.StreamChunk)) (llmshared.StepOutcome, error) {
	c := t.client
	req := t.request
	req.Messages = t.messages

	response, err := c.sendChat(ctx, req)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}

	usage := c.buildTokenCount(response.Usage)
	c.PublishTokenCount(usage)

	if len(response.Choices) == 0 {
		return llmshared.StepOutcome{}, ai.NonRetryable(errEmptyResponse)
	}

	assistant := response.Choices[0].Message
	if reasoning := strings.TrimSpace(decodeContentText(assistant.Reasoning)); reasoning != "" {
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
			return llmshared.StepOutcome{}, ai.NonRetryable(errEmptyResponse)
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
		return llmshared.StepOutcome{}, ai.NonRetryable(errToolCallNoHandler)
	}

	// Dedupe the requested calls the same way the shared loop will, so
	// the recorded assistant message carries exactly the calls that run.
	keptWire, converted, err := llmshared.DedupeChatToolCalls(assistant.ToolCalls, nil)
	if err != nil {
		return llmshared.StepOutcome{}, err
	}

	message := assistant.toChatMessage()
	message.ToolCalls = keptWire
	t.messages = append(t.messages, message)

	return llmshared.StepOutcome{ToolCalls: converted, Usage: usage}, nil
}

// AddToolResults appends one tool message per executed call (plus any
// extracted media payloads) so the next step sees the results.
func (t *turnState) AddToolResults(_ context.Context, results []llmshared.PreparedToolResult) error {
	for _, result := range results {
		encoded := llmshared.EncodeToolResult(result, t.supportsBlob)
		t.messages = append(t.messages, chatMessage{
			Role:       "tool",
			Content:    newMessageContentFromText(encoded.Text),
			ToolCallID: result.Call.ID,
		})
		for _, blob := range encoded.Blobs {
			t.messages = append(t.messages, buildImageUserMessage(blob))
		}
	}
	return nil
}
