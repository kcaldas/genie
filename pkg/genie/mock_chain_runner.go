package genie

import (
	"context"
	"fmt"

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

type MockChainRunner struct {
	responses map[string]*MockResponse
	eventBus  events.EventBus
}

func NewMockChainRunner(eventBus events.EventBus) *MockChainRunner {
	return &MockChainRunner{
		responses: make(map[string]*MockResponse),
		eventBus:  eventBus,
	}
}

func (r *MockChainRunner) ExpectMessage(message string) *MockResponseBuilder {
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

func (r *MockChainRunner) ExpectSimpleMessage(message, response string) {
	mockResponse := &MockResponse{
		Message:   message,
		Response:  response,
		ToolCalls: []MockToolCall{},
	}
	r.responses[message] = mockResponse
}

type MockResponseBuilder struct {
	runner   *MockChainRunner
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

func (r *MockChainRunner) RunChain(ctx context.Context, chain *ai.Chain, chainCtx *ai.ChainContext, eventBus events.EventBus) error {
	// Get the message from the chain context
	message, exists := chainCtx.Data["message"]
	if !exists {
		return fmt.Errorf("no message found in chain context")
	}
	
	// Find the configured response for this message
	mockResponse, exists := r.responses[message]
	if !exists {
		return fmt.Errorf("no mock response configured for message: %q", message)
	}
	
	// Check if response was set (either via ExpectSimpleMessage or RespondWith)
	if mockResponse.Response == "" {
		return fmt.Errorf("no response configured for message %q - use RespondWith() or ExpectSimpleMessage()", message)
	}
	
	// Execute any mocked tool calls and publish events
	for _, toolCall := range mockResponse.ToolCalls {
		err := r.executeMockToolCall(ctx, chainCtx, toolCall)
		if err != nil {
			return fmt.Errorf("failed to execute mock tool call %q: %w", toolCall.ToolName, err)
		}
	}
	
	// Set the final response in the chain context
	chainCtx.Data["response"] = mockResponse.Response
	
	// Publish chat.response event that CLI/TUI are waiting for
	if r.eventBus != nil {
		sessionID := "unknown"
		if ctx != nil {
			if id, ok := ctx.Value("sessionID").(string); ok && id != "" {
				sessionID = id
			}
		}
		
		event := events.ChatResponseEvent{
			SessionID: sessionID,
			Message:   message,
			Response:  mockResponse.Response,
			Error:     nil,
		}
		r.eventBus.Publish(event.Topic(), event)
	}
	
	return nil
}

func (r *MockChainRunner) executeMockToolCall(ctx context.Context, chainCtx *ai.ChainContext, toolCall MockToolCall) error {
	// Extract session information for events
	sessionID := "unknown"
	executionID := "unknown"
	if ctx != nil {
		if id, ok := ctx.Value("sessionID").(string); ok && id != "" {
			sessionID = id
		}
		if id, ok := ctx.Value("executionID").(string); ok && id != "" {
			executionID = id
		}
	}
	
	// Publish tool execution event (simulates real tool behavior)
	if r.eventBus != nil {
		event := events.ToolExecutedEvent{
			ExecutionID: executionID,
			SessionID:   sessionID,
			ToolName:    toolCall.ToolName,
			Parameters:  map[string]any{}, // Mock tools don't need real parameters
			Message:     "Executed (mocked)",
		}
		r.eventBus.Publish(event.Topic(), event)
	}
	
	// Note: For MockChainRunner, we don't need to store tool results in chain context
	// since we're bypassing the normal chain logic entirely
	
	return nil
}