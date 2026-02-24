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
	"github.com/kcaldas/genie/pkg/skills"
)

// personasFS embeds internal persona prompts for built-in personas
//
//go:embed personas/*
var personasFS embed.FS

// PersonaPromptFactory creates prompts based on persona name with discovery from multiple locations
type PersonaPromptFactory struct {
	promptLoader prompts.Loader
	skillManager skills.SkillManager
	userHome     string
}

// NewPersonaPromptFactory creates a new persona prompt factory
func NewPersonaPromptFactory(promptLoader prompts.Loader, skillManager skills.SkillManager) PersonaAwarePromptFactory {
	userHome, _ := os.UserHomeDir()

	return &PersonaPromptFactory{
		promptLoader: promptLoader,
		skillManager: skillManager,
		userHome:     userHome,
	}
}

// GetPromptFromBytes loads a prompt directly from YAML bytes and enhances it with skills.
// This is used for in-memory persona configuration, bypassing file-based discovery.
func (f *PersonaPromptFactory) GetPromptFromBytes(ctx context.Context, yamlContent []byte) (*ai.Prompt, error) {
	prompt, err := f.promptLoader.LoadPromptFromBytes(yamlContent)
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt from bytes: %w", err)
	}
	return f.enhancePromptWithSkills(ctx, &prompt)
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
			return f.enhancePromptWithSkills(ctx, &prompt)
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
			return f.enhancePromptWithSkills(ctx, &prompt)
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
		return f.enhancePromptWithSkills(ctx, &prompt)
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("unable to access internal persona %q at %s: %w", personaName, embeddedPath, statErr)
	}

	return nil, fmt.Errorf("persona %s not found in any location (project, user, or internal)", personaName)
}

// enhancePromptWithSkills injects available skills metadata into the prompt's instruction
func (f *PersonaPromptFactory) enhancePromptWithSkills(ctx context.Context, prompt *ai.Prompt) (*ai.Prompt, error) {
	// If no skill manager, return prompt as-is
	if f.skillManager == nil {
		return prompt, nil
	}

	// Get available skills
	skills, err := f.skillManager.ListSkills(ctx)
	if err != nil || len(skills) == 0 {
		// If we can't list skills or there are none, just return the original prompt
		return prompt, nil
	}

	// Build skills section
	skillsSection := f.buildSkillsSection(skills)

	// Inject into instruction
	if prompt.Instruction != "" {
		prompt.Instruction += "\n\n" + skillsSection
	} else {
		prompt.Instruction = skillsSection
	}

	return prompt, nil
}

// buildSkillsSection formats the skills metadata for injection into prompts
func (f *PersonaPromptFactory) buildSkillsSection(skillsList []skills.SkillMetadata) string {
	var sb strings.Builder

	sb.WriteString("## Skills (mandatory)\n\n")
	sb.WriteString("**Before replying to any user message**, scan the skill list below and match against the user's request.\n\n")
	sb.WriteString("- If exactly one skill clearly applies: load it with the `Skill` tool, then follow its instructions.\n")
	sb.WriteString("- If multiple skills could apply: choose the most specific one.\n")
	sb.WriteString("- If none clearly apply: respond normally without loading any skill.\n\n")

	sb.WriteString("**Available skills:**\n\n")
	for _, skill := range skillsList {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", skill.Name, skill.Description))
	}

	sb.WriteString("\n**How to use a skill:**\n")
	sb.WriteString("1. Load: `Skill(skill=\"skill-name\")` — the skill's full guidance will be injected into your context\n")
	sb.WriteString("2. Follow the skill's instructions exactly to complete the task\n")
	sb.WriteString("3. When done: `Skill(skill=\"\")` to clear the skill from context\n")

	return sb.String()
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
