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
	"fmt"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"gopkg.in/yaml.v2"
)

// PersonaAwarePromptFactory creates prompts based on persona names
type PersonaAwarePromptFactory interface {
	GetPrompt(ctx context.Context, personaName string) (*ai.Prompt, error)
	// GetPromptFromBytes loads a prompt directly from YAML bytes and enhances it with skills.
	// This is used for in-memory persona configuration, bypassing file-based discovery.
	GetPromptFromBytes(ctx context.Context, yamlContent []byte) (*ai.Prompt, error)
}

// PersonaSource indicates where a persona is loaded from
type PersonaSource string

const (
	// PersonaSourceInternal indicates the persona is built into Genie
	PersonaSourceInternal PersonaSource = "internal"
	// PersonaSourceProject indicates the persona is from the project's .genie/personas directory
	PersonaSourceProject PersonaSource = "project"
	// PersonaSourceUser indicates the persona is from the user's ~/.genie/personas directory
	PersonaSourceUser PersonaSource = "user"
)

// Persona represents a discovered persona with its metadata
type Persona struct {
	ID     string        // The folder name
	Name   string        // The name from prompt.yaml
	Source PersonaSource // Where the persona was found
}

// GetID returns the persona's ID (folder name)
func (p Persona) GetID() string {
	return p.ID
}

// GetName returns the persona's name from prompt.yaml
func (p Persona) GetName() string {
	return p.Name
}

// GetSource returns where the persona was found
func (p Persona) GetSource() string {
	return string(p.Source)
}

// PersonaManager manages different personas and their prompts
type PersonaManager interface {
	GetPrompt(ctx context.Context) (*ai.Prompt, error)
	ListPersonas(ctx context.Context) ([]Persona, error)
	// SetInMemoryPersonaYAML sets an in-memory persona from YAML bytes, bypassing file-based discovery.
	// When set, GetPrompt() will use this persona instead of discovering from files.
	SetInMemoryPersonaYAML(yamlContent []byte) error
}

// DefaultPersonaManager is the default implementation of PersonaManager
type DefaultPersonaManager struct {
	promptFactory       PersonaAwarePromptFactory
	configManager       config.Manager
	defaultPersona      string
	userHome            string
	inMemoryPersonaYAML []byte     // In-memory persona YAML bytes, bypasses file discovery when set
	inMemoryPrompt      *ai.Prompt // Cached prompt from in-memory persona
}

// NewDefaultPersonaManager creates a new DefaultPersonaManager with the given dependencies
func NewDefaultPersonaManager(promptFactory PersonaAwarePromptFactory, configManager config.Manager) PersonaManager {
	// Check GENIE_PERSONA environment variable via config manager, fallback to "genie"
	defaultPersona := configManager.GetStringWithDefault("GENIE_PERSONA", "genie")
	userHome, _ := os.UserHomeDir()

	return &DefaultPersonaManager{
		promptFactory:  promptFactory,
		configManager:  configManager,
		defaultPersona: defaultPersona,
		userHome:       userHome,
	}
}

func (m *DefaultPersonaManager) GetPrompt(ctx context.Context) (*ai.Prompt, error) {
	// If in-memory persona is set, use it instead of file-based discovery
	if m.inMemoryPrompt != nil {
		return m.inMemoryPrompt, nil
	}

	// Get persona from context, fallback to default
	persona := m.defaultPersona
	if contextPersona, ok := ctx.Value("persona").(string); ok && contextPersona != "" {
		persona = contextPersona
	}

	prompt, err := m.promptFactory.GetPrompt(ctx, persona)
	if err == nil {
		return prompt, nil
	}

	if persona != m.defaultPersona {
		fallbackPrompt, fallbackErr := m.promptFactory.GetPrompt(ctx, m.defaultPersona)
		if fallbackErr == nil {
			return fallbackPrompt, nil
		}
		return nil, fmt.Errorf("persona %s not found: %v (and default persona %s failed: %w)", persona, err, m.defaultPersona, fallbackErr)
	}

	return nil, fmt.Errorf("persona %s not found: %w", persona, err)
}

// SetInMemoryPersonaYAML sets an in-memory persona from YAML bytes, bypassing file-based discovery.
// When set, GetPrompt() will use this persona instead of discovering from files.
func (m *DefaultPersonaManager) SetInMemoryPersonaYAML(yamlContent []byte) error {
	if len(yamlContent) == 0 {
		return fmt.Errorf("persona YAML content is empty")
	}

	// Use the prompt factory to load and enhance the prompt (includes skill injection)
	prompt, err := m.promptFactory.GetPromptFromBytes(context.Background(), yamlContent)
	if err != nil {
		return fmt.Errorf("failed to load persona from YAML: %w", err)
	}

	m.inMemoryPersonaYAML = yamlContent
	m.inMemoryPrompt = prompt

	return nil
}

func (m *DefaultPersonaManager) ListPersonas(ctx context.Context) ([]Persona, error) {
	personaMap := make(map[string]Persona)

	// Get working directory from context, fallback to current directory
	cwd, ok := ctx.Value("cwd").(string)
	if !ok {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = ""
		}
	}

	// Load internal personas (lowest priority)
	internalPersonas, err := m.discoverInternalPersonas()
	if err == nil {
		for _, p := range internalPersonas {
			personaMap[p.ID] = p
		}
	}

	// Load user personas (medium priority)
	if m.userHome != "" {
		userPersonas, err := m.discoverPersonasInDir(filepath.Join(m.userHome, ".genie", "personas"), PersonaSourceUser)
		if err == nil {
			for _, p := range userPersonas {
				personaMap[p.ID] = p
			}
		}
	}

	// Load project personas (highest priority)
	if cwd != "" {
		projectPersonas, err := m.discoverPersonasInDir(filepath.Join(cwd, ".genie", "personas"), PersonaSourceProject)
		if err == nil {
			for _, p := range projectPersonas {
				personaMap[p.ID] = p
			}
		}
	}

	// Convert map to slice
	personas := make([]Persona, 0, len(personaMap))
	for _, p := range personaMap {
		personas = append(personas, p)
	}

	return personas, nil
}

// discoverInternalPersonas discovers personas from the embedded filesystem
func (m *DefaultPersonaManager) discoverInternalPersonas() ([]Persona, error) {
	var personas []Persona

	entries, err := personasFS.ReadDir("personas")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			personaID := entry.Name()
			promptPath := fmt.Sprintf("personas/%s/prompt.yaml", personaID)

			// Try to read the prompt file to get the name
			data, err := personasFS.ReadFile(promptPath)
			if err != nil {
				continue
			}

			name := m.extractNameFromPromptYAML(data, personaID)

			personas = append(personas, Persona{
				ID:     personaID,
				Name:   name,
				Source: PersonaSourceInternal,
			})
		}
	}

	return personas, nil
}

// discoverPersonasInDir discovers personas from a directory in the filesystem
func (m *DefaultPersonaManager) discoverPersonasInDir(dir string, source PersonaSource) ([]Persona, error) {
	var personas []Persona

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return personas, nil // Return empty list if directory doesn't exist
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			personaID := entry.Name()
			promptPath := filepath.Join(dir, personaID, "prompt.yaml")

			// Try to read the prompt file to get the name
			data, err := os.ReadFile(promptPath)
			if err != nil {
				continue
			}

			name := m.extractNameFromPromptYAML(data, personaID)

			personas = append(personas, Persona{
				ID:     personaID,
				Name:   name,
				Source: source,
			})
		}
	}

	return personas, nil
}

// extractNameFromPromptYAML extracts the name field from prompt YAML content
func (m *DefaultPersonaManager) extractNameFromPromptYAML(data []byte, defaultName string) string {
	var prompt struct {
		Name string `yaml:"name"`
	}

	if err := yaml.Unmarshal(data, &prompt); err != nil {
		return defaultName
	}

	if prompt.Name != "" {
		return prompt.Name
	}

	return defaultName
}
