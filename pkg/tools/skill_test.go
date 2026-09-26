package tools

import (
	"context"
	"fmt"
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
	p := &skillTestProvider{
		skills: map[string]*skills.Skill{
			"xlsx": {SkillMetadata: skills.SkillMetadata{Name: "xlsx", Description: "Spreadsheets"}, Content: "XLSX INSTRUCTIONS", BaseDir: "virtual/xlsx"},
			"pdf":  {SkillMetadata: skills.SkillMetadata{Name: "pdf", Description: "Documents"}, Content: "PDF INSTRUCTIONS", BaseDir: "virtual/pdf"},
		},
		loads: map[string]int{},
	}
	// Numbered skills for multi-skill and cap tests.
	for i := 1; i <= skills.DefaultMaxActiveSkills+1; i++ {
		name := fmt.Sprintf("s%02d", i)
		p.skills[name] = &skills.Skill{SkillMetadata: skills.SkillMetadata{Name: name, Description: "Numbered"}, Content: "BODY OF " + name, BaseDir: "virtual/" + name}
	}
	return p
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

	_, err = tool.Run(ctx, SkillParams{Skill: "pdf"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: "xlsx", Force: true})
	require.NoError(t, err)
	require.Equal(t, "loaded", resp.Status)
	require.Equal(t, "XLSX INSTRUCTIONS", resp.Content)
	require.Equal(t, 2, provider.loads["xlsx"])
	require.Equal(t, 1, provider.loads["pdf"], "force reloads only the named skill")
	require.Equal(t, []string{"xlsx", "pdf"}, resp.Active, "reload keeps the skill's position")
}

func TestSkillToolSecondSkillIsAddedAndBothRenderInLoadOrder(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: "pdf"})
	require.NoError(t, err)
	require.Equal(t, "loaded", resp.Status)
	require.Equal(t, "PDF INSTRUCTIONS", resp.Content)
	require.Equal(t, []string{"xlsx", "pdf"}, resp.Active)
	require.Empty(t, resp.Evicted)

	content := activeSkillContext(t, part, ctx)
	require.Equal(t, 2, strings.Count(content, "# Active Skill:"))
	require.Equal(t, 1, strings.Count(content, "XLSX INSTRUCTIONS"))
	require.Equal(t, 1, strings.Count(content, "PDF INSTRUCTIONS"))
	require.Less(t, strings.Index(content, "# Active Skill: xlsx"), strings.Index(content, "# Active Skill: pdf"))
	require.Less(t, strings.Index(content, "XLSX INSTRUCTIONS"), strings.Index(content, "PDF INSTRUCTIONS"))

	// Loading the first skill again is a no-op that changes nothing.
	again, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)
	require.Equal(t, "already_active", again.Status)
	require.Equal(t, []string{"xlsx", "pdf"}, again.Active)
	require.Equal(t, content, activeSkillContext(t, part, ctx))
}

func TestSkillToolClearOneLeavesTheOther(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)
	_, err = tool.Run(ctx, SkillParams{Skill: "pdf"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: "xlsx", Clear: true})
	require.NoError(t, err)
	require.Equal(t, "completed", resp.Status)
	require.Equal(t, "xlsx", resp.SkillName)
	require.Equal(t, []string{"pdf"}, resp.Active)
	require.Contains(t, resp.Message, "Skill 'xlsx' cleared")

	content := activeSkillContext(t, part, ctx)
	require.Equal(t, 1, strings.Count(content, "# Active Skill:"))
	require.Contains(t, content, "PDF INSTRUCTIONS")
	require.NotContains(t, content, "XLSX INSTRUCTIONS")

	// Clearing a skill that is not active is not an error.
	notActive, err := tool.Run(ctx, SkillParams{Skill: "xlsx", Clear: true})
	require.NoError(t, err)
	require.Equal(t, "completed", notActive.Status)
	require.Contains(t, notActive.Message, "was not active")
	require.Equal(t, []string{"pdf"}, notActive.Active)
}

func TestSkillToolSixSkillsRenderInLoadOrder(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	names := []string{"s01", "s02", "s03", "s04", "s05", "s06"}
	for _, name := range names {
		resp, err := tool.Run(ctx, SkillParams{Skill: name})
		require.NoError(t, err)
		require.Equal(t, "loaded", resp.Status)
		require.Empty(t, resp.Evicted)
	}

	content := activeSkillContext(t, part, ctx)
	require.Equal(t, len(names), strings.Count(content, "# Active Skill:"))
	last := -1
	for _, name := range names {
		require.Equal(t, 1, strings.Count(content, "BODY OF "+name))
		at := strings.Index(content, "# Active Skill: "+name)
		require.Greater(t, at, last, "%s must render after the skill loaded before it", name)
		last = at
	}
}

func TestSkillToolEvictsOldestBeyondCap(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	cap := skills.DefaultMaxActiveSkills
	for i := 1; i <= cap; i++ {
		resp, err := tool.Run(ctx, SkillParams{Skill: fmt.Sprintf("s%02d", i)})
		require.NoError(t, err)
		require.Empty(t, resp.Evicted)
	}
	require.Equal(t, cap, strings.Count(activeSkillContext(t, part, ctx), "# Active Skill:"))

	overflow := fmt.Sprintf("s%02d", cap+1)
	resp, err := tool.Run(ctx, SkillParams{Skill: overflow})
	require.NoError(t, err)
	require.Equal(t, "loaded", resp.Status)
	require.Equal(t, "s01", resp.Evicted)
	require.Contains(t, resp.Message, "'s01' was evicted")
	require.Len(t, resp.Active, cap)
	require.Equal(t, "s02", resp.Active[0])
	require.Equal(t, overflow, resp.Active[cap-1])

	content := activeSkillContext(t, part, ctx)
	require.Equal(t, cap, strings.Count(content, "# Active Skill:"))
	require.NotContains(t, content, "# Active Skill: s01\n")
	require.Contains(t, content, "BODY OF "+overflow)
}

func TestSkillToolCapIsConfigurable(t *testing.T) {
	provider := newSkillTestProvider()
	manager := skills.NewSkillManager(provider, skills.WithMaxActiveSkills(2))
	tool := NewSkillTool(manager, nil)
	ctx := toolctx.WithSessionID(context.Background(), "small-cap")

	for _, name := range []string{"xlsx", "pdf"} {
		_, err := tool.Run(ctx, SkillParams{Skill: name})
		require.NoError(t, err)
	}
	resp, err := tool.Run(ctx, SkillParams{Skill: "s01"})
	require.NoError(t, err)
	require.Equal(t, "xlsx", resp.Evicted)
	require.Equal(t, []string{"pdf", "s01"}, resp.Active)
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

func TestSkillToolFreshLoadWithMissingFileDoesNotActivate(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	failed, err := tool.Run(ctx, SkillParams{Skill: "xlsx", File: "missing.md"})
	require.Error(t, err)
	require.Equal(t, "error", failed.Status)
	require.Contains(t, failed.Message, "missing.md")
	require.Contains(t, failed.Message, "was NOT activated")
	require.Empty(t, failed.Active)
	require.Empty(t, activeSkillContext(t, part, ctx), "a failed fresh load must not leave the skill active")

	retry, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)
	require.Equal(t, "loaded", retry.Status, "retry must deliver the instructions, not already_active")
	require.Equal(t, "XLSX INSTRUCTIONS", retry.Content)

	content := activeSkillContext(t, part, ctx)
	require.Equal(t, 1, strings.Count(content, "# Active Skill:"))
	require.Contains(t, content, "XLSX INSTRUCTIONS")
}

func TestSkillToolFailedLoadLeavesActiveSetUntouchedAtCap(t *testing.T) {
	provider := newSkillTestProvider()
	manager := skills.NewSkillManager(provider, skills.WithMaxActiveSkills(2))
	tool := NewSkillTool(manager, nil)
	part := skills.NewSkillContextPartProvider(manager, nil)
	ctx := toolctx.WithSessionID(context.Background(), "cap-two")

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx", File: "references/guide.md"})
	require.NoError(t, err)
	_, err = tool.Run(ctx, SkillParams{Skill: "pdf"})
	require.NoError(t, err)
	before := activeSkillContext(t, part, ctx)
	require.Contains(t, before, "GUIDE CONTENT")

	failed, err := tool.Run(ctx, SkillParams{Skill: "s01", File: "missing.md"})
	require.Error(t, err)
	require.Equal(t, "error", failed.Status)
	require.Empty(t, failed.Evicted)
	require.NotContains(t, failed.Message, "evicted")
	require.Contains(t, failed.Message, "was NOT activated")
	require.Equal(t, []string{"xlsx", "pdf"}, failed.Active)

	active, err := manager.GetActiveSkills(ctx)
	require.NoError(t, err)
	require.Len(t, active, 2)
	require.Equal(t, "xlsx", active[0].Name)
	require.Equal(t, "pdf", active[1].Name)
	require.Equal(t, "GUIDE CONTENT", active[0].LoadedFiles["references/guide.md"])
	require.Equal(t, before, activeSkillContext(t, part, ctx), "context must be byte-identical after a failed load")
}

func TestSkillToolForceReloadWithMissingFileKeepsSkillActive(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)

	failed, err := tool.Run(ctx, SkillParams{Skill: "xlsx", File: "missing.md", Force: true})
	require.Error(t, err)
	require.Equal(t, "error", failed.Status)
	require.NotContains(t, failed.Message, "was NOT activated")
	require.Equal(t, []string{"xlsx"}, failed.Active)
	require.Contains(t, activeSkillContext(t, part, ctx), "XLSX INSTRUCTIONS")
}

func TestSkillToolEmptyNameClearsAll(t *testing.T) {
	tool, _, part, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)
	_, err = tool.Run(ctx, SkillParams{Skill: "pdf"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: ""})
	require.NoError(t, err)
	require.Equal(t, "completed", resp.Status)
	require.Empty(t, resp.Active)
	require.Empty(t, activeSkillContext(t, part, ctx))
}

func TestSkillToolFileWithoutNameGoesToMostRecentSkill(t *testing.T) {
	tool, provider, part, ctx := newSkillToolUnderTest(t)

	_, err := tool.Run(ctx, SkillParams{Skill: "xlsx"})
	require.NoError(t, err)
	_, err = tool.Run(ctx, SkillParams{Skill: "pdf"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, SkillParams{Skill: "", File: "references/guide.md"})
	require.NoError(t, err)
	require.Equal(t, "loaded", resp.Status)
	require.Equal(t, 1, provider.loads["pdf/references/guide.md"])
	require.Equal(t, 0, provider.loads["xlsx/references/guide.md"])
	require.Contains(t, activeSkillContext(t, part, ctx), "virtual/pdf/references/guide.md")
}

func TestSkillToolDescriptionDoesNotRequireClear(t *testing.T) {
	decl := (&SkillTool{}).Declaration()
	require.NotContains(t, decl.Description, "Clear when done")
	require.Contains(t, decl.Description, "already_active")
	require.Contains(t, decl.Description, `Skill(skill="")`)
	require.Contains(t, decl.Description, "clear=true")
	require.Contains(t, decl.Description, "Several skills can be active at once")
	require.Contains(t, decl.Description, fmt.Sprintf("at most %d skills", skills.DefaultMaxActiveSkills))
	require.Contains(t, decl.Parameters.Properties, "force")
	require.Contains(t, decl.Parameters.Properties, "clear")
}
