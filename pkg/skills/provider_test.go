package skills

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/stretchr/testify/require"
)

// A provider with no local directories proves resources don't bypass it.
type memoryProvider struct {
	denied bool
	skill  *Skill
}

func (p *memoryProvider) ListSkills(context.Context) ([]SkillMetadata, error) {
	if p.denied {
		return nil, nil
	}
	return []SkillMetadata{p.skill.SkillMetadata}, nil
}
func (p *memoryProvider) LoadSkill(_ context.Context, name string) (*Skill, error) {
	if p.denied || name != p.skill.Name {
		return nil, &SkillNotFoundError{Name: name}
	}
	return p.skill, nil
}
func (p *memoryProvider) ReadFile(ctx context.Context, name, path string) ([]byte, error) {
	if _, err := p.LoadSkill(ctx, name); err != nil {
		return nil, err
	}
	if path != "references/guide.md" {
		return nil, fs.ErrNotExist
	}
	return []byte("provider resource"), nil
}
func (p *memoryProvider) ListFiles(ctx context.Context, name string) ([]string, error) {
	if _, err := p.LoadSkill(ctx, name); err != nil {
		return nil, err
	}
	return []string{"references/guide.md"}, nil
}
func newMemoryProvider() *memoryProvider {
	return &memoryProvider{skill: &Skill{SkillMetadata: SkillMetadata{Name: "host-skill", Description: "Host supplied"}, Content: "host instructions", BaseDir: "virtual/host-skill"}}
}

func TestProviderOwnsDiscoveryLoadingAndResources(t *testing.T) {
	p := newMemoryProvider()
	m := NewSkillManager(p)
	ctx := toolctx.WithSessionID(context.Background(), "a")
	list, err := m.ListSkills(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "host-skill", list[0].Name)
	_, err = m.LoadSkill(ctx, "skill-creator")
	require.Error(t, err, "custom provider must replace default discovery")
	skill, err := m.LoadSkill(ctx, "host-skill")
	require.NoError(t, err)
	require.NoError(t, m.SetActiveSkill(ctx, skill))
	require.NoError(t, m.LoadSkillFile(ctx, "references/guide.md"))
	active, err := m.GetActiveSkill(ctx)
	require.NoError(t, err)
	require.Equal(t, "provider resource", active.LoadedFiles["references/guide.md"])
	files, err := m.ListSkillFiles(ctx, "host-skill")
	require.NoError(t, err)
	require.Equal(t, []string{"references/guide.md"}, files)
	require.Empty(t, p.skill.LoadedFiles, "session mutations must not alter provider-owned data")
}

func TestProviderRevocationStopsActiveContextAndResourceReads(t *testing.T) {
	p := newMemoryProvider()
	m := NewSkillManager(p)
	ctx := context.Background()
	skill, err := m.LoadSkill(ctx, "host-skill")
	require.NoError(t, err)
	require.NoError(t, m.SetActiveSkill(ctx, skill))
	p.denied = true
	active, err := m.GetActiveSkill(ctx)
	require.Error(t, err)
	require.Nil(t, active)
	require.Error(t, m.LoadSkillFile(ctx, "references/guide.md"))
	require.Error(t, m.SetActiveSkill(ctx, skill))
}

func TestProviderSessionSnapshotsAreIndependent(t *testing.T) {
	p := newMemoryProvider()
	m := NewSkillManager(p)
	a := toolctx.WithSessionID(context.Background(), "a")
	b := toolctx.WithSessionID(context.Background(), "b")
	skill, err := m.LoadSkill(a, "host-skill")
	require.NoError(t, err)
	require.NoError(t, m.SetActiveSkill(a, skill))
	require.NoError(t, m.SetActiveSkill(b, skill))
	require.NoError(t, m.LoadSkillFile(a, "references/guide.md"))
	activeB, err := m.GetActiveSkill(b)
	require.NoError(t, err)
	require.Empty(t, activeB.LoadedFiles)
	activeA, err := m.GetActiveSkill(a)
	require.NoError(t, err)
	activeA.LoadedFiles["references/guide.md"] = "mutated"
	again, err := m.GetActiveSkill(a)
	require.NoError(t, err)
	require.Equal(t, "provider resource", again.LoadedFiles["references/guide.md"])
}

func TestSkillContextUsesSessionState(t *testing.T) {
	m, _, root := newTestManager(t)
	writeSkillDir(t, filepath.Join(root, ".genie", "skills"), "session-context", "session-context", "Scoped", "Session instructions")
	p := NewSkillContextPartProvider(m, events.NewEventBus())
	a := toolctx.WithSessionID(context.Background(), "a")
	b := toolctx.WithSessionID(context.Background(), "b")
	skill, err := m.LoadSkill(a, "session-context")
	require.NoError(t, err)
	require.NoError(t, m.SetActiveSkill(a, skill))
	part, err := p.GetPart(a)
	require.NoError(t, err)
	require.Contains(t, part.Content, "Session instructions")
	part, err = p.GetPart(b)
	require.NoError(t, err)
	require.Empty(t, part.Content)
	require.NoError(t, p.ClearPart())
	active, err := m.GetActiveSkill(a)
	require.NoError(t, err)
	require.Nil(t, active)
}

func TestProviderConcurrentResourcesAndContext(t *testing.T) {
	m := NewSkillManager(newMemoryProvider())
	ctx := context.Background()
	skill, err := m.LoadSkill(ctx, "host-skill")
	require.NoError(t, err)
	require.NoError(t, m.SetActiveSkill(ctx, skill))
	p := NewSkillContextPartProvider(m, events.NewEventBus())
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 30 {
				if err := m.LoadSkillFile(ctx, "references/guide.md"); err != nil {
					t.Error(err)
				}
			}
		}()
		go func() {
			defer wg.Done()
			for range 30 {
				if _, err := p.GetPart(ctx); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestDefaultProviderEmbeddedResources(t *testing.T) {
	m, _, _ := newTestManager(t)
	ctx := context.Background()
	skill, err := m.LoadSkill(ctx, "skill-creator")
	require.NoError(t, err)
	require.NoError(t, m.SetActiveSkill(ctx, skill))
	require.NoError(t, m.LoadSkillFile(ctx, "scripts/init_skill.py"))
	files, err := m.ListSkillFiles(ctx, "skill-creator")
	require.NoError(t, err)
	require.Contains(t, files, "scripts/init_skill.py")
}

func TestSkillLoadErrorUnwrap(t *testing.T) {
	require.True(t, errors.Is(&SkillLoadError{Name: "broken", Cause: fs.ErrPermission}, fs.ErrPermission))
}

func TestDefaultProviderConcurrentDiscoveryRoots(t *testing.T) {
	m, _, _ := newTestManager(t)
	roots := []string{t.TempDir(), t.TempDir()}
	for i, root := range roots {
		body := []string{"first agent", "second agent"}[i]
		writeSkillDir(t, filepath.Join(root, ".genie", "skills"), "same-name", "same-name", "Scoped", body)
	}
	var wg sync.WaitGroup
	for i, root := range roots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := toolctx.WithGenieHome(context.Background(), root)
			for range 50 {
				skill, err := m.LoadSkill(ctx, "same-name")
				if err != nil {
					t.Error(err)
					return
				}
				if skill.Content != []string{"first agent", "second agent"}[i] {
					t.Errorf("crossed discovery roots: %q", skill.Content)
				}
			}
		}()
	}
	wg.Wait()
}

func TestDefaultProviderPreservesExtensionMetadata(t *testing.T) {
	m, _, root := newTestManager(t)
	dir := writeSkillDir(t, filepath.Join(root, ".genie", "skills"), "metadata-skill", "metadata-skill", "Metadata", "body")
	writeFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: metadata-skill\ndescription: Metadata\nmetadata:\n  mutiro.requires-tools: readFile writeFile\n---\nbody\n")
	meta, err := m.GetSkillMetadata(context.Background(), "metadata-skill")
	require.NoError(t, err)
	require.Equal(t, "readFile writeFile", meta.Metadata["mutiro.requires-tools"])
	meta.Metadata["mutiro.requires-tools"] = "mutated"
	skill, err := m.LoadSkill(context.Background(), "metadata-skill")
	require.NoError(t, err)
	require.Equal(t, "readFile writeFile", skill.Metadata["mutiro.requires-tools"])
	again, err := m.GetSkillMetadata(context.Background(), "metadata-skill")
	require.NoError(t, err)
	require.Equal(t, "readFile writeFile", again.Metadata["mutiro.requires-tools"])
}

func TestUnavailableSkillIsOmittedFromActiveContext(t *testing.T) {
	p := newMemoryProvider()
	m := NewSkillManager(p)
	ctx := context.Background()
	skill, err := m.LoadSkill(ctx, "host-skill")
	require.NoError(t, err)
	require.NoError(t, m.SetActiveSkill(ctx, skill))
	renderer := NewSkillContextPartProvider(m, nil)
	part, err := renderer.GetPart(ctx)
	require.NoError(t, err)
	require.Contains(t, part.Content, "host instructions")
	p.denied = true
	part, err = renderer.GetPart(ctx)
	require.NoError(t, err)
	require.Empty(t, part.Content)
}
