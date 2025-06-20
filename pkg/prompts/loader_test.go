package prompts

import (
	"context"
	"errors"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGen implements ai.Gen for testing
type MockGen struct {
	mock.Mock
}

func (m *MockGen) GenerateContent(ctx context.Context, prompt ai.Prompt, debug bool, args ...string) (string, error) {
	arguments := m.Called(ctx, prompt, debug, args)
	return arguments.String(0), arguments.Error(1)
}

func (m *MockGen) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	arguments := m.Called(ctx, prompt, debug, attrs)
	return arguments.String(0), arguments.Error(1)
}

// MockLoader implements Loader for testing
type MockLoader struct {
	MockPrompts map[string]ai.Prompt
	LoadCount   int // Track how many times LoadPrompt is called
}

func (mpl *MockLoader) LoadPrompt(promptName string) (ai.Prompt, error) {
	mpl.LoadCount++
	prompt, exists := mpl.MockPrompts[promptName]
	if !exists {
		return ai.Prompt{}, errors.New("prompt not found")
	}
	return prompt, nil
}

// TestPromptLoader_EnhancesWithTools tests that the PromptLoader enhances prompts with tools
func TestPromptLoader_EnhancesWithTools(t *testing.T) {
	// Create a PromptLoader with a no-op publisher
	publisher := &events.NoOpPublisher{}
	loader := NewPromptLoader(publisher)

	// Load a prompt (this should enhance it with tools)
	prompt, err := loader.LoadPrompt("conversation")
	assert.NoError(t, err)
	
	// Verify the prompt was enhanced with tools
	assert.NotNil(t, prompt.Functions, "Prompt should have functions after loading")
	assert.NotNil(t, prompt.Handlers, "Prompt should have handlers after loading")
	assert.Greater(t, len(prompt.Functions), 0, "Prompt should have tool functions")
	assert.Greater(t, len(prompt.Handlers), 0, "Prompt should have tool handlers")
	
	// Check for specific tools (let's see what tools are actually added)
	toolNames := make([]string, len(prompt.Functions))
	for i, fn := range prompt.Functions {
		toolNames[i] = fn.Name
	}
	
	// Just verify we have some common tools
	assert.Contains(t, toolNames, "listFiles", "Should have listFiles tool")
	assert.Contains(t, toolNames, "runBashCommand", "Should have runBashCommand tool")
}

