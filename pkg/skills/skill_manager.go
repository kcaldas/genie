package skills

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/toolctx"
)

// DefaultSkillManager owns active skills per session. Discovery and resource
// access are delegated to its provider; no provider data is mutated.
type DefaultSkillManager struct {
	provider     Provider
	mu           sync.RWMutex
	activeSkills map[string]*Skill
}

// NewSkillManager creates independent session state over a supplied provider.
// The provider must be non-nil and safe for concurrent use.
func NewSkillManager(provider Provider) *DefaultSkillManager {
	return &DefaultSkillManager{provider: provider, activeSkills: make(map[string]*Skill)}
}

// NewDefaultSkillManager uses the default filesystem and embedded provider.
func NewDefaultSkillManager() (*DefaultSkillManager, error) {
	p, err := NewDefaultProvider()
	if err != nil {
		return nil, err
	}
	return NewSkillManager(p), nil
}

// SetGenieHome configures the default filesystem provider's discovery root.
// Custom providers receive discovery context on each operation instead.
func (m *DefaultSkillManager) SetGenieHome(home string) {
	if p, ok := m.provider.(*DefaultProvider); ok {
		p.SetGenieHome(home)
	}
}

func (m *DefaultSkillManager) ListSkills(ctx context.Context) ([]SkillMetadata, error) {
	list, err := m.provider.ListSkills(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]SkillMetadata, len(list))
	for i, meta := range list {
		result[i] = cloneMetadata(meta)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (m *DefaultSkillManager) GetSkillMetadata(ctx context.Context, name string) (*SkillMetadata, error) {
	list, err := m.ListSkills(ctx)
	if err != nil {
		return nil, err
	}
	for _, meta := range list {
		if meta.Name == name {
			return &meta, nil
		}
	}
	return nil, &SkillNotFoundError{Name: name}
}

func (m *DefaultSkillManager) LoadSkill(ctx context.Context, name string) (*Skill, error) {
	if _, err := m.GetSkillMetadata(ctx, name); err != nil {
		return nil, err
	}
	skill, err := m.provider.LoadSkill(ctx, name)
	if err != nil {
		return nil, err
	}
	if skill == nil || skill.Name != name {
		return nil, fmt.Errorf("provider returned invalid skill for %q", name)
	}
	return cloneSkill(skill), nil
}

func (m *DefaultSkillManager) ListSkillFiles(ctx context.Context, name string) ([]string, error) {
	if _, err := m.GetSkillMetadata(ctx, name); err != nil {
		return nil, err
	}
	files, err := m.provider.ListFiles(ctx, name)
	if err != nil {
		return nil, err
	}
	result := append([]string(nil), files...)
	sort.Strings(result)
	return result, nil
}

func (m *DefaultSkillManager) LoadSkillFile(ctx context.Context, resource string) error {
	resource, err := cleanResourcePath(resource)
	if err != nil {
		return err
	}
	id := sessionID(ctx)
	m.mu.RLock()
	active := m.activeSkills[id]
	m.mu.RUnlock()
	if active == nil {
		return fmt.Errorf("no active skill to load file into; invoke Skill first")
	}
	if _, err := m.GetSkillMetadata(ctx, active.Name); err != nil {
		return err
	}
	data, err := m.provider.ReadFile(ctx, active.Name, resource)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeSkills[id] != active {
		return fmt.Errorf("active skill changed while loading resource")
	}
	active.LoadedFiles[resource] = string(data)
	return nil
}

func (m *DefaultSkillManager) GetActiveSkill(ctx context.Context) (*Skill, error) {
	m.mu.RLock()
	skill := cloneSkill(m.activeSkills[sessionID(ctx)])
	m.mu.RUnlock()
	if skill != nil {
		if _, err := m.GetSkillMetadata(ctx, skill.Name); err != nil {
			return nil, err
		}
	}
	return skill, nil
}

func (m *DefaultSkillManager) SetActiveSkill(ctx context.Context, skill *Skill) error {
	if skill == nil {
		return m.ClearActiveSkill(ctx)
	}
	if _, err := m.GetSkillMetadata(ctx, skill.Name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeSkills[sessionID(ctx)] = cloneSkill(skill)
	return nil
}

func (m *DefaultSkillManager) ClearActiveSkill(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.activeSkills, sessionID(ctx))
	return nil
}

func (m *DefaultSkillManager) ClearAllActiveSkills() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.activeSkills)
}

func cloneMetadata(meta SkillMetadata) SkillMetadata {
	meta.Metadata = maps.Clone(meta.Metadata)
	return meta
}

func cloneSkill(skill *Skill) *Skill {
	if skill == nil {
		return nil
	}
	copy := *skill
	copy.SkillMetadata = cloneMetadata(skill.SkillMetadata)
	copy.LoadedFiles = maps.Clone(skill.LoadedFiles)
	if copy.LoadedFiles == nil {
		copy.LoadedFiles = make(map[string]string)
	}
	return &copy
}

func sessionID(ctx context.Context) string {
	if id, ok := toolctx.SessionID(ctx); ok {
		return id
	}
	return "default"
}

func cleanResourcePath(resource string) (string, error) {
	if strings.Contains(resource, "\\") {
		return "", fmt.Errorf("resource paths must use forward slashes")
	}
	clean := filepath.Clean(resource)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("file path must be relative: %s", resource)
	}
	if startsWithDotDot(clean) {
		return "", fmt.Errorf("file path cannot start with ..: %s", resource)
	}
	if clean == "." {
		return "", fmt.Errorf("file path must name a resource")
	}
	return filepath.ToSlash(clean), nil
}

// isPathWithinBase checks if a path is within the base directory (no path traversal)
func isPathWithinBase(path, base string) bool {
	// Get absolute paths
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}

	// Check if path starts with base
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}

	// If relative path starts with "..", it's outside the base
	return !filepath.IsAbs(rel) && !startsWithDotDot(rel)
}

// startsWithDotDot checks if a path starts with ".."
func startsWithDotDot(path string) bool {
	return len(path) >= 2 && path[0] == '.' && path[1] == '.' && (len(path) == 2 || path[2] == filepath.Separator)
}
