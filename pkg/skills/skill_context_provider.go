package skills

import (
	"context"
	"fmt"
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
	// The event could be a map or struct with a Skill field
	var skill interface{}

	// Try type assertion for map
	if eventMap, ok := event.(map[string]interface{}); ok {
		skill = eventMap["Skill"]
	} else {
		// Try struct with Skill field
		type skillInvokedEvent interface {
			GetSkill() interface{}
		}
		if se, ok := event.(skillInvokedEvent); ok {
			skill = se.GetSkill()
		} else {
			// Try direct skill invoked event
			if se, ok := event.(SkillInvokedEvent); ok {
				skill = se.Skill
			}
		}
	}

	// Convert interface{} to *Skill
	if s, ok := skill.(*Skill); ok {
		p.mu.Lock()
		p.activeSkill = s
		p.mu.Unlock()
	}
}

// handleSkillCleared handles skill.cleared events
func (p *SkillContextPartProvider) handleSkillCleared(event interface{}) {
	p.mu.Lock()
	p.activeSkill = nil
	p.mu.Unlock()
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

	// Format skill content for context
	content := fmt.Sprintf(`# Active Skill: %s

%s`, activeSkill.Name, activeSkill.Content)

	return ctx.ContextPart{
		Key:     "active_skill",
		Content: content,
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

// SkillInvokedEvent is published when a skill is invoked
type SkillInvokedEvent struct {
	Skill *Skill
}

// Topic returns the event topic
func (e SkillInvokedEvent) Topic() string {
	return "skill.invoked"
}

// SkillClearedEvent is published when a skill is cleared
type SkillClearedEvent struct{}

// Topic returns the event topic
func (e SkillClearedEvent) Topic() string {
	return "skill.cleared"
}
