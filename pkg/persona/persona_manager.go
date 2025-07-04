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

// PersonaManager manages different personas and their chains
type PersonaManager interface {
	// GetChain returns an appropriate chain
	GetChain(ctx context.Context) (*ai.Chain, error)
}

// DefaultPersonaManager is the default implementation of PersonaManager
type DefaultPersonaManager struct {
	chainFactory ChainFactory
}

// NewDefaultPersonaManager creates a new DefaultPersonaManager with the given ChainFactory
func NewDefaultPersonaManager(chainFactory ChainFactory) PersonaManager {
	return &DefaultPersonaManager{
		chainFactory: chainFactory,
	}
}

// GetChain creates and returns a chain
func (m *DefaultPersonaManager) GetChain(ctx context.Context) (*ai.Chain, error) {
	chain, err := m.chainFactory.CreateChain(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain: %w", err)
	}

	return chain, nil
}

