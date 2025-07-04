package persona

import (
	"context"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
)

// ChainFactory creates conversation chains - allows tests to inject custom chains
type ChainFactory interface {
	CreateChain(ctx context.Context) (*ai.Chain, error)
}

// PersonaAwareChainFactory creates chains based on persona names
type PersonaAwareChainFactory interface {
	CreateChain(ctx context.Context, personaName string) (*ai.Chain, error)
}

// PersonaManager manages different personas and their chains
type PersonaManager interface {
	// GetChain returns an appropriate chain
	GetChain(ctx context.Context) (*ai.Chain, error)
}

// DefaultPersonaManager is the default implementation of PersonaManager
type DefaultPersonaManager struct {
	chainFactory PersonaAwareChainFactory
	defaultPersona string
}

// NewDefaultPersonaManager creates a new DefaultPersonaManager with the given PersonaAwareChainFactory
func NewDefaultPersonaManager(chainFactory PersonaAwareChainFactory) PersonaManager {
	return &DefaultPersonaManager{
		chainFactory: chainFactory,
		defaultPersona: "engineer", // Default to engineer persona
	}
}

// GetChain creates and returns a chain using the default persona
func (m *DefaultPersonaManager) GetChain(ctx context.Context) (*ai.Chain, error) {
	chain, err := m.chainFactory.CreateChain(ctx, m.defaultPersona)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain for persona %s: %w", m.defaultPersona, err)
	}

	return chain, nil
}

