package tools

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/skills"
	"github.com/kcaldas/genie/pkg/toolctx"
)

// skillTestProvider serves two skills from memory and counts loads so tests
// can tell a real reload from an idempotent no-op.
type skillTestProvider struct {
	skills map[string]*skills.Skill
	loads  map[string]int
}

func newSkillTestProvider() *skillTestProvider {
	return &skillTestProvider{
		skills: map[string]*skills.Skill{
			"xlsx": {SkillMetadata: skills.SkillMetadata{Name: "xlsx", Description: "Spreadsheets"}, Content: "XLSX INSTRUCTIONS", BaseDir: "virtual/xlsx"},
			"pdf":  {SkillMetadata: skills.SkillMetadata{Name: "pdf", Description: "Documents"}, Content: "PDF INSTRUCTIONS", BaseDir: "virtual/pdf"},
		},
		loads: map[string]int{},
	}
}

func (p *skillTestProvider) ListSkills(context.Context) ([]skills.SkillMetadata, error) {
	list := make([]skills.SkillMetadata, 0, len(p.skills))
	for _, s := range p.skills {
		list = append(list, s.SkillMetadata)
	}
	return list, nil
}

func (p *skillTestProvider) LoadSkill(_ context.Context, name string) (*skills.Skill, error) {
	s, ok := p.skills[name]
	if !ok {
		return nil, &skills.SkillNotFoundError{Name: name}
	}
	p.loads[name]++
	return s, nil
}

func (p *skillTestProvider) ReadFile(_ context.Context, name, path string) ([]byte, error) {
	if _, ok := p.skills[name]; !ok {
		return nil, &skills.SkillNotFoundError{Name: name}
	}
	if path != "references/guide.md" {
		return nil, fs.ErrNotExist
	}
	p.loads[name+"/"+path]++
	return []byte("GUIDE CONTENT"), nil
}

func (p *skillTestProvider) ListFiles(context.Context, string) ([]string, error) {
	return []string{"references/guide.md"}, nil
}

func newSkillToolUnderTest(t *testing.T) (*SkillTool, *skillTestProvider, *skills.SkillContextPartProvider, context.Context) {
	t.Helper()
	provider := newSkillTestProvider()
	manager := skills.NewSkillManager(provider)
	tool := NewSkillTool(manager, nil)
	part := skills.NewSkillContextPartProvider(manager, nil)
	ctx := toolctx.WithSessionID(context.Background(), "test-session")
	return tool, provider, part, ctx
}

func activeSkillContext(t *testing.T, part *skills.SkillContextPartProvider, ctx context.Context) string {
	t.Helper()
	p, err := part.GetPart(ctx)
	require.NoError(t, err)
	return p.Content
}

func TestSkillToolLoadIsIdempotentWhenAlreadyActive(t *testing.T) {
	tool, provider, part, ctx := newSkillToolUnderTest(t)

	first, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)
	require.Equal(t, "loaded", first.Status)
	require.Equal(t, "XLSX INSTRUCTIONS", first.Content)
	before := activeSkillContext(t, part, ctx)
	require.Equal(t, 1, strings.Count(before, "# Active Skill:"))
	require.Equal(t, 1, strings.Count(before, "XLSX INSTRUCTIONS"))

	second, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)
	require.Equal(t, "already_active", second.Status)
	require.Equal(t, "xlsx", second.SkillName)
	require.Empty(t, second.Content, "already-active load must not re-inject the skill body")
	require.Contains(t, second.Message, `Skill 'xlsx' is already active`)
	require.Equal(t, 1, provider.loads["xlsx"], "provider must not be asked to load the skill again")

	after := activeSkillContext(t, part, ctx)
	require.Equal(t, before, after, "active skill context section must be unchanged")
	require.Equal(t, 1, strings.Count(after, "# Active Skill:"))
	require.Equal(t, 1, strings.Count(after, "XLSX INSTRUCTIONS"))
}

func TestSkillToolForceReloadsActiveSkill(t *testing.T) {
	tool, provider, _, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: "xlsx", Force: true})
	require.NoError(t, err)
	require.Equal(t, "loaded", resp.Status)
	require.Equal(t, "XLSX INSTRUCTIONS", resp.Content)
	require.Equal(t, 2, provider.loads["xlsx"])
}

func TestSkillToolDifferentSkillReplacesActiveSkill(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: "pdf"})
	require.NoError(t, err)
	require.Equal(t, "loaded", resp.Status)
	require.Equal(t, "PDF INSTRUCTIONS", resp.Content)

	content := activeSkillContext(t, part, ctx)
	require.Equal(t, 1, strings.Count(content, "# Active Skill:"))
	require.Contains(t, content, "# Active Skill: pdf")
	require.Contains(t, content, "PDF INSTRUCTIONS")
	require.NotContains(t, content, "XLSX INSTRUCTIONS")
}

func TestSkillToolFileOnActiveSkillLoadsOnce(t *testing.T) {
	tool, provider, part, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)

	withFile, err := tool.Run(ctx, SkillParams{Skill: "xlsx", File: "references/guide.md"})
	require.NoError(t, err)
	require.Equal(t, "loaded", withFile.Status)
	require.Empty(t, withFile.Content, "loading a file into the active skill must not re-inject SKILL.md")
	require.Contains(t, withFile.Message, "references/guide.md")
	require.Equal(t, 1, provider.loads["xlsx"], "skill itself is not reloaded")
	require.Equal(t, 1, provider.loads["xlsx/references/guide.md"])

	content := activeSkillContext(t, part, ctx)
	require.Equal(t, 1, strings.Count(content, "# Active Skill:"))
	require.Equal(t, 1, strings.Count(content, "GUIDE CONTENT"))

	again, err := tool.Run(ctx, SkillParams{Skill: "xlsx", File: "references/guide.md"})
	require.NoError(t, err)
	require.Equal(t, "already_active", again.Status)
	require.Equal(t, 1, provider.loads["xlsx/references/guide.md"], "file is not read again")
	require.Equal(t, content, activeSkillContext(t, part, ctx))
}

func TestSkillToolFileOnActiveSkillNotFoundIsAnError(t *testing.T) {
	tool, _, _, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: "xlsx", File: "missing.md"})
	require.Error(t, err)
	require.Equal(t, "error", resp.Status)
	require.Contains(t, resp.Message, "missing.md")
}

func TestSkillToolEmptyNameStillClears(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: ""})
	require.NoError(t, err)
	require.Equal(t, "completed", resp.Status)
	require.Empty(t, activeSkillContext(t, part, ctx))
}

func TestSkillToolDescriptionDoesNotRequireClear(t *testing.T) {
	decl := (&SkillTool{}).Declaration()
	require.NotContains(t, decl.Description, "Clear when done")
	require.Contains(t, decl.Description, "already_active")
	require.Contains(t, decl.Description, `Skill(skill="")`)
	require.Contains(t, decl.Parameters.Properties, "force")
}
