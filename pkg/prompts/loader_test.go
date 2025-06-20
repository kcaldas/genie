package prompts

import (
	"context"
	"errors"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
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
	// Create a PromptLoader with a no-op publisher and default tool registry
	publisher := &events.NoOpPublisher{}
	toolRegistry := tools.NewDefaultRegistry()
	loader := NewPromptLoader(publisher, toolRegistry)

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

// TestPromptLoader_Caching tests that the PromptLoader caches loaded prompts
func TestPromptLoader_Caching(t *testing.T) {
	// Create a PromptLoader with a no-op publisher and default tool registry
	publisher := &events.NoOpPublisher{}
	toolRegistry := tools.NewDefaultRegistry()
	loader := NewPromptLoader(publisher, toolRegistry).(*DefaultLoader)

	// Initial cache should be empty
	assert.Equal(t, 0, loader.CacheSize(), "Cache should be empty initially")

	// Load the same prompt twice
	prompt1, err1 := loader.LoadPrompt("conversation")
	assert.NoError(t, err1)
	assert.Equal(t, 1, loader.CacheSize(), "Cache should contain one item after first load")
	
	prompt2, err2 := loader.LoadPrompt("conversation")
	assert.NoError(t, err2)
	assert.Equal(t, 1, loader.CacheSize(), "Cache size should remain 1 after second load")
	
	// Verify both prompts are identical (cached)
	assert.Equal(t, prompt1.Text, prompt2.Text, "Cached prompt should have same text")
	assert.Equal(t, len(prompt1.Functions), len(prompt2.Functions), "Cached prompt should have same number of functions")
	
	// Verify cache contains the prompt
	loader.cacheMutex.RLock()
	cachedPrompt, exists := loader.promptCache["conversation"]
	loader.cacheMutex.RUnlock()
	
	assert.True(t, exists, "Prompt should be in cache")
	assert.Equal(t, prompt1.Text, cachedPrompt.Text, "Cached prompt should match loaded prompt")
}

// TestPromptLoader_UsesCustomRegistry tests that the PromptLoader uses tools from a custom registry
func TestPromptLoader_UsesCustomRegistry(t *testing.T) {
	// Create a custom registry with specific tools
	customRegistry := tools.NewRegistry()
	
	// Create a mock tool
	mockTool := &MockTool{name: "customTool"}
	customRegistry.Register(mockTool)
	
	// Create a PromptLoader with the custom registry
	publisher := &events.NoOpPublisher{}
	loader := NewPromptLoader(publisher, customRegistry)
	
	// Load a prompt (this should enhance it with tools from custom registry)
	prompt, err := loader.LoadPrompt("conversation")
	assert.NoError(t, err)
	
	// Verify the prompt was enhanced with the custom tool
	assert.NotNil(t, prompt.Functions, "Prompt should have functions after loading")
	assert.NotNil(t, prompt.Handlers, "Prompt should have handlers after loading")
	assert.Len(t, prompt.Functions, 1, "Prompt should have exactly one tool function")
	assert.Len(t, prompt.Handlers, 1, "Prompt should have exactly one tool handler")
	
	// Check that it's our custom tool
	assert.Equal(t, "customTool", prompt.Functions[0].Name, "Should have custom tool")
	_, hasHandler := prompt.Handlers["customTool"]
	assert.True(t, hasHandler, "Should have handler for custom tool")
}

// MockTool for testing tool registry integration
type MockTool struct {
	name string
}

func (m *MockTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        m.name,
		Description: "Mock tool for testing",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"input": {
					Type:        ai.TypeString,
					Description: "Test input",
				},
			},
		},
	}
}

func (m *MockTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		return map[string]any{"result": "mock result"}, nil
	}
}

