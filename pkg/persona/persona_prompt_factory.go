package persona

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	// Get genie home directory from context (where .genie/ config lives)
	// Fall back to "cwd" for backward compatibility, then os.Getwd()
	genieHome, ok := ctx.Value("genie_home").(string)
	if !ok {
		// Try fallback to cwd for backward compatibility
		genieHome, ok = ctx.Value("cwd").(string)
		if !ok {
			// Final fallback to current working directory
			var err error
			genieHome, err = os.Getwd()
			if err != nil {
				genieHome = "" // Will skip project-level persona discovery
			}
		}
	}

	// Try loading personas in order: project > user > internal
	var prompt ai.Prompt
	var err error

	// 1. Try project personas: $genieHome/.genie/personas/{personaName}/prompt.yaml
	if genieHome != "" {
		genieHomeFS := os.DirFS(genieHome)
		// Note: fs.FS always uses forward slashes, regardless of OS
		relativePath := ".genie/personas/" + personaName + "/prompt.yaml"
		projectPath := filepath.Join(genieHome, relativePath)

		if _, statErr := fs.Stat(genieHomeFS, relativePath); statErr == nil {
			prompt, err = f.promptLoader.LoadPromptFromFS(genieHomeFS, relativePath)
			if err != nil {
				return nil, formatPersonaLoadError("project", personaName, projectPath, err)
			}
			return &prompt, nil
		} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("unable to access project persona %q at %s: %w", personaName, projectPath, statErr)
		}
	}

	// 2. Try user personas: ~/.genie/personas/{personaName}/prompt.yaml
	if f.userHome != "" {
		homeFS := os.DirFS(f.userHome)
		// Note: fs.FS always uses forward slashes, regardless of OS
		relativePath := ".genie/personas/" + personaName + "/prompt.yaml"
		userPath := filepath.Join(f.userHome, relativePath)

		if _, statErr := fs.Stat(homeFS, relativePath); statErr == nil {
			prompt, err = f.promptLoader.LoadPromptFromFS(homeFS, relativePath)
			if err != nil {
				return nil, formatPersonaLoadError("user", personaName, userPath, err)
			}
			return &prompt, nil
		} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("unable to access user persona %q at %s: %w", personaName, userPath, statErr)
		}
	}

	// 3. Try internal personas from embedded FS
	// Note: embedded FS always uses forward slashes, regardless of OS
	embeddedPath := "personas/" + personaName + "/prompt.yaml"
	if _, statErr := fs.Stat(personasFS, embeddedPath); statErr == nil {
		prompt, err = f.promptLoader.LoadPromptFromFS(personasFS, embeddedPath)
		if err != nil {
			return nil, formatPersonaLoadError("internal", personaName, embeddedPath, err)
		}
		return &prompt, nil
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("unable to access internal persona %q at %s: %w", personaName, embeddedPath, statErr)
	}

	return nil, fmt.Errorf("persona %s not found in any location (project, user, or internal)", personaName)
}

func formatPersonaLoadError(source, personaName, location string, loadErr error) error {
	hint := "Please resolve the error above and try again."
	if strings.Contains(loadErr.Error(), "missing required tools") {
		hint = "To fix this, either register the missing tools or remove them from the persona's required_tools list."
	}

	if strings.TrimSpace(location) != "" {
		return fmt.Errorf("failed to load %s persona %q from %s: %w\n\n%s", source, personaName, location, loadErr, hint)
	}
	return fmt.Errorf("failed to load %s persona %q: %w\n\n%s", source, personaName, loadErr, hint)
}
