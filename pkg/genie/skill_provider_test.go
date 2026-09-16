package genie

import (
	"context"
	"io/fs"
	"testing"

	"github.com/kcaldas/genie/pkg/skills"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/require"
)

type hostSkillProvider struct{ name string }

func (p hostSkillProvider) ListSkills(ctx context.Context) ([]skills.SkillMetadata, error) {
	if id, _ := toolctx.SessionID(ctx); id == "blocked" {
		return nil, nil
	}
	return []skills.SkillMetadata{{Name: p.name, Description: "host catalog entry"}}, nil
}
func (p hostSkillProvider) LoadSkill(_ context.Context, name string) (*skills.Skill, error) {
	if name != p.name {
		return nil, &skills.SkillNotFoundError{Name: name}
	}
	return &skills.Skill{SkillMetadata: skills.SkillMetadata{Name: name, Description: "host catalog entry"}, Content: "host instructions", BaseDir: "virtual/" + name}, nil
}
func (p hostSkillProvider) ReadFile(_ context.Context, name, path string) ([]byte, error) {
	if name != p.name || path != "guide.md" {
		return nil, fs.ErrNotExist
	}
	return []byte("host reference"), nil
}
func (p hostSkillProvider) ListFiles(context.Context, string) ([]string, error) {
	return []string{"guide.md"}, nil
}

func TestCustomSkillProviderWiresAllConsumers(t *testing.T) {
	g, err := NewGenie(WithSkillProvider(hostSkillProvider{name: "host-only"}))
	require.NoError(t, err)
	c := g.(*core)
	ctx := toolctx.WithSessionID(context.Background(), "parent")
	// The prompt catalog uses the same manager as the tool and context.
	require.NoError(t, c.personaManager.SetInMemoryPersonaYAML([]byte("name: test\ninstruction: Base prompt\n")))
	prompt, err := c.personaManager.GetPrompt(ctx)
	require.NoError(t, err)
	require.Contains(t, prompt.Instruction, "host-only")
	require.NotContains(t, prompt.Instruction, "skill-creator")
	registry := c.toolRegistry
	tool, ok := registry.Get("Skill")
	require.True(t, ok)
	response, err := tool.(*tools.SkillTool).Run(ctx, tools.SkillParams{Skill: "host-only", File: "guide.md", ListFiles: true})
	require.NoError(t, err)
	require.Equal(t, "host instructions", response.Content)
	require.Equal(t, []string{"guide.md"}, response.Files)
	parts, err := c.contextMgr.GetContextParts(ctx)
	require.NoError(t, err)
	require.Contains(t, parts["active_skill"], "host reference")
	other := toolctx.WithSessionID(context.Background(), "other")
	parts, err = c.contextMgr.GetContextParts(other)
	require.NoError(t, err)
	require.Empty(t, parts["active_skill"])
	_, err = tool.(*tools.SkillTool).Run(ctx, tools.SkillParams{Skill: "skill-creator"})
	require.Error(t, err)
	child, _, err := (&nativeTaskExecutor{parent: c}).newChildGenie()
	require.NoError(t, err)
	childRegistry := child.(*core).toolRegistry
	childTool, ok := childRegistry.Get("Skill")
	require.True(t, ok)
	// Even identical session IDs cannot share activation state across instances.
	parts, err = child.(*core).contextMgr.GetContextParts(ctx)
	require.NoError(t, err)
	require.Empty(t, parts["active_skill"])
	_, err = childTool.(*tools.SkillTool).Run(ctx, tools.SkillParams{Skill: "host-only"})
	require.NoError(t, err)
	_, err = childTool.(*tools.SkillTool).Run(ctx, tools.SkillParams{})
	require.NoError(t, err)
	parts, err = c.contextMgr.GetContextParts(ctx)
	require.NoError(t, err)
	require.Contains(t, parts["active_skill"], "host reference")
}

func TestDefaultSkillManagersAreInstanceScoped(t *testing.T) {
	first, err := ProvideSkillManager()
	require.NoError(t, err)
	second, err := ProvideSkillManager()
	require.NoError(t, err)
	ctx := context.Background()
	skill, err := first.LoadSkill(ctx, "skill-creator")
	require.NoError(t, err)
	require.NoError(t, first.SetActiveSkill(ctx, skill))
	active, err := second.GetActiveSkill(ctx)
	require.NoError(t, err)
	require.Nil(t, active)
}

func TestProviderContextReachesInMemoryPersona(t *testing.T) {
	g, err := NewGenie(WithSkillProvider(hostSkillProvider{name: "host-only"}))
	require.NoError(t, err)
	manager := g.(*core).personaManager
	require.NoError(t, manager.SetInMemoryPersonaYAML([]byte("name: test\ninstruction: Base prompt\n")))
	allowed, err := manager.GetPrompt(context.Background())
	require.NoError(t, err)
	require.Contains(t, allowed.Instruction, "host-only")
	blocked := toolctx.WithSessionID(context.Background(), "blocked")
	prompt, err := manager.GetPrompt(blocked)
	require.NoError(t, err)
	require.NotContains(t, prompt.Instruction, "host-only")
	// A separate render must not mutate a previous caller's prompt.
	require.Contains(t, allowed.Instruction, "host-only")
}
