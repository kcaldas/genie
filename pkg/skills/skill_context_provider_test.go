package skills

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
)

func newTestSkill(name, baseDir, content string) *Skill {
	return &Skill{
		SkillMetadata: SkillMetadata{
			Name:        name,
			Description: "test skill",
			Source:      SkillSourceProject,
			FilePath:    baseDir + "/SKILL.md",
		},
		Content:     content,
		BaseDir:     baseDir,
		LoadedFiles: make(map[string]string),
	}
}

func TestGetPartEmptyWhenNoActiveSkill(t *testing.T) {
	provider := NewSkillContextPartProvider(nil, events.NewEventBus())

	part, err := provider.GetPart(context.Background())
	if err != nil {
		t.Fatalf("GetPart returned error: %v", err)
	}
	if part.Key != "active_skill" {
		t.Errorf("Key = %q, want %q", part.Key, "active_skill")
	}
	if part.Content != "" {
		t.Errorf("Content should be empty without an active skill, got %q", part.Content)
	}
}

func TestGetPartRendersActiveSkillAfterInvokedEvent(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewSkillContextPartProvider(nil, bus)

	skill := newTestSkill("render-me", "/skills/render-me", "# Render Me\n\nDo the thing.")
	skill.LoadedFiles["docs/extra.md"] = "loaded file body"

	bus.PublishSync("skill.invoked", events.SkillInvokedEvent{Skill: skill})

	workingDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", workingDir)

	part, err := provider.GetPart(ctx)
	if err != nil {
		t.Fatalf("GetPart returned error: %v", err)
	}

	wantFragments := []string{
		"# Active Skill: render-me",
		"## Environment",
		"**WORKING_DIRECTORY**: `" + workingDir + "`",
		"**SKILL_DIRECTORY**: `/skills/render-me`",
		"## /skills/render-me/SKILL.md",
		"Do the thing.",
		"## /skills/render-me/docs/extra.md",
		"loaded file body",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(part.Content, fragment) {
			t.Errorf("GetPart content missing %q\ncontent:\n%s", fragment, part.Content)
		}
	}
}

func TestSkillInvokedEventAsMapPayload(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewSkillContextPartProvider(nil, bus)

	skill := newTestSkill("map-skill", "/skills/map-skill", "# Map")
	bus.PublishSync("skill.invoked", map[string]interface{}{"Skill": skill})

	if got := provider.GetActiveSkill(); got == nil || got.Name != "map-skill" {
		t.Errorf("active skill = %v, want map-skill", got)
	}
}

func TestSkillInvokedEventWithUnexpectedPayloadIsIgnored(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewSkillContextPartProvider(nil, bus)

	bus.PublishSync("skill.invoked", "not an event")
	if got := provider.GetActiveSkill(); got != nil {
		t.Errorf("active skill should stay nil on bogus payload, got %v", got)
	}

	// A payload of the right event type but wrong skill type is also ignored
	bus.PublishSync("skill.invoked", events.SkillInvokedEvent{Skill: "not a skill"})
	if got := provider.GetActiveSkill(); got != nil {
		t.Errorf("active skill should stay nil on wrong skill type, got %v", got)
	}
}

func TestSkillClearedEventClearsActiveSkill(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewSkillContextPartProvider(nil, bus)

	skill := newTestSkill("clear-me", "/skills/clear-me", "# Clear Me")
	bus.PublishSync("skill.invoked", events.SkillInvokedEvent{Skill: skill})
	if provider.GetActiveSkill() == nil {
		t.Fatal("expected active skill after invoked event")
	}

	bus.PublishSync("skill.cleared", events.SkillClearedEvent{})

	if got := provider.GetActiveSkill(); got != nil {
		t.Errorf("active skill should be nil after cleared event, got %v", got)
	}

	part, err := provider.GetPart(context.Background())
	if err != nil {
		t.Fatalf("GetPart returned error: %v", err)
	}
	if part.Content != "" {
		t.Errorf("Content should be empty after clear, got %q", part.Content)
	}
}

func TestClearPartClearsActiveSkill(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewSkillContextPartProvider(nil, bus)

	bus.PublishSync("skill.invoked", events.SkillInvokedEvent{Skill: newTestSkill("part", "/skills/part", "# P")})
	if err := provider.ClearPart(); err != nil {
		t.Fatalf("ClearPart returned error: %v", err)
	}
	if got := provider.GetActiveSkill(); got != nil {
		t.Errorf("active skill should be nil after ClearPart, got %v", got)
	}
}

func TestProviderConcurrentGetPartAndActivationIsRaceFree(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewSkillContextPartProvider(nil, bus)

	ctx := context.WithValue(context.Background(), "cwd", t.TempDir())

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				// Fresh skill per activation so readers never observe a
				// skill instance that is being mutated.
				skill := newTestSkill("racer", "/skills/racer", "# Racer")
				skill.LoadedFiles["extra.md"] = "extra"
				bus.PublishSync("skill.invoked", events.SkillInvokedEvent{Skill: skill})
				bus.PublishSync("skill.cleared", events.SkillClearedEvent{})
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if _, err := provider.GetPart(ctx); err != nil {
					t.Errorf("GetPart returned error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
