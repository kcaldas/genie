package prompts

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
	"gopkg.in/yaml.v2"
)

//go:embed prompts/*
var promptsFS embed.FS

// Loader defines how prompts are loaded
type Loader interface {
	LoadPrompt(promptName string) (ai.Prompt, error)
}

// Executor defines the behavior of a service that generates a response for a given prompt and highlight in context
type Executor interface {
	Execute(promptName string, debug bool, promptData ...ai.Attr) (string, error)
	ExecuteWithSchema(promptName string, schema *ai.Schema, promptData ...ai.Attr) (string, error)
	CacheSize() int // For testing purposes
}

// Loader loads prompts from embedded file system
type DefaultLoader struct{}

// LoadPrompt loads a prompt from the embedded file system
func (l *DefaultLoader) LoadPrompt(promptName string) (ai.Prompt, error) {
	data, err := promptsFS.ReadFile("prompts/" + promptName + ".yaml")
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error reading embedded prompt file: %w", err)
	}

	var prompt ai.Prompt
	err = yaml.Unmarshal(data, &prompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error unmarshaling prompt: %w", err)
	}

	return prompt, nil
}

// FileLoader is the file-based implementation of Loader
type FileLoader struct {
	PromptsPath string
}

// LoadPrompt loads a prompt from disk
func (l *FileLoader) LoadPrompt(promptName string) (ai.Prompt, error) {
	data, err := os.ReadFile(filepath.Join(l.PromptsPath, promptName+".yaml"))
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error reading prompt file: %w", err)
	}

	var prompt ai.Prompt
	err = yaml.Unmarshal(data, &prompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("error unmarshaling prompt: %w", err)
	}

	return prompt, nil
}

// Executor represents a service that generates a response for a given prompt and highlight in context
type DefaultExecutor struct {
	Gen         ai.Gen
	Loader      Loader
	promptCache map[string]ai.Prompt // Cache to store loaded prompts
	cacheMutex  sync.RWMutex         // Mutex to protect the cache map
}

// CacheSize returns the number of prompts in the cache (for testing purposes)
func (s *DefaultExecutor) CacheSize() int {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return len(s.promptCache)
}

// NewExecutor creates a new DefaultExecutor with embedded prompts
func NewExecutor(gen ai.Gen) Executor {
	loader := &DefaultLoader{}

	return &DefaultExecutor{
		Gen:         gen,
		Loader:      loader,
		promptCache: make(map[string]ai.Prompt),
	}
}

// NewFileBasedExecutor creates a new DefaultExecutor with a file-based loader
func NewFileBasedExecutor(gen ai.Gen, promptsPath string) Executor {
	loader := &FileLoader{
		PromptsPath: promptsPath,
	}

	return &DefaultExecutor{
		Gen:         gen,
		Loader:      loader,
		promptCache: make(map[string]ai.Prompt),
	}
}

// NewWithLoader creates a DefaultExecutor with a custom loader (useful for testing)
func NewWithLoader(gen ai.Gen, loader Loader) Executor {
	return &DefaultExecutor{
		Gen:         gen,
		Loader:      loader,
		promptCache: make(map[string]ai.Prompt),
	}
}

// getPrompt loads a prompt from cache or uses the loader if not cached
func (s *DefaultExecutor) getPrompt(promptName string) (ai.Prompt, error) {
	// First, check if the prompt is in the cache
	s.cacheMutex.RLock()
	prompt, exists := s.promptCache[promptName]
	s.cacheMutex.RUnlock()

	if exists {
		return prompt, nil
	}

	// Not in cache, use the prompt loader
	newPrompt, err := s.Loader.LoadPrompt(promptName)
	if err != nil {
		return ai.Prompt{}, err
	}

	// Store in cache
	s.cacheMutex.Lock()
	s.promptCache[promptName] = newPrompt
	s.cacheMutex.Unlock()

	return newPrompt, nil
}

// Execute generates a response for the given prompt and highlight in context
func (s *DefaultExecutor) Execute(promptName string, debug bool, promptData ...ai.Attr) (string, error) {
	prompt, err := s.getPrompt(promptName)
	if err != nil {
		return "", err
	}

	result, err := s.Gen.GenerateContentAttr(prompt, debug, promptData)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}
	result = ai.RemoveSurroundingMarkdown(result)

	return result, nil
}

func (s *DefaultExecutor) ExecuteWithSchema(promptName string, schema *ai.Schema, promptData ...ai.Attr) (string, error) {
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
	result = ai.RemoveSurroundingMarkdown(result)

	return result, nil
}