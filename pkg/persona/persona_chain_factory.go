package persona

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/prompts"
)

// PersonaChainFactory creates chains based on persona name with discovery from multiple locations
type PersonaChainFactory struct {
	promptLoader prompts.Loader
	userHome     string
}

// NewPersonaChainFactory creates a new persona chain factory
func NewPersonaChainFactory(promptLoader prompts.Loader) PersonaAwareChainFactory {
	userHome, _ := os.UserHomeDir()
	
	return &PersonaChainFactory{
		promptLoader: promptLoader,
		userHome:     userHome,
	}
}

// CreateChain creates a chain for the specified persona
func (f *PersonaChainFactory) CreateChain(ctx context.Context, personaName string) (*ai.Chain, error) {
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

	// Load prompt from persona directory
	promptPath := filepath.Join(personaPath, "prompt.yaml")
	prompt, err := f.promptLoader.LoadPromptFromFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt from %s: %w", promptPath, err)
	}

	// Create simple conversation chain with the persona prompt
	chain := &ai.Chain{
		Name: fmt.Sprintf("%s-persona-chat", personaName),
		Steps: []interface{}{
			ai.ChainStep{
				Name:      "conversation",
				Prompt:    &prompt,
				ForwardAs: "response",
			},
		},
	}

	return chain, nil
}

// discoverPersona finds the persona directory following priority: project > user > internal
func (f *PersonaChainFactory) discoverPersona(cwd, personaName string) (string, error) {
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

	// 3. Check internal personas: pkg/persona/personas/{personaName}
	// This path is relative to the module root, we'll use a more robust approach
	internalPath := f.getInternalPersonaPath(personaName)
	if f.personaExists(internalPath) {
		return internalPath, nil
	}

	return "", fmt.Errorf("persona %s not found in any location (project, user, or internal)", personaName)
}

// personaExists checks if a persona directory exists and has a prompt.yaml file
func (f *PersonaChainFactory) personaExists(personaPath string) bool {
	if _, err := os.Stat(personaPath); os.IsNotExist(err) {
		return false
	}

	promptPath := filepath.Join(personaPath, "prompt.yaml")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		return false
	}

	return true
}

// getInternalPersonaPath returns the path to internal personas
// This assumes the personas directory is in the same module as this code
func (f *PersonaChainFactory) getInternalPersonaPath(personaName string) string {
	// Get the directory where this file is located
	_, filename, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(filename)
	
	// Navigate to personas directory relative to this package
	return filepath.Join(pkgDir, "personas", personaName)
}