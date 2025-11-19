package skills

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
)

// SkillContextPartProvider provides the active skill's content as context
type SkillContextPartProvider struct {
	skillManager SkillManager
	eventBus     events.EventBus
	mu           sync.RWMutex
	activeSkill  *Skill
}

// NewSkillContextPartProvider creates a new skill context provider
func NewSkillContextPartProvider(skillManager SkillManager, eventBus events.EventBus) *SkillContextPartProvider {
	provider := &SkillContextPartProvider{
		skillManager: skillManager,
		eventBus:     eventBus,
	}

	if eventBus != nil {
		// Subscribe to skill lifecycle events
		eventBus.Subscribe("skill.invoked", provider.handleSkillInvoked)
		eventBus.Subscribe("skill.cleared", provider.handleSkillCleared)
	}

	return provider
}

// handleSkillInvoked handles skill.invoked events
func (p *SkillContextPartProvider) handleSkillInvoked(event interface{}) {
	// Try to extract the skill from the event
	var skill interface{}

	// Try direct event type from events package
	if se, ok := event.(events.SkillInvokedEvent); ok {
		skill = se.Skill
	} else if eventMap, ok := event.(map[string]interface{}); ok {
		// Fallback: try map access
		skill = eventMap["Skill"]
	} else {
		slog.Error("Unexpected event type for skill.invoked", "event_type", fmt.Sprintf("%T", event))
		return
	}

	// Convert interface{} to *Skill
	if s, ok := skill.(*Skill); ok {
		p.mu.Lock()
		p.activeSkill = s
		p.mu.Unlock()
		slog.Debug("Active skill set in context provider", "skill", s.Name, "base_dir", s.BaseDir)
	} else {
		slog.Error("Failed to convert skill to *Skill type", "skill_type", fmt.Sprintf("%T", skill))
	}
}

// handleSkillCleared handles skill.cleared events
func (p *SkillContextPartProvider) handleSkillCleared(event interface{}) {
	p.mu.Lock()
	previousSkill := p.activeSkill
	p.activeSkill = nil
	p.mu.Unlock()

	if previousSkill != nil {
		slog.Debug("Active skill cleared from context provider", "previous_skill", previousSkill.Name)
	} else {
		slog.Debug("Skill cleared event received but no active skill was set")
	}
}

// GetPart returns the active skill's content as context
func (p *SkillContextPartProvider) GetPart(c context.Context) (ctx.ContextPart, error) {
	p.mu.RLock()
	activeSkill := p.activeSkill
	p.mu.RUnlock()

	// If no active skill, return empty context
	if activeSkill == nil {
		return ctx.ContextPart{
			Key:     "active_skill",
			Content: "",
		}, nil
	}

	// Build content with base path and all loaded files
	var contentBuilder strings.Builder

	// Start with skill header
	contentBuilder.WriteString(fmt.Sprintf("# Active Skill: %s\n\n", activeSkill.Name))

	// Add working directory and paths information
	workingDir, ok := c.Value("cwd").(string)
	if !ok || workingDir == "" {
		workingDir, _ = os.Getwd()
	}

	contentBuilder.WriteString("## Environment\n")
	contentBuilder.WriteString(fmt.Sprintf("- **Working Directory**: `%s`\n", workingDir))
	contentBuilder.WriteString(fmt.Sprintf("- **Skill Directory**: `%s`\n", activeSkill.BaseDir))
	contentBuilder.WriteString("\n")
	contentBuilder.WriteString("## File Storage Rules\n")
	contentBuilder.WriteString(fmt.Sprintf("- **Temporary/Output Files**: MUST be saved to `tmp/` relative to working directory: `%s/tmp/`\n", workingDir))
	contentBuilder.WriteString("- **DO NOT** use `.genie/`, `.genie/temp/`, or any hidden directories for output files\n")
	contentBuilder.WriteString("- **Example**: To save `invoice.pdf`, use path: `tmp/invoice.pdf`\n")
	contentBuilder.WriteString("\n")

	// Add SKILL.md content with full path header
	skillFilePath := activeSkill.BaseDir + "/SKILL.md"
	contentBuilder.WriteString(fmt.Sprintf("## %s\n%s\n", skillFilePath, activeSkill.Content))

	// Add any loaded files
	if len(activeSkill.LoadedFiles) > 0 {
		for relPath, content := range activeSkill.LoadedFiles {
			fullPath := activeSkill.BaseDir + "/" + relPath
			contentBuilder.WriteString(fmt.Sprintf("\n## %s\n%s\n", fullPath, content))
		}
	}

	return ctx.ContextPart{
		Key:     "active_skill",
		Content: contentBuilder.String(),
	}, nil
}

// ClearPart clears the active skill
func (p *SkillContextPartProvider) ClearPart() error {
	p.mu.Lock()
	p.activeSkill = nil
	p.mu.Unlock()
	return nil
}

// GetActiveSkill returns the currently active skill (for testing)
func (p *SkillContextPartProvider) GetActiveSkill() *Skill {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.activeSkill
}
