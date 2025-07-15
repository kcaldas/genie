// Package persona provides persona-based prompt management for Genie.
//
// This package implements a flexible persona system that discovers and loads
// persona prompts from multiple locations with priority ordering:
//
// 1. Project personas: $cwd/.genie/personas/{name}/prompt.yaml (highest priority)
// 2. User personas: ~/.genie/personas/{name}/prompt.yaml
// 3. Internal personas: embedded pkg/persona/personas/{name}/prompt.yaml (lowest priority)
//
// The PersonaManager provides a simple interface for prompt retrieval, defaulting
// to the "engineer" persona. The PersonaPromptFactory handles the discovery logic
// and creates prompts enhanced with tools and model defaults.
//
// Internal personas included:
// - engineer: Full-featured software engineering assistant
// - product_owner: Product management and planning focused assistant
// - persona_creator: Expert in designing custom personas for specific user objectives
package persona

import (
	"context"

	"github.com/kcaldas/genie/pkg/ai"
)

// PersonaAwarePromptFactory creates prompts based on persona names
type PersonaAwarePromptFactory interface {
	GetPrompt(ctx context.Context, personaName string) (*ai.Prompt, error)
}

// PersonaManager manages different personas and their prompts
type PersonaManager interface {
	GetPrompt(ctx context.Context) (*ai.Prompt, error)
}

// DefaultPersonaManager is the default implementation of PersonaManager
type DefaultPersonaManager struct {
	promptFactory  PersonaAwarePromptFactory
	defaultPersona string
}

// NewDefaultPersonaManager creates a new DefaultPersonaManager with the given PersonaAwarePromptFactory
func NewDefaultPersonaManager(promptFactory PersonaAwarePromptFactory) PersonaManager {
	return &DefaultPersonaManager{
		promptFactory:  promptFactory,
		defaultPersona: "engineer", // Default to engineer persona
	}
}

func (m *DefaultPersonaManager) GetPrompt(ctx context.Context) (*ai.Prompt, error) {
	// Get persona from context, fallback to default
	persona := m.defaultPersona
	if contextPersona, ok := ctx.Value("persona").(string); ok && contextPersona != "" {
		persona = contextPersona
	}

	prompt, err := m.promptFactory.GetPrompt(ctx, persona)
	if err != nil {
		return nil, err
	}

	return prompt, nil
}
