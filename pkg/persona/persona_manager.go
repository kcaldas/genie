package persona

import (
	"context"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/genie"
)

// PersonaManager manages different personas and their chains
type PersonaManager interface {
	// GetChain returns an appropriate chain
	GetChain(ctx context.Context) (*ai.Chain, error)
}

// DefaultPersonaManager is the default implementation of PersonaManager
type DefaultPersonaManager struct {
	chainFactory genie.ChainFactory
}

// NewDefaultPersonaManager creates a new DefaultPersonaManager with the given ChainFactory
func NewDefaultPersonaManager(chainFactory genie.ChainFactory) PersonaManager {
	return &DefaultPersonaManager{
		chainFactory: chainFactory,
	}
}

// GetChain creates and returns a chain
func (m *DefaultPersonaManager) GetChain(ctx context.Context) (*ai.Chain, error) {
	chain, err := m.chainFactory.CreateChain()
	if err != nil {
		return nil, fmt.Errorf("failed to create chain: %w", err)
	}

	return chain, nil
}

