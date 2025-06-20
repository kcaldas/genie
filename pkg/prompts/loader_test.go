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

// TestPromptLoader_Caching tests that the PromptLoader caches loaded prompts
func TestPromptLoader_Caching(t *testing.T) {
	// Create a PromptLoader with a no-op publisher
	publisher := &events.NoOpPublisher{}
	loader := NewPromptLoader(publisher).(*DefaultLoader)

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

