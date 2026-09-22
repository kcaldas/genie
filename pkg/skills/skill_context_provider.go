package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
)

// SkillContextPartProvider renders the manager's session-scoped snapshot.
// Lifecycle events are notifications only, never a second source of state.
type SkillContextPartProvider struct{ skillManager SkillManager }

// NewSkillContextPartProvider uses the manager as the source of active state.
// The event bus argument is retained for callers; events no longer drive state.
func NewSkillContextPartProvider(manager SkillManager, _ events.EventBus) *SkillContextPartProvider {
	return &SkillContextPartProvider{skillManager: manager}
}
func (p *SkillContextPartProvider) SetTokenBudget(int) {}
func (p *SkillContextPartProvider) GetPart(c context.Context) (ctx.ContextPart, error) {
	empty := ctx.ContextPart{Key: "active_skill"}
	if p.skillManager == nil {
		return empty, nil
	}
	activeSkill, err := p.skillManager.GetActiveSkill(c)
	if err != nil {
		var unavailable *SkillNotFoundError
		if errors.As(err, &unavailable) {
			return empty, nil
		}
		return empty, err
	}
	if activeSkill == nil {
		return empty, nil
	}
	// Build content with base path and all loaded files
	var contentBuilder strings.Builder

	// Start with skill header
	fmt.Fprintf(&contentBuilder, "# Active Skill: %s\n\n", activeSkill.Name)

	// Add working directory and paths information
	workingDir, ok := toolctx.WorkingDir(c)
	if !ok || workingDir == "" {
		workingDir, _ = os.Getwd()
	}

	contentBuilder.WriteString("## Environment\n")
	fmt.Fprintf(&contentBuilder, "- **WORKING_DIRECTORY**: `%s`\n", workingDir)
	fmt.Fprintf(&contentBuilder, "- **SKILL_DIRECTORY**: `%s`\n", activeSkill.BaseDir)
	contentBuilder.WriteString("\n")
	contentBuilder.WriteString("## How to Execute Skill Scripts\n")
	fmt.Fprintf(&contentBuilder, "When skill instructions reference `$SKILL_DIRECTORY`, use this path: `%s`\n\n", activeSkill.BaseDir)
	contentBuilder.WriteString("**Example:**\n")
	contentBuilder.WriteString("- Instruction: `python3 $SKILL_DIRECTORY/scripts/generate.py input.json`\n")
	fmt.Fprintf(&contentBuilder, "- You run: `python3 %s/scripts/generate.py input.json`\n", activeSkill.BaseDir)
	contentBuilder.WriteString("\n")
	contentBuilder.WriteString("## File Storage Rules\n")
	fmt.Fprintf(&contentBuilder, "- **Temporary/Output Files**: MUST be saved to `tmp/` relative to working directory: `%s/tmp/`\n", workingDir)
	contentBuilder.WriteString("- **DO NOT** use `.genie/`, `.genie/temp/`, or any hidden directories for output files\n")
	contentBuilder.WriteString("- **Example**: To save `invoice.pdf`, use path: `tmp/invoice.pdf`\n")
	contentBuilder.WriteString("\n")

	// Add SKILL.md content with full path header
	skillFilePath := activeSkill.BaseDir + "/SKILL.md"
	fmt.Fprintf(&contentBuilder, "## %s\n%s\n", skillFilePath, activeSkill.Content)

	// Add any loaded files
	if len(activeSkill.LoadedFiles) > 0 {
		for _, relPath := range sortedResourceNames(activeSkill.LoadedFiles) {
			content := activeSkill.LoadedFiles[relPath]
			fullPath := activeSkill.BaseDir + "/" + relPath
			fmt.Fprintf(&contentBuilder, "\n## %s\n%s\n", fullPath, content)
		}
	}

	return ctx.ContextPart{
		Key:     "active_skill",
		Content: contentBuilder.String(),
	}, nil
}

// ClearPart clears active state for this manager's sessions.
func (p *SkillContextPartProvider) ClearPart() error {
	if p.skillManager != nil {
		p.skillManager.ClearAllActiveSkills()
	}
	return nil
}
func sortedResourceNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
