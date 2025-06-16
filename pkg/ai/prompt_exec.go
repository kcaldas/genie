package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v2"
)

// PromptLoader defines how prompts are loaded
type PromptLoader interface {
	LoadPrompt(promptName string) (Prompt, error)
}

// PromptExecutor defines the behavior of a service that generates a response for a given prompt and highlight in context
type PromptExecutor interface {
	Execute(promptName string, debug bool, promptData ...Attr) (string, error)
	ExecuteWithSchema(promptName string, schema *Schema, promptData ...Attr) (string, error)
	CacheSize() int // For testing purposes
}

// DefaultPromptLoader is the default implementation of PromptLoader
type DefaultPromptLoader struct {
	PromptsPath string
}

// LoadPrompt loads a prompt from disk
func (l *DefaultPromptLoader) LoadPrompt(promptName string) (Prompt, error) {
	data, err := os.ReadFile(filepath.Join(l.PromptsPath, promptName+".yml"))
	if err != nil {
		return Prompt{}, fmt.Errorf("error reading prompt file: %w", err)
	}

	var prompt Prompt
	err = yaml.Unmarshal(data, &prompt)
	if err != nil {
		return Prompt{}, fmt.Errorf("error unmarshaling prompt: %w", err)
	}

	return prompt, nil
}

// DefaultPromptExecutor represents a service that generates a response for a given prompt and highlight in context
type DefaultPromptExecutor struct {
	Gen          Gen
	PromptLoader PromptLoader
	promptCache  map[string]Prompt // Cache to store loaded prompts
	cacheMutex   sync.RWMutex      // Mutex to protect the cache map
}

// CacheSize returns the number of prompts in the cache (for testing purposes)
func (s *DefaultPromptExecutor) CacheSize() int {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return len(s.promptCache)
}

// NewDefaultPromptExecutor creates a new DefaultPromptExecutor with a default prompt loader
func NewDefaultPromptExecutor(gen Gen, promptsPath string) PromptExecutor {
	loader := &DefaultPromptLoader{
		PromptsPath: promptsPath,
	}

	return &DefaultPromptExecutor{
		Gen:          gen,
		PromptLoader: loader,
		promptCache:  make(map[string]Prompt),
	}
}

// NewPromptExecutorWithLoader creates a DefaultPromptExecutor with a custom loader (useful for testing)
func NewPromptExecutorWithLoader(gen Gen, loader PromptLoader) PromptExecutor {
	return &DefaultPromptExecutor{
		Gen:          gen,
		PromptLoader: loader,
		promptCache:  make(map[string]Prompt),
	}
}

// getPrompt loads a prompt from cache or uses the loader if not cached
func (s *DefaultPromptExecutor) getPrompt(promptName string) (Prompt, error) {
	// First, check if the prompt is in the cache
	s.cacheMutex.RLock()
	prompt, exists := s.promptCache[promptName]
	s.cacheMutex.RUnlock()

	if exists {
		return prompt, nil
	}

	// Not in cache, use the prompt loader
	newPrompt, err := s.PromptLoader.LoadPrompt(promptName)
	if err != nil {
		return Prompt{}, err
	}

	// Store in cache
	s.cacheMutex.Lock()
	s.promptCache[promptName] = newPrompt
	s.cacheMutex.Unlock()

	return newPrompt, nil
}

// Execute generates a response for the given prompt and highlight in context
func (s *DefaultPromptExecutor) Execute(promptName string, debug bool, promptData ...Attr) (string, error) {
	prompt, err := s.getPrompt(promptName)
	if err != nil {
		return "", err
	}

	result, err := s.Gen.GenerateContentAttr(prompt, debug, promptData)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}
	result = removeSurroundingMarkdown(result)

	return result, nil
}

func (s *DefaultPromptExecutor) ExecuteWithSchema(promptName string, schema *Schema, promptData ...Attr) (string, error) {
	prompt, err := s.getPrompt(promptName)
	if err != nil {
		return "", err
	}

	// Make a copy of the prompt and add the schema
	promptCopy := prompt
	promptCopy.ResponseSchema = schema

	result, err := s.Gen.GenerateContentAttr(promptCopy, true, promptData)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}
	result = removeSurroundingMarkdown(result)

	return result, nil
}
