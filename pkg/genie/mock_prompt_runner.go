package genie

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

type MockToolCall struct {
	ToolName string
	Returns  map[string]any
}

type MockResponse struct {
	Message   string
	Response  string
	ToolCalls []MockToolCall
}

type MockPromptRunner struct {
	responses       map[string]*MockResponse
	eventBus        events.EventBus
	capturedPrompts []*ai.Prompt
	capturedData    []map[string]string
}

func NewMockPromptRunner(eventBus events.EventBus) *MockPromptRunner {
	return &MockPromptRunner{
		responses: make(map[string]*MockResponse),
		eventBus:  eventBus,
	}
}

func (r *MockPromptRunner) ExpectMessage(message string) *MockResponseBuilder {
	mockResponse := &MockResponse{
		Message:   message,
		Response:  "", // Will be set by RespondWith
		ToolCalls: []MockToolCall{},
	}
	r.responses[message] = mockResponse

	return &MockResponseBuilder{
		runner:   r,
		response: mockResponse,
	}
}

func (r *MockPromptRunner) ExpectSimpleMessage(message, response string) {
	mockResponse := &MockResponse{
		Message:   message,
		Response:  response,
		ToolCalls: []MockToolCall{},
	}
	r.responses[message] = mockResponse
}

type MockResponseBuilder struct {
	runner   *MockPromptRunner
	response *MockResponse
}

func (b *MockResponseBuilder) MockTool(toolName string) *MockToolBuilder {
	return &MockToolBuilder{
		builder:  b,
		toolName: toolName,
	}
}

func (b *MockResponseBuilder) RespondWith(response string) *MockResponseBuilder {
	b.response.Response = response
	return b
}

type MockToolBuilder struct {
	builder  *MockResponseBuilder
	toolName string
}

func (t *MockToolBuilder) Returns(result map[string]any) *MockResponseBuilder {
	toolCall := MockToolCall{
		ToolName: t.toolName,
		Returns:  result,
	}
	t.builder.response.ToolCalls = append(t.builder.response.ToolCalls, toolCall)
	return t.builder
}

func (r *MockPromptRunner) RunPrompt(ctx context.Context, prompt *ai.Prompt, data map[string]string, eventBus events.EventBus) (string, error) {
	if prompt != nil {
		copyPrompt := *prompt
		r.capturedPrompts = append(r.capturedPrompts, &copyPrompt)
	}
	if data != nil {
		dataCopy := maps.Clone(data)
		r.capturedData = append(r.capturedData, dataCopy)
	}

	// Get the message from the prompt context
	message, exists := data["message"]
	if !exists {
		return "", fmt.Errorf("no message found in prompt context")
	}

	// Find the configured response for this message
	mockResponse, exists := r.responses[message]
	if !exists {
		return "", fmt.Errorf("no mock response configured for message: %q", message)
	}

	// Check if response was set (either via ExpectSimpleMessage or RespondWith)
	if mockResponse.Response == "" {
		return "", fmt.Errorf("no response configured for message %q - use RespondWith() or ExpectSimpleMessage()", message)
	}

	// Execute any mocked tool calls and publish events
	for _, toolCall := range mockResponse.ToolCalls {
		err := r.executeMockToolCall(ctx, data, toolCall)
		if err != nil {
			return "", fmt.Errorf("failed to execute mock tool call %q: %w", toolCall.ToolName, err)
		}
	}

	return mockResponse.Response, nil
}

func (r *MockPromptRunner) RunPromptStream(ctx context.Context, prompt *ai.Prompt, data map[string]string, eventBus events.EventBus) (string, error) {
	response, err := r.RunPrompt(ctx, prompt, data, eventBus)
	if err != nil {
		return "", err
	}

	if eventBus != nil {
		requestID := requestIDFromContext(ctx)
		if requestID == "" {
			return response, nil
		}
		chunkEvent := events.ChatChunkEvent{
			RequestID: requestID,
			Chunk: &ai.StreamChunk{
				Text: response,
			},
		}
		eventBus.Publish(chunkEvent.Topic(), chunkEvent)
	}

	return response, nil
}

func (r *MockPromptRunner) CountTokens(ctx context.Context, prompt *ai.Prompt, data map[string]string, eventBus events.EventBus) (*ai.TokenCount, error) {
	// Mock implementation - estimate tokens based on text length
	// Rough estimate: ~4 characters per token
	argsStr := strings.Join(slices.Collect(maps.Values(data)), " ")
	textLength := len(prompt.Text) + len(prompt.Instruction) + len(argsStr)
	estimatedTokens := int32(textLength / 4)
	if estimatedTokens < 1 {
		estimatedTokens = 1
	}

	return &ai.TokenCount{
		TotalTokens:  estimatedTokens,
		InputTokens:  estimatedTokens,
		OutputTokens: 0, // No output tokens for counting input
	}, nil
}

// CapturedPrompts returns copies of the prompts captured during RunPrompt invocations.
func (r *MockPromptRunner) CapturedPrompts() []*ai.Prompt {
	return append([]*ai.Prompt(nil), r.capturedPrompts...)
}

// CapturedData returns copies of the prompt data arguments captured during RunPrompt invocations.
func (r *MockPromptRunner) CapturedData() []map[string]string {
	copies := make([]map[string]string, 0, len(r.capturedData))
	for _, data := range r.capturedData {
		copies = append(copies, maps.Clone(data))
	}
	return copies
}

func (r *MockPromptRunner) executeMockToolCall(ctx context.Context, data interface{}, toolCall MockToolCall) error {
	// Extract session information for events
	executionID := "unknown"
	if ctx != nil {
		if id, ok := ctx.Value("executionID").(string); ok && id != "" {
			executionID = id
		}
	}

	// Publish tool execution event (simulates real tool behavior)
	if r.eventBus != nil {
		event := events.ToolExecutedEvent{
			ExecutionID: executionID,
			ToolName:    toolCall.ToolName,
			Parameters:  map[string]any{}, // Mock tools don't need real parameters
			Message:     "Executed (mocked)",
		}
		r.eventBus.Publish(event.Topic(), event)
	}

	// Note: For MockPromptRunner, we don't need to store tool results in prompt context
	// since we're bypassing the normal prompt processing logic entirely

	return nil
}

// GetStatus returns mock status for testing
func (r *MockPromptRunner) GetStatus() *ai.Status {
	return &ai.Status{
		Connected: true,
		Backend:   "mock-prompt-runner",
		Message:   "Mock prompt runner is available",
	}
}
