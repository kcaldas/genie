// Package persona provides persona-based chain management for Genie.
//
// This package implements a flexible persona system that discovers and loads
// persona prompts from multiple locations with priority ordering:
//
// 1. Project personas: $cwd/.genie/personas/{name}/prompt.yaml (highest priority)
// 2. User personas: ~/.genie/personas/{name}/prompt.yaml  
// 3. Internal personas: embedded pkg/persona/personas/{name}/prompt.yaml (lowest priority)
//
// The PersonaManager provides a simple interface for chain retrieval, defaulting
// to the "engineer" persona. The PersonaChainFactory handles the discovery logic
// and creates chains enhanced with tools and model defaults.
//
// Internal personas included:
// - engineer: Full-featured software engineering assistant
// - product_owner: Product management and planning focused assistant
// - persona_creator: Expert in designing custom personas for specific user objectives
package persona

import (
	"context"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
)

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

// GetChain creates and returns a chain using the persona from context or default
func (m *DefaultPersonaManager) GetChain(ctx context.Context) (*ai.Chain, error) {
	// Get persona from context, fallback to default
	persona := m.defaultPersona
	if contextPersona, ok := ctx.Value("persona").(string); ok && contextPersona != "" {
		persona = contextPersona
	}

	chain, err := m.chainFactory.CreateChain(ctx, persona)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain for persona %s: %w", persona, err)
	}

	return chain, nil
}

