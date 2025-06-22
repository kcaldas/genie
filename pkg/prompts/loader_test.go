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
	eventBus := &events.NoOpEventBus{}
	toolRegistry := tools.NewDefaultRegistry(eventBus)
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
	eventBus := &events.NoOpEventBus{}
	toolRegistry := tools.NewDefaultRegistry(eventBus)
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

// TestPromptLoader_MissingRequiredTools tests error handling for missing tools
func TestPromptLoader_MissingRequiredTools(t *testing.T) {
	// Create a custom registry with no tools
	customRegistry := tools.NewRegistry()
	
	// Create a PromptLoader with the empty registry
	publisher := &events.NoOpPublisher{}
	loader := NewPromptLoader(publisher, customRegistry)
	
	// Load conversation prompt (which requires tools) - should fail
	_, err := loader.LoadPrompt("conversation")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required tools")
}

// TestPromptLoader_RequiredToolsOnly tests that only required tools are loaded
func TestPromptLoader_RequiredToolsOnly(t *testing.T) {
	// Create a registry with all the tools that conversation prompt needs
	customRegistry := tools.NewRegistry()
	
	// Add the required tools for conversation prompt
	requiredTools := []tools.Tool{
		tools.NewLsTool(),
		tools.NewFindTool(),
		tools.NewCatTool(),
		tools.NewGrepTool(),
		tools.NewGitStatusTool(),
		tools.NewBashTool(nil, nil, false),
	}
	
	for _, tool := range requiredTools {
		customRegistry.Register(tool)
	}
	
	// Add an extra tool that shouldn't be included
	extraTool := &MockTool{name: "extraTool"}
	customRegistry.Register(extraTool)
	
	// Create a PromptLoader with the registry
	publisher := &events.NoOpPublisher{}
	loader := NewPromptLoader(publisher, customRegistry)
	
	// Load conversation prompt
	prompt, err := loader.LoadPrompt("conversation")
	assert.NoError(t, err)
	
	// Verify prompt has exactly the required tools (not the extra one)
	assert.Len(t, prompt.Functions, 6, "Should have exactly 6 required tools")
	assert.Len(t, prompt.Handlers, 6, "Should have exactly 6 tool handlers")
	
	// Verify extra tool is not included
	toolNames := make([]string, len(prompt.Functions))
	for i, fn := range prompt.Functions {
		toolNames[i] = fn.Name
	}
	assert.NotContains(t, toolNames, "extraTool", "Should not include extra tool")
}

// TestPromptLoader_NoRequiredTools tests that prompts without required_tools get no tools
func TestPromptLoader_NoRequiredTools(t *testing.T) {
	// Create a mock loader that returns a prompt without required_tools
	mockLoader := &MockLoader{
		MockPrompts: map[string]ai.Prompt{
			"simple": {
				Name:        "simple",
				Text:        "Simple prompt",
				Instruction: "Just a simple prompt",
				// No RequiredTools field - should get no tools
			},
		},
	}
	
	// Use the mock loader directly to test addTools behavior
	publisher := &events.NoOpPublisher{}
	eventBus := &events.NoOpEventBus{}
	toolRegistry := tools.NewDefaultRegistry(eventBus)
	loader := &DefaultLoader{
		Publisher:    publisher,
		ToolRegistry: toolRegistry,
		promptCache:  make(map[string]ai.Prompt),
	}
	
	// Get prompt from mock and test addTools
	prompt, err := mockLoader.LoadPrompt("simple")
	assert.NoError(t, err)
	
	// Apply addTools
	err = loader.addTools(&prompt)
	assert.NoError(t, err)
	
	// Verify no tools were added
	assert.Nil(t, prompt.Functions, "Prompt without required_tools should have no functions")
	assert.Nil(t, prompt.Handlers, "Prompt without required_tools should have no handlers")
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

func (m *MockTool) FormatOutput(result map[string]interface{}) string {
	return "Mock formatted output"
}

