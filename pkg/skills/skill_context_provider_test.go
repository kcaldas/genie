package skills

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
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

func TestGetPartRendersActiveSkillFromManager(t *testing.T) {
	bus := events.NewEventBus()
	manager := NewSkillManager(newMemoryProvider())
	provider := NewSkillContextPartProvider(manager, bus)

	skill := newTestSkill("render-me", "/skills/render-me", "# Render Me\n\nDo the thing.")
	skill.LoadedFiles["docs/extra.md"] = "loaded file body"

	manager.provider.(*memoryProvider).skill = skill
	if _, err := manager.ActivateSkill(context.Background(), skill); err != nil {
		t.Fatal(err)
	}

	workingDir := t.TempDir()
	ctx := toolctx.WithWorkingDir(context.Background(), workingDir)

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

func TestProviderConcurrentGetPartAndActivationIsRaceFree(t *testing.T) {
	bus := events.NewEventBus()
	manager := NewSkillManager(newMemoryProvider())
	provider := NewSkillContextPartProvider(manager, bus)

	ctx := toolctx.WithWorkingDir(context.Background(), t.TempDir())

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				// Fresh skill per activation so readers never observe a
				// skill instance that is being mutated.
				skill := newTestSkill("host-skill", "/skills/host-skill", "# Racer")
				skill.LoadedFiles["extra.md"] = "extra"
				if _, err := manager.ActivateSkill(ctx, skill); err != nil {
					t.Error(err)
				}
				_ = manager.ClearActiveSkills(ctx)
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
