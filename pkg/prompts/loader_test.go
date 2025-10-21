package prompts

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
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
	LoadCount int // Track how many times LoadPromptFromFS is called
}

func (mpl *MockLoader) LoadPromptFromFS(filesystem fs.FS, filePath string) (ai.Prompt, error) {
	mpl.LoadCount++
	// For testing, just return a basic prompt
	return ai.Prompt{
		Name:        "test-from-file",
		Instruction: "Test prompt loaded from file",
		Text:        "Test: {{.message}}",
	}, nil
}

// TestPromptLoader_EnhancesWithTools tests that the PromptLoader enhances prompts with tools
func TestPromptLoader_EnhancesWithTools(t *testing.T) {
	// Create a temporary test prompt file
	tempDir := t.TempDir()
	promptFile := filepath.Join(tempDir, "test-prompt.yaml")

	promptContent := `name: "test-conversation"
instruction: "You are a test AI assistant."
text: "User: {{.message}}"
required_tools:
  - "listFiles"
  - "findFiles" 
  - "readFile"
  - "writeFile"
  - "searchInFiles"
  - "bash"`

	err := os.WriteFile(promptFile, []byte(promptContent), 0644)
	assert.NoError(t, err)

	// Create a PromptLoader with a no-op publisher and default tool registry
	publisher := &events.NoOpPublisher{}
	eventBus := &events.NoOpEventBus{}
	todoManager := tools.NewTodoManager()
	toolRegistry := tools.NewDefaultRegistry(eventBus, todoManager)
	loader := NewPromptLoader(publisher, toolRegistry)

	// Load a prompt from file (this should enhance it with tools)
	prompt, err := loader.LoadPromptFromFS(os.DirFS(tempDir), "test-prompt.yaml")
	assert.NoError(t, err)

	// Verify the prompt was enhanced with tools
	assert.NotNil(t, prompt.Functions, "Prompt should have functions after loading")
	assert.NotNil(t, prompt.Handlers, "Prompt should have handlers after loading")
	assert.Greater(t, len(prompt.Functions), 0, "Prompt should have tool functions")
	assert.Greater(t, len(prompt.Handlers), 0, "Prompt should have tool handlers")

	// Check for specific tools
	toolNames := make([]string, len(prompt.Functions))
	for i, fn := range prompt.Functions {
		toolNames[i] = fn.Name
	}

	// Just verify we have some common tools
	assert.Contains(t, toolNames, "listFiles", "Should have listFiles tool")
	assert.Contains(t, toolNames, "readFile", "Should have readFile tool")
}

// TestPromptLoader_Caching tests that the PromptLoader caches loaded prompts
func TestPromptLoader_Caching(t *testing.T) {
	// Create a temporary test prompt file
	tempDir := t.TempDir()
	promptFile := filepath.Join(tempDir, "cache-test.yaml")

	promptContent := `name: "cache-test"
instruction: "Cache test prompt"
text: "Test: {{.message}}"
required_tools: []`

	err := os.WriteFile(promptFile, []byte(promptContent), 0644)
	assert.NoError(t, err)

	// Create a PromptLoader with a no-op publisher and default tool registry
	publisher := &events.NoOpPublisher{}
	eventBus := &events.NoOpEventBus{}
	todoManager := tools.NewTodoManager()
	toolRegistry := tools.NewDefaultRegistry(eventBus, todoManager)
	loader := NewPromptLoader(publisher, toolRegistry).(*DefaultLoader)

	// Initial cache should be empty
	assert.Equal(t, 0, loader.CacheSize(), "Cache should be empty initially")

	// Load the same prompt file twice to test caching
	filePrompt1, err := loader.LoadPromptFromFS(os.DirFS(tempDir), "cache-test.yaml")
	assert.NoError(t, err)
	assert.Equal(t, 1, loader.CacheSize(), "Cache should have one entry after first load")

	// Second load should come from cache
	filePrompt2, err := loader.LoadPromptFromFS(os.DirFS(tempDir), "cache-test.yaml")
	assert.NoError(t, err)
	assert.Equal(t, 1, loader.CacheSize(), "Cache should still have one entry after second load")

	assert.Equal(t, filePrompt1.Text, filePrompt2.Text, "Cached prompts should have same content")
	assert.Equal(t, filePrompt1.Name, filePrompt2.Name, "Cached prompts should have same name")
}

// TestPromptLoader_MissingRequiredTools tests error handling for missing tools
func TestPromptLoader_MissingRequiredTools(t *testing.T) {
	// Create a temporary test prompt file with required tools
	tempDir := t.TempDir()
	promptFile := filepath.Join(tempDir, "missing-tools-test.yaml")

	promptContent := `name: "missing-tools-test"
instruction: "Test prompt with missing tools"
text: "Test: {{.message}}"
required_tools:
  - "nonExistentTool"
  - "anotherMissingTool"`

	err := os.WriteFile(promptFile, []byte(promptContent), 0644)
	assert.NoError(t, err)

	// Create a custom registry with no tools
	customRegistry := tools.NewRegistry()

	// Create a PromptLoader with the empty registry
	publisher := &events.NoOpPublisher{}
	loader := NewPromptLoader(publisher, customRegistry)

	// Load prompt from file (which requires tools) - should fail
	_, err = loader.LoadPromptFromFS(os.DirFS(tempDir), "missing-tools-test.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required tools")
}

func TestPromptLoader_AppliesLLMProviderDefault(t *testing.T) {
	t.Setenv("GENIE_LLM_PROVIDER", "OpenAI")

	publisher := &events.NoOpPublisher{}
	eventBus := &events.NoOpEventBus{}
	todoManager := tools.NewTodoManager()
	toolRegistry := tools.NewDefaultRegistry(eventBus, todoManager)
	loader := NewPromptLoader(publisher, toolRegistry).(*DefaultLoader)

	prompt := &ai.Prompt{}
	loader.ApplyModelDefaults(prompt)

	assert.Equal(t, "openai", prompt.LLMProvider)
}

// TestPromptLoader_RequiredToolsOnly tests that only required tools are loaded
func TestPromptLoader_RequiredToolsOnly(t *testing.T) {
	// Create a temporary test prompt file
	tempDir := t.TempDir()
	promptFile := filepath.Join(tempDir, "required-tools-test.yaml")

	promptContent := `name: "required-tools-test"
instruction: "Test prompt with specific required tools"
text: "Test: {{.message}}"
required_tools:
  - "listFiles"
  - "findFiles" 
  - "readFile"
  - "writeFile"
  - "searchInFiles"
  - "bash"`

	err := os.WriteFile(promptFile, []byte(promptContent), 0644)
	assert.NoError(t, err)

	// Create a registry with all the tools that prompt needs
	customRegistry := tools.NewRegistry()

	// Create a no-op publisher for tests
	mockPublisher := &events.NoOpPublisher{}

	// Add the required tools for prompt
	requiredTools := []tools.Tool{
		tools.NewLsTool(mockPublisher),
		tools.NewFindTool(mockPublisher),
		tools.NewReadFileTool(mockPublisher),
		tools.NewGrepTool(mockPublisher),
		tools.NewGitStatusTool(),
		tools.NewWriteTool(nil, nil, false),
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

	// Load prompt from file
	prompt, err := loader.LoadPromptFromFS(os.DirFS(tempDir), "required-tools-test.yaml")
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
	// Create a temporary test prompt file without required_tools
	tempDir := t.TempDir()
	promptFile := filepath.Join(tempDir, "simple-test.yaml")

	promptContent := `name: "simple"
instruction: "Just a simple prompt"
text: "Simple prompt"`

	err := os.WriteFile(promptFile, []byte(promptContent), 0644)
	assert.NoError(t, err)

	// Create a PromptLoader
	publisher := &events.NoOpPublisher{}
	eventBus := &events.NoOpEventBus{}
	todoManager := tools.NewTodoManager()
	toolRegistry := tools.NewDefaultRegistry(eventBus, todoManager)
	loader := NewPromptLoader(publisher, toolRegistry)

	// Load prompt from file
	prompt, err := loader.LoadPromptFromFS(os.DirFS(tempDir), "simple-test.yaml")
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
