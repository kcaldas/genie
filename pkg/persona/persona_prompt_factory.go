package persona

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/prompts"
	"gopkg.in/yaml.v2"
)

// personasFS embeds internal persona prompts for built-in personas
//
//go:embed personas/*
var personasFS embed.FS

// PersonaPromptFactory creates prompts based on persona name with discovery from multiple locations
type PersonaPromptFactory struct {
	promptLoader prompts.Loader
	userHome     string
}

// NewPersonaPromptFactory creates a new persona prompt factory
func NewPersonaPromptFactory(promptLoader prompts.Loader) PersonaAwarePromptFactory {
	userHome, _ := os.UserHomeDir()

	return &PersonaPromptFactory{
		promptLoader: promptLoader,
		userHome:     userHome,
	}
}

func (f *PersonaPromptFactory) GetPrompt(ctx context.Context, personaName string) (*ai.Prompt, error) {
	// Get working directory from context, fallback to current directory
	cwd, ok := ctx.Value("cwd").(string)
	if !ok {
		// Fallback to current working directory for backward compatibility
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "" // Will skip project-level persona discovery
		}
	}

	// Discover persona location with priority: project > user > internal
	personaPath, err := f.discoverPersona(cwd, personaName)
	if err != nil {
		return nil, fmt.Errorf("failed to discover persona %s: %w", personaName, err)
	}

	// Load prompt from persona directory or embedded FS
	return f.getPrompt(personaPath)
}

func (f *PersonaPromptFactory) getPrompt(personaPath string) (*ai.Prompt, error) {
	// Load prompt from persona directory or embedded FS
	var prompt ai.Prompt

	if filepath.HasPrefix(personaPath, "embedded://") {
		// Extract persona name from embedded path
		embeddedPersonaName := filepath.Base(personaPath)
		var err error
		prompt, err = f.loadEmbeddedPersonaPrompt(embeddedPersonaName)
		if err != nil {
			return nil, fmt.Errorf("failed to load embedded persona prompt for %s: %w", embeddedPersonaName, err)
		}
	} else {
		// Load from file system
		promptPath := filepath.Join(personaPath, "prompt.yaml")
		var err error
		prompt, err = f.promptLoader.LoadPromptFromFile(promptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load prompt from %s: %w", promptPath, err)
		}
	}
	return &prompt, nil
}

// discoverPersona finds the persona directory following priority: project > user > internal
func (f *PersonaPromptFactory) discoverPersona(cwd, personaName string) (string, error) {
	// 1. Check project personas: $cwd/.genie/personas/{personaName}
	if cwd != "" {
		projectPath := filepath.Join(cwd, ".genie", "personas", personaName)
		if f.personaExists(projectPath) {
			return projectPath, nil
		}
	}

	// 2. Check user personas: ~/.genie/personas/{personaName}
	if f.userHome != "" {
		userPath := filepath.Join(f.userHome, ".genie", "personas", personaName)
		if f.personaExists(userPath) {
			return userPath, nil
		}
	}

	// 3. Check internal personas: embedded in binary
	if f.internalPersonaExists(personaName) {
		return f.getEmbeddedPersonaPath(personaName), nil
	}

	return "", fmt.Errorf("persona %s not found in any location (project, user, or internal)", personaName)
}

// personaExists checks if a persona directory exists and has a prompt.yaml file
func (f *PersonaPromptFactory) personaExists(personaPath string) bool {
	if _, err := os.Stat(personaPath); os.IsNotExist(err) {
		return false
	}

	promptPath := filepath.Join(personaPath, "prompt.yaml")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		return false
	}

	return true
}

// internalPersonaExists checks if a persona exists in the embedded FS
func (f *PersonaPromptFactory) internalPersonaExists(personaName string) bool {
	promptPath := filepath.Join("personas", personaName, "prompt.yaml")
	_, err := personasFS.ReadFile(promptPath)
	return err == nil
}

// getEmbeddedPersonaPath returns a special marker for embedded personas
// This will be handled differently in the prompt loading
func (f *PersonaPromptFactory) getEmbeddedPersonaPath(personaName string) string {
	return fmt.Sprintf("embedded://personas/%s", personaName)
}

// loadEmbeddedPersonaPrompt loads a persona prompt from the embedded FS
// and enhances it using the full prompt loader pipeline
func (f *PersonaPromptFactory) loadEmbeddedPersonaPrompt(personaName string) (ai.Prompt, error) {
	promptPath := filepath.Join("personas", personaName, "prompt.yaml")

	// Read from embedded FS
	data, err := personasFS.ReadFile(promptPath)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("failed to read embedded persona prompt %s: %w", promptPath, err)
	}

	// Parse YAML
	var prompt ai.Prompt
	err = yaml.Unmarshal(data, &prompt)
	if err != nil {
		return ai.Prompt{}, fmt.Errorf("failed to unmarshal embedded persona prompt %s: %w", promptPath, err)
	}

	// Apply model defaults and tool enhancement using the prompt loader
	// We need to cast to access the private methods
	if defaultLoader, ok := f.promptLoader.(*prompts.DefaultLoader); ok {
		// Apply model defaults
		defaultLoader.ApplyModelDefaults(&prompt)

		// Add tools based on required_tools
		err = defaultLoader.AddTools(&prompt)
		if err != nil {
			return ai.Prompt{}, fmt.Errorf("failed to enhance embedded persona prompt %s with tools: %w", promptPath, err)
		}
	}

	return prompt, nil
}
