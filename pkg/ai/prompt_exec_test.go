package ai

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Use SharedMockGen from testutil_test.go instead of duplicate MockGen

// MockPromptLoader implements PromptLoader for testing
type MockPromptLoader struct {
	MockPrompts map[string]Prompt
	LoadCount   int // Track how many times LoadPrompt is called
}

func (mpl *MockPromptLoader) LoadPrompt(promptName string) (Prompt, error) {
	mpl.LoadCount++
	prompt, exists := mpl.MockPrompts[promptName]
	if !exists {
		return Prompt{}, errors.New("prompt not found")
	}
	return prompt, nil
}

// TestPromptCaching_WithDependencyInjection tests caching using proper dependency injection
func TestPromptCaching_WithDependencyInjection(t *testing.T) {
	// Create test prompts
	promptName := "test-prompt"
	expectedPrompt := Prompt{
		Text:        "Test prompt",
		Instruction: "Test instruction",
	}

	// Setup the mock loader with our test prompt
	mockPrompts := map[string]Prompt{
		promptName: expectedPrompt,
	}
	mockLoader := &MockPromptLoader{
		MockPrompts: mockPrompts,
		LoadCount:   0,
	}

	// Create a mock Gen that returns a fixed response
	mockGen := new(SharedMockGen)
	mockGen.On("GenerateContentAttr", mock.Anything, true, mock.Anything).Return("Generated response", nil)

	// Create our executor with the mock loader
	executor := NewPromptExecutorWithLoader(mockGen, mockLoader)

	// Initial cache should be empty
	assert.Equal(t, 0, executor.CacheSize(), "Cache should be empty initially")

	// First call should load from loader and cache
	result, err := executor.Execute(promptName, true)
	assert.NoError(t, err)
	assert.Equal(t, "Generated response", result)
	assert.Equal(t, 1, executor.CacheSize(), "Cache should contain one item after first load")
	assert.Equal(t, 1, mockLoader.LoadCount, "Loader should have been called once")

	// Second call should use cached prompt and not call the loader again
	result, err = executor.Execute(promptName, true)
	assert.NoError(t, err)
	assert.Equal(t, "Generated response", result)
	assert.Equal(t, 1, executor.CacheSize(), "Cache size should remain 1 after second call")
	assert.Equal(t, 1, mockLoader.LoadCount, "Loader should still have been called only once")

	// Test with a different prompt
	nonexistentResult, err := executor.Execute("nonexistent", true)
	assert.Error(t, err)
	assert.Equal(t, "", nonexistentResult)
	assert.Equal(t, 1, executor.CacheSize(), "Cache size should still be 1 after failed load")
	assert.Equal(t, 2, mockLoader.LoadCount, "Loader should have been called a second time")
}
