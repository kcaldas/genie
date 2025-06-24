package genie

import (
	"context"
	"sync"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/llm/genai"
)

// LazyAIProvider implements AIProvider with lazy initialization of the LLM client
// This allows fast startup without network calls, only initializing when actually needed
type LazyAIProvider struct {
	// Dependencies for creating the real AI provider when needed
	handlerRegistry ai.HandlerRegistry
	debug           bool
	
	// Lazy initialization state
	mu              sync.Mutex
	initialized     bool
	llmClient       ai.Gen
	initError       error
}

// NewLazyAIProvider creates a new lazy AI provider
func NewLazyAIProvider(handlerRegistry ai.HandlerRegistry, debug bool) AIProvider {
	return &LazyAIProvider{
		handlerRegistry: handlerRegistry,
		debug:           debug,
	}
}

// GetLLMClient returns the LLM client, initializing it if necessary
func (p *LazyAIProvider) GetLLMClient() ai.Gen {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// If already initialized (successfully or with error), return cached result
	if p.initialized {
		if p.initError != nil {
			// Return a client that will always error - this maintains interface compatibility
			return &ErrorLLMClient{err: p.initError}
		}
		return p.llmClient
	}
	
	// First time - attempt initialization
	p.initialized = true
	
	// Try to create the real LLM client (this may make network calls)
	client, err := genai.NewClientWithError()
	if err != nil {
		p.initError = err
		return &ErrorLLMClient{err: err}
	}
	
	p.llmClient = client
	return p.llmClient
}

// GetChainRunner returns a chain runner (may initialize LLM client if needed)
func (p *LazyAIProvider) GetChainRunner() ChainRunner {
	// This will trigger LLM client initialization if needed
	llmClient := p.GetLLMClient()
	return NewDefaultChainRunner(llmClient, p.handlerRegistry, p.debug)
}

// IsInitialized returns whether the LLM client has been initialized
func (p *LazyAIProvider) IsInitialized() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initialized
}

// GetInitializationError returns any error that occurred during initialization
func (p *LazyAIProvider) GetInitializationError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initError
}

// ErrorLLMClient is a placeholder client that always returns an error
// This maintains interface compatibility when initialization fails
type ErrorLLMClient struct {
	err error
}

// GenerateContentAttr always returns the initialization error
func (e *ErrorLLMClient) GenerateContentAttr(ctx context.Context, prompt ai.Prompt, debug bool, attrs []ai.Attr) (string, error) {
	return "", e.err
}

// GenerateContent always returns the initialization error  
func (e *ErrorLLMClient) GenerateContent(ctx context.Context, p ai.Prompt, debug bool, args ...string) (string, error) {
	return "", e.err
}

// GetStatus returns the error information
func (e *ErrorLLMClient) GetStatus() (connected bool, backend string, message string) {
	return false, "error", e.err.Error()
}