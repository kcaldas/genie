package persona

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/prompts"
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

	// Try loading personas in order: project > user > internal
	var prompt ai.Prompt
	var err error

	// 1. Try project personas: $cwd/.genie/personas/{personaName}/prompt.yaml
	if cwd != "" {
		cwdFS := os.DirFS(cwd)
		// Note: fs.FS always uses forward slashes, regardless of OS
		relativePath := ".genie/personas/" + personaName + "/prompt.yaml"
		prompt, err = f.promptLoader.LoadPromptFromFS(cwdFS, relativePath)
		if err == nil {
			return &prompt, nil
		}
	}

	// 2. Try user personas: ~/.genie/personas/{personaName}/prompt.yaml
	if f.userHome != "" {
		homeFS := os.DirFS(f.userHome)
		// Note: fs.FS always uses forward slashes, regardless of OS
		relativePath := ".genie/personas/" + personaName + "/prompt.yaml"
		prompt, err = f.promptLoader.LoadPromptFromFS(homeFS, relativePath)
		if err == nil {
			return &prompt, nil
		}
	}

	// 3. Try internal personas from embedded FS
	// Note: embedded FS always uses forward slashes, regardless of OS
	embeddedPath := "personas/" + personaName + "/prompt.yaml"
	prompt, err = f.promptLoader.LoadPromptFromFS(personasFS, embeddedPath)
	if err == nil {
		return &prompt, nil
	}

	return nil, fmt.Errorf("persona %s not found in any location (project, user, or internal): %w", personaName, err)
}
