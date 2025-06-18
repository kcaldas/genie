package prompts

import (
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

func (m *MockGen) GenerateContent(prompt ai.Prompt, debug bool, args ...string) (string, error) {
	arguments := m.Called(prompt, debug, args)
	return arguments.String(0), arguments.Error(1)
}

func (m *MockGen) GenerateContentAttr(prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	arguments := m.Called(prompt, debug, attrs)
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

// TestPromptCaching_WithDependencyInjection tests caching using proper dependency injection
func TestPromptCaching_WithDependencyInjection(t *testing.T) {
	// Create test prompts
	promptName := "test-prompt"
	expectedPrompt := ai.Prompt{
		Text:        "Test prompt",
		Instruction: "Test instruction",
	}

	// Setup the mock loader with our test prompt
	mockPrompts := map[string]ai.Prompt{
		promptName: expectedPrompt,
	}
	mockLoader := &MockLoader{
		MockPrompts: mockPrompts,
		LoadCount:   0,
	}

	// Create a mock Gen that returns a fixed response
	mockGen := new(MockGen)
	mockGen.On("GenerateContentAttr", mock.Anything, true, mock.Anything).Return("Generated response", nil)

	// Create our executor with the mock loader and no-op publisher
	publisher := &events.NoOpPublisher{}
	executor := NewWithLoader(mockGen, publisher, mockLoader)

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