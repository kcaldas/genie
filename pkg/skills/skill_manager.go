package skills

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/toolctx"
)

// DefaultMaxActiveSkills is the per-session cap on active skills. It is a
// safety valve against unbounded context growth, not a working limit: when
// exceeded, the oldest active skill is evicted.
const DefaultMaxActiveSkills = 16

// DefaultSkillManager owns active skills per session. Several skills can be
// active at once; they are kept in load order until cleared. Discovery and
// resource access are delegated to its provider; no provider data is mutated.
type DefaultSkillManager struct {
	provider        Provider
	maxActiveSkills int
	mu              sync.RWMutex
	activeSkills    map[string][]*Skill // per session, in load order
}

// ManagerOption configures a DefaultSkillManager.
type ManagerOption func(*DefaultSkillManager)

// WithMaxActiveSkills overrides the per-session cap on active skills.
// Values below 1 keep the default.
func WithMaxActiveSkills(n int) ManagerOption {
	return func(m *DefaultSkillManager) {
		if n >= 1 {
			m.maxActiveSkills = n
		}
	}
}

// NewSkillManager creates independent session state over a supplied provider.
// The provider must be non-nil and safe for concurrent use.
func NewSkillManager(provider Provider, opts ...ManagerOption) *DefaultSkillManager {
	m := &DefaultSkillManager{
		provider:        provider,
		maxActiveSkills: DefaultMaxActiveSkills,
		activeSkills:    make(map[string][]*Skill),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// NewDefaultSkillManager uses the default filesystem and embedded provider.
func NewDefaultSkillManager(opts ...ManagerOption) (*DefaultSkillManager, error) {
	p, err := NewDefaultProvider()
	if err != nil {
		return nil, err
	}
	return NewSkillManager(p, opts...), nil
}

// MaxActiveSkills returns the per-session cap on active skills.
func (m *DefaultSkillManager) MaxActiveSkills() int { return m.maxActiveSkills }

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

// ReadSkillFile reads a resource of a skill through the provider without
// touching session state. It returns the cleaned resource path (the key
// used in Skill.LoadedFiles) and the content.
func (m *DefaultSkillManager) ReadSkillFile(ctx context.Context, skillName, resource string) (string, string, error) {
	resource, err := cleanResourcePath(resource)
	if err != nil {
		return "", "", err
	}
	if _, err := m.GetSkillMetadata(ctx, skillName); err != nil {
		return "", "", err
	}
	data, err := m.provider.ReadFile(ctx, skillName, resource)
	if err != nil {
		return "", "", err
	}
	return resource, string(data), nil
}

func (m *DefaultSkillManager) LoadSkillFile(ctx context.Context, skillName, resource string) error {
	id := sessionID(ctx)
	m.mu.RLock()
	active := findActive(m.activeSkills[id], skillName)
	m.mu.RUnlock()
	if active == nil {
		if skillName == "" {
			return fmt.Errorf("no active skill to load file into; invoke Skill first")
		}
		return fmt.Errorf("skill %q is not active; invoke Skill first", skillName)
	}
	resource, content, err := m.ReadSkillFile(ctx, active.Name, resource)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if findActive(m.activeSkills[id], active.Name) != active {
		return fmt.Errorf("active skill changed while loading resource")
	}
	active.LoadedFiles[resource] = content
	return nil
}

// GetActiveSkills returns the session's active skills in load order. A skill
// whose catalog entry is no longer available is omitted; other catalog
// failures are returned.
func (m *DefaultSkillManager) GetActiveSkills(ctx context.Context) ([]*Skill, error) {
	m.mu.RLock()
	active := m.activeSkills[sessionID(ctx)]
	result := make([]*Skill, 0, len(active))
	for _, skill := range active {
		result = append(result, cloneSkill(skill))
	}
	m.mu.RUnlock()
	available := result[:0]
	for _, skill := range result {
		if _, err := m.GetSkillMetadata(ctx, skill.Name); err != nil {
			var notFound *SkillNotFoundError
			if errors.As(err, &notFound) {
				continue
			}
			return nil, err
		}
		available = append(available, skill)
	}
	return available, nil
}

// GetActiveSkill returns the most recently loaded active skill, or nil.
func (m *DefaultSkillManager) GetActiveSkill(ctx context.Context) (*Skill, error) {
	m.mu.RLock()
	skill := cloneSkill(findActive(m.activeSkills[sessionID(ctx)], ""))
	m.mu.RUnlock()
	if skill != nil {
		if _, err := m.GetSkillMetadata(ctx, skill.Name); err != nil {
			return nil, err
		}
	}
	return skill, nil
}

// ActivateSkill adds the skill to the session's active set. A skill that is
// already active is replaced in place, keeping its position in load order.
// When the set would exceed the cap, the oldest active skill is evicted and
// its name returned.
func (m *DefaultSkillManager) ActivateSkill(ctx context.Context, skill *Skill) (evicted string, err error) {
	if skill == nil {
		return "", fmt.Errorf("cannot activate a nil skill")
	}
	if _, err := m.GetSkillMetadata(ctx, skill.Name); err != nil {
		return "", err
	}
	id := sessionID(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	active := m.activeSkills[id]
	for i, existing := range active {
		if existing.Name == skill.Name {
			active[i] = cloneSkill(skill)
			return "", nil
		}
	}
	if len(active) >= m.maxActiveSkills {
		evicted = active[0].Name
		active = append([]*Skill(nil), active[1:]...)
	}
	m.activeSkills[id] = append(active, cloneSkill(skill))
	return evicted, nil
}

// DeactivateSkill removes one skill from the session's active set and reports
// whether it was active.
func (m *DefaultSkillManager) DeactivateSkill(ctx context.Context, name string) (bool, error) {
	id := sessionID(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	active := m.activeSkills[id]
	for i, existing := range active {
		if existing.Name == name {
			m.activeSkills[id] = append(active[:i:i], active[i+1:]...)
			if len(m.activeSkills[id]) == 0 {
				delete(m.activeSkills, id)
			}
			return true, nil
		}
	}
	return false, nil
}

// ClearActiveSkills removes every active skill from the session.
func (m *DefaultSkillManager) ClearActiveSkills(ctx context.Context) error {
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

// findActive returns the active skill with the given name, or the most
// recently loaded one when name is empty. Callers hold m.mu.
func findActive(active []*Skill, name string) *Skill {
	if len(active) == 0 {
		return nil
	}
	if name == "" {
		return active[len(active)-1]
	}
	for _, skill := range active {
		if skill.Name == name {
			return skill
		}
	}
	return nil
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
