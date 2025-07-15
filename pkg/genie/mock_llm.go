package genie

import (
	"context"
	"fmt"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

// LLMInteraction captures a complete interaction with the LLM for debugging
type LLMInteraction struct {
	Timestamp     time.Time
	Prompt        ai.Prompt
	Args          []string
	Attrs         []ai.Attr
	RawResponse   string
	ProcessedResponse string
	Error         error
	Duration      time.Duration
	ToolsInPrompt []string
	Context       map[string]interface{} // Additional context for debugging
}

// MockLLMClient provides a comprehensive mock implementation of ai.Gen for testing
// It supports configurable responses, errors, delays, and detailed interaction capture
type MockLLMClient struct {
	responses         []string
	errors            []error
	currentIndex      int
	capturedPrompts   []ai.Prompt
	capturedArgs      [][]string
	capturedAttrs     [][]ai.Attr
	simulateDelay     time.Duration
	toolCallResponses map[string]string // Tool name -> response
	promptResponses   map[string]string // Prompt name -> response
	defaultResponse   string
	
	// Enhanced debugging and inspection
	interactionLog    []LLMInteraction
	debugMode         bool
	responseProcessor func(prompt ai.Prompt, rawResponse string) string
}

// NewMockLLMClient creates a new mock LLM client with sensible defaults
func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		toolCallResponses: make(map[string]string),
		promptResponses:   make(map[string]string),
		defaultResponse:   "Mock LLM response",
	}
}

// SetResponses configures the mock to return specific responses in sequence
// After all configured responses are used, it will return the default response
func (m *MockLLMClient) SetResponses(responses ...string) {
	m.responses = responses
	m.currentIndex = 0
}

// SetErrors configures the mock to return specific errors in sequence
// After all configured errors are used, it will return successful responses
func (m *MockLLMClient) SetErrors(errors ...error) {
	m.errors = errors
	m.currentIndex = 0
}

// SetDefaultResponse sets the fallback response when no specific responses are configured
func (m *MockLLMClient) SetDefaultResponse(response string) {
	m.defaultResponse = response
}

// SetToolResponse configures mock responses for specific tool calls
// When the LLM prompt contains a tool with this name, return the configured response
func (m *MockLLMClient) SetToolResponse(toolName, response string) {
	m.toolCallResponses[toolName] = response
}

// SetResponseForPrompt configures mock responses for specific prompt names
// This makes tests prompt-agnostic - they work regardless of prompt structure
func (m *MockLLMClient) SetResponseForPrompt(promptName, response string) {
	m.promptResponses[promptName] = response
}

// SetToolResponseWithJSON configures a mock response that includes JSON tool output
// This can be used to test JSON leakage issues
func (m *MockLLMClient) SetToolResponseWithJSON(toolName, jsonOutput, response string) {
	fullResponse := fmt.Sprintf("%s\n\n%s", jsonOutput, response)
	m.toolCallResponses[toolName] = fullResponse
}

// SetDelay simulates LLM processing time for testing async behavior
func (m *MockLLMClient) SetDelay(delay time.Duration) {
	m.simulateDelay = delay
}

// GenerateContent implements ai.Gen interface
func (m *MockLLMClient) GenerateContent(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (string, error) {
	startTime := time.Now()
	
	// Create interaction record
	interaction := LLMInteraction{
		Timestamp: startTime,
		Prompt:    prompt,
		Args:      append([]string{}, args...), // Copy args
		Context:   make(map[string]interface{}),
	}
	
	// Capture tools in prompt for debugging
	if prompt.Functions != nil {
		for _, fn := range prompt.Functions {
			interaction.ToolsInPrompt = append(interaction.ToolsInPrompt, fn.Name)
		}
	}
	
	// Simulate processing delay if configured
	if m.simulateDelay > 0 {
		select {
		case <-time.After(m.simulateDelay):
		case <-ctx.Done():
			interaction.Duration = time.Since(startTime)
			interaction.Error = ctx.Err()
			m.interactionLog = append(m.interactionLog, interaction)
			return "", ctx.Err()
		}
	}

	// Capture the call for backward compatibility
	m.capturedPrompts = append(m.capturedPrompts, prompt)
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	m.capturedArgs = append(m.capturedArgs, argsCopy)

	var rawResponse string
	var err error

	// Check for prompt-specific responses first (highest priority)
	if response, exists := m.promptResponses[prompt.Name]; exists {
		rawResponse = response
		interaction.Context["triggeredPrompt"] = prompt.Name
		interaction.Context["promptBasedResponse"] = true
		goto processResponse
	}

	// Check for tool calls in the prompt and return appropriate responses
	if prompt.Functions != nil {
		for _, fn := range prompt.Functions {
			if response, exists := m.toolCallResponses[fn.Name]; exists {
				rawResponse = response
				interaction.Context["triggeredTool"] = fn.Name
				goto processResponse
			}
		}
	}

	// Return configured error if available
	if m.currentIndex < len(m.errors) && m.errors[m.currentIndex] != nil {
		err = m.errors[m.currentIndex]
		m.currentIndex++
		interaction.Duration = time.Since(startTime)
		interaction.Error = err
		m.interactionLog = append(m.interactionLog, interaction)
		return "", err
	}

	// Return configured response if available
	if m.currentIndex < len(m.responses) {
		rawResponse = m.responses[m.currentIndex]
		m.currentIndex++
		interaction.Context["responseSource"] = "configured"
	} else {
		// Extract message from args for more realistic responses
		message := m.extractMessageFromArgs(args)
		if message != "" {
			rawResponse = fmt.Sprintf("%s to: %s", m.defaultResponse, message)
			interaction.Context["responseSource"] = "default_with_message"
		} else {
			rawResponse = m.defaultResponse
			interaction.Context["responseSource"] = "default"
		}
	}

processResponse:
	// Apply response processor if configured
	processedResponse := rawResponse
	if m.responseProcessor != nil {
		processedResponse = m.responseProcessor(prompt, rawResponse)
		interaction.Context["processed"] = true
	}
	
	// Complete interaction record
	interaction.Duration = time.Since(startTime)
	interaction.RawResponse = rawResponse
	interaction.ProcessedResponse = processedResponse
	m.interactionLog = append(m.interactionLog, interaction)
	
	// Debug logging if enabled
	if m.debugMode {
		fmt.Printf("[MockLLM] Interaction #%d:\n", len(m.interactionLog))
		fmt.Printf("  Tools: %v\n", interaction.ToolsInPrompt)
		fmt.Printf("  Raw Response: %q\n", rawResponse)
		fmt.Printf("  Processed: %q\n", processedResponse)
		fmt.Printf("  Duration: %v\n", interaction.Duration)
	}

	return processedResponse, nil
}

// GenerateContentAttr implements ai.Gen interface
func (m *MockLLMClient) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	// Capture the call
	m.capturedPrompts = append(m.capturedPrompts, prompt)
	attrsCopy := make([]ai.Attr, len(attrs))
	copy(attrsCopy, attrs)
	m.capturedAttrs = append(m.capturedAttrs, attrsCopy)

	// Convert attrs to args for consistent handling
	var args []string
	for _, attr := range attrs {
		args = append(args, attr.Key, attr.Value)
	}

	return m.GenerateContent(ctx, prompt, debug, args...)
}

// CountTokens returns mock token count for testing
func (m *MockLLMClient) CountTokens(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (*ai.TokenCount, error) {
	// Mock implementation - estimate tokens based on text length
	// Rough estimate: ~4 characters per token
	textLength := len(prompt.Text) + len(prompt.Instruction)
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

// GetStatus returns mock status - always connected for testing
func (m *MockLLMClient) GetStatus() *ai.Status {
	return &ai.Status{Connected: true, Backend: "mock", Message: "Mock LLM client - always available for testing"}
}

// extractMessageFromArgs extracts the message parameter from args
func (m *MockLLMClient) extractMessageFromArgs(args []string) string {
	for i := 0; i < len(args)-1; i += 2 {
		if args[i] == "message" {
			return args[i+1]
		}
	}
	return ""
}

// GetCapturedPrompts returns all prompts sent to the mock LLM
func (m *MockLLMClient) GetCapturedPrompts() []ai.Prompt {
	return m.capturedPrompts
}

// GetCapturedArgs returns all args sent to the mock LLM
func (m *MockLLMClient) GetCapturedArgs() [][]string {
	return m.capturedArgs
}

// GetCapturedAttrs returns all attrs sent to the mock LLM
func (m *MockLLMClient) GetCapturedAttrs() [][]ai.Attr {
	return m.capturedAttrs
}

// GetLastPrompt returns the most recent prompt sent to the mock LLM
func (m *MockLLMClient) GetLastPrompt() *ai.Prompt {
	if len(m.capturedPrompts) == 0 {
		return nil
	}
	return &m.capturedPrompts[len(m.capturedPrompts)-1]
}

// GetCallCount returns the number of times the LLM was called
func (m *MockLLMClient) GetCallCount() int {
	return len(m.capturedPrompts)
}

// Reset clears all captured data and resets state
func (m *MockLLMClient) Reset() {
	m.responses = nil
	m.errors = nil
	m.currentIndex = 0
	m.capturedPrompts = nil
	m.capturedArgs = nil
	m.capturedAttrs = nil
	m.toolCallResponses = make(map[string]string)
	m.simulateDelay = 0
	m.interactionLog = nil
	m.responseProcessor = nil
}

// Enhanced debugging and inspection methods

// EnableDebugMode turns on detailed logging of all LLM interactions
func (m *MockLLMClient) EnableDebugMode() {
	m.debugMode = true
}

// DisableDebugMode turns off debug logging
func (m *MockLLMClient) DisableDebugMode() {
	m.debugMode = false
}

// SetResponseProcessor allows custom processing of responses (useful for testing response parsing)
func (m *MockLLMClient) SetResponseProcessor(processor func(prompt ai.Prompt, rawResponse string) string) {
	m.responseProcessor = processor
}

// GetInteractionLog returns all recorded interactions for detailed inspection
func (m *MockLLMClient) GetInteractionLog() []LLMInteraction {
	return m.interactionLog
}

// GetLastInteraction returns the most recent interaction
func (m *MockLLMClient) GetLastInteraction() *LLMInteraction {
	if len(m.interactionLog) == 0 {
		return nil
	}
	return &m.interactionLog[len(m.interactionLog)-1]
}

// PrintInteractionSummary prints a human-readable summary of all interactions
func (m *MockLLMClient) PrintInteractionSummary() {
	fmt.Printf("\n=== LLM Interaction Summary ===\n")
	fmt.Printf("Total interactions: %d\n\n", len(m.interactionLog))
	
	for i, interaction := range m.interactionLog {
		fmt.Printf("Interaction #%d (%v):\n", i+1, interaction.Timestamp.Format("15:04:05.000"))
		fmt.Printf("  Tools in prompt: %v\n", interaction.ToolsInPrompt)
		fmt.Printf("  Args: %v\n", interaction.Args)
		if interaction.Error != nil {
			fmt.Printf("  Error: %v\n", interaction.Error)
		} else {
			fmt.Printf("  Raw response: %q\n", interaction.RawResponse)
			if interaction.ProcessedResponse != interaction.RawResponse {
				fmt.Printf("  Processed: %q\n", interaction.ProcessedResponse)
			}
		}
		fmt.Printf("  Duration: %v\n", interaction.Duration)
		if len(interaction.Context) > 0 {
			fmt.Printf("  Context: %v\n", interaction.Context)
		}
		fmt.Printf("\n")
	}
}

// CaptureRealLLMResponse allows recording actual LLM responses for replay in tests
func (m *MockLLMClient) CaptureRealLLMResponse(promptText string, args []string, response string) {
	// This could be used to capture real LLM interactions and replay them in tests
	m.SetResponses(response)
	
	// Also log as an interaction for debugging
	interaction := LLMInteraction{
		Timestamp:         time.Now(),
		Args:              args,
		RawResponse:       response,
		ProcessedResponse: response,
		Context: map[string]interface{}{
			"source": "real_llm_capture",
			"prompt_text": promptText,
		},
	}
	m.interactionLog = append(m.interactionLog, interaction)
}

// Convenience methods for common test scenarios

// SimulateTimeout configures the mock to simulate context timeout
func (m *MockLLMClient) SimulateTimeout() {
	m.SetDelay(10 * time.Second) // Long enough to trigger most timeouts
}

// SimulateRealisticDelay sets a realistic processing delay (100-500ms)
func (m *MockLLMClient) SimulateRealisticDelay() {
	m.SetDelay(200 * time.Millisecond)
}

// SimulateResponseWithMalformedJSON allows testing various response formatting issues
func (m *MockLLMClient) SimulateResponseWithMalformedJSON(toolName, jsonPart, textPart string) {
	malformedResponse := fmt.Sprintf("%s\n%s", jsonPart, textPart)
	m.SetToolResponse(toolName, malformedResponse)
}

// SimulateConversationalResponse sets up responses that look like normal chat
func (m *MockLLMClient) SimulateConversationalResponse(userMessage, assistantResponse string) {
	m.SetResponses(assistantResponse)
}

// Scenario Replay - Load and replay captured real-world interactions

// ReplayInteraction configures the mock to replay a specific captured interaction
func (m *MockLLMClient) ReplayInteraction(interaction ai.Interaction) {
	// Set up the mock to return the captured response
	if interaction.Error != nil {
		// If the original interaction had an error, simulate it
		m.SetErrors(fmt.Errorf("%s", interaction.Error.Message))
	} else {
		m.SetResponses(interaction.Response)
	}
	
	// If there were specific tools involved, set up tool responses
	if len(interaction.Tools) > 0 {
		for _, toolName := range interaction.Tools {
			m.SetToolResponse(toolName, interaction.Response)
		}
	}
	
	// Simulate the original duration if it was significant
	if interaction.Duration > 100*time.Millisecond {
		m.SetDelay(interaction.Duration)
	}
	
	// Store metadata for inspection
	m.interactionLog = append(m.interactionLog, LLMInteraction{
		Timestamp:         interaction.Timestamp,
		RawResponse:       interaction.Response,
		ProcessedResponse: interaction.Response,
		Duration:          interaction.Duration,
		ToolsInPrompt:     interaction.Tools,
		Context: map[string]interface{}{
			"replay_source":     "captured_interaction",
			"original_id":       interaction.ID,
			"original_provider": interaction.LLMProvider,
			"original_debug":    interaction.Debug,
		},
	})
}

// ReplayScenarioFromFile loads a captured scenario file and sets up replay
func (m *MockLLMClient) ReplayScenarioFromFile(filename string) error {
	interactions, err := ai.LoadInteractionsFromFile(filename)
	if err != nil {
		return fmt.Errorf("failed to load scenario from %s: %w", filename, err)
	}
	
	if len(interactions) == 0 {
		return fmt.Errorf("no interactions found in file %s", filename)
	}
	
	// If multiple interactions, set up responses in sequence
	responses := make([]string, 0, len(interactions))
	for _, interaction := range interactions {
		if interaction.Error == nil {
			responses = append(responses, interaction.Response)
		}
	}
	
	m.SetResponses(responses...)
	
	// Log all interactions for inspection
	for _, interaction := range interactions {
		m.interactionLog = append(m.interactionLog, LLMInteraction{
			Timestamp:         interaction.Timestamp,
			RawResponse:       interaction.Response,
			ProcessedResponse: interaction.Response,
			Duration:          interaction.Duration,
			ToolsInPrompt:     interaction.Tools,
			Context: map[string]interface{}{
				"replay_source":     "scenario_file",
				"scenario_file":     filename,
				"original_id":       interaction.ID,
				"original_provider": interaction.LLMProvider,
			},
		})
	}
	
	if m.debugMode {
		fmt.Printf("[MockLLM] Loaded scenario from %s with %d interactions\n", filename, len(interactions))
	}
	
	return nil
}

// ReplaySpecificInteraction replays a specific interaction by ID from a scenario file
func (m *MockLLMClient) ReplaySpecificInteraction(filename, interactionID string) error {
	interactions, err := ai.LoadInteractionsFromFile(filename)
	if err != nil {
		return fmt.Errorf("failed to load scenario from %s: %w", filename, err)
	}
	
	for _, interaction := range interactions {
		if interaction.ID == interactionID {
			m.ReplayInteraction(interaction)
			return nil
		}
	}
	
	return fmt.Errorf("interaction %s not found in file %s", interactionID, filename)
}

// GetReplayMetadata returns metadata about the currently replayed scenario
func (m *MockLLMClient) GetReplayMetadata() map[string]interface{} {
	if len(m.interactionLog) == 0 {
		return nil
	}
	
	lastInteraction := m.interactionLog[len(m.interactionLog)-1]
	if replayData, exists := lastInteraction.Context["replay_source"]; exists {
		return map[string]interface{}{
			"is_replay":     true,
			"replay_source": replayData,
			"context":       lastInteraction.Context,
		}
	}
	
	return map[string]interface{}{
		"is_replay": false,
	}
}

// Scenario creation helpers

// CreateScenarioFromCurrentSession creates a scenario file from the current mock session
func (m *MockLLMClient) CreateScenarioFromCurrentSession(filename string) error {
	if len(m.interactionLog) == 0 {
		return fmt.Errorf("no interactions to save")
	}
	
	// Convert mock interactions to ai.Interaction format
	interactions := make([]ai.Interaction, 0, len(m.interactionLog))
	
	for i, mockInteraction := range m.interactionLog {
		interaction := ai.Interaction{
			ID:          fmt.Sprintf("mock_interaction_%d", i+1),
			Timestamp:   mockInteraction.Timestamp,
			Response:    mockInteraction.RawResponse,
			Duration:    mockInteraction.Duration,
			Tools:       mockInteraction.ToolsInPrompt,
			LLMProvider: "mock",
			Context:     mockInteraction.Context,
		}
		
		if mockInteraction.Error != nil {
			interaction.Error = &ai.CapturedError{
				Message: mockInteraction.Error.Error(),
				Type:    fmt.Sprintf("%T", mockInteraction.Error),
			}
		}
		
		interactions = append(interactions, interaction)
	}
	
	return ai.SaveInteractionsToFile(interactions, filename)
}