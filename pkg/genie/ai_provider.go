package genie

import (
	"github.com/kcaldas/genie/pkg/ai"
)

// AIProvider encapsulates AI-related dependencies (LLM client and chain runner)
// This allows easy swapping between production and test implementations
type AIProvider interface {
	GetLLMClient() ai.Gen
	GetChainRunner() ChainRunner
}

// ProductionAIProvider provides real AI dependencies for production use
type ProductionAIProvider struct {
	llmClient       ai.Gen
	handlerRegistry ai.HandlerRegistry
	debug           bool
}

// NewProductionAIProvider creates a new production AI provider
func NewProductionAIProvider(llmClient ai.Gen, handlerRegistry ai.HandlerRegistry, debug bool) AIProvider {
	return &ProductionAIProvider{
		llmClient:       llmClient,
		handlerRegistry: handlerRegistry,
		debug:           debug,
	}
}

// GetLLMClient returns the production LLM client
func (p *ProductionAIProvider) GetLLMClient() ai.Gen {
	return p.llmClient
}

// GetChainRunner returns a new DefaultChainRunner for production use
func (p *ProductionAIProvider) GetChainRunner() ChainRunner {
	return NewDefaultChainRunner(p.llmClient, p.handlerRegistry, p.debug)
}

// TestAIProvider provides mock AI dependencies for testing
type TestAIProvider struct {
	mockLLM         ai.Gen
	mockChainRunner ChainRunner
}

// NewTestAIProvider creates a new test AI provider with mocks
func NewTestAIProvider(mockLLM ai.Gen, mockChainRunner ChainRunner) AIProvider {
	return &TestAIProvider{
		mockLLM:         mockLLM,
		mockChainRunner: mockChainRunner,
	}
}

// GetLLMClient returns the mock LLM client
func (t *TestAIProvider) GetLLMClient() ai.Gen {
	return t.mockLLM
}

// GetChainRunner returns the mock chain runner
func (t *TestAIProvider) GetChainRunner() ChainRunner {
	return t.mockChainRunner
}