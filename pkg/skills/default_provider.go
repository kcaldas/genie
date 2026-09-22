package skills

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/toolctx"
)

//go:embed internal/skills
var internalSkillsFS embed.FS

// DefaultProvider discovers embedded, user-home and project skills, in that
// precedence order. Project .claude/skills takes precedence over .genie/skills.
// It owns no session state and may be shared by managers.
type DefaultProvider struct {
	loader    *SkillLoader
	userHome  string
	genieHome string
	mu        sync.Mutex
	catalogs  map[string]map[string]*SkillMetadata
}

// NewDefaultProvider creates a provider with the standard discovery sources.
func NewDefaultProvider() (*DefaultProvider, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home: %w", err)
	}
	return &DefaultProvider{loader: NewSkillLoader(), userHome: home, catalogs: make(map[string]map[string]*SkillMetadata)}, nil
}

// SetGenieHome sets the fallback discovery root and invalidates cached catalogs.
// A GenieHome in the operation's context takes precedence.
func (p *DefaultProvider) SetGenieHome(home string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.genieHome = home
	p.catalogs = make(map[string]map[string]*SkillMetadata)
}

func (p *DefaultProvider) ListSkills(ctx context.Context) ([]SkillMetadata, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	home := p.genieHome
	if h, ok := toolctx.GenieHome(ctx); ok && h != "" {
		home = h
	}
	catalog, ok := p.catalogs[home]
	if !ok {
		var err error
		catalog, err = p.discoverAllSkills(home)
		if err != nil {
			return nil, err
		}
		p.catalogs[home] = catalog
	}
	result := make([]SkillMetadata, 0, len(catalog))
	for _, m := range catalog {
		result = append(result, cloneMetadata(*m))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (p *DefaultProvider) LoadSkill(ctx context.Context, name string) (*Skill, error) {
	list, err := p.ListSkills(ctx)
	if err != nil {
		return nil, err
	}
	for _, metadata := range list {
		if metadata.Name == name {
			var skill *Skill
			if metadata.Source == SkillSourceInternal {
				skill, err = p.loadInternalSkill(name)
			} else {
				skill, err = p.loader.LoadSkillFile(metadata.FilePath, metadata.Source)
			}
			if err != nil {
				return nil, &SkillLoadError{Name: name, Cause: err}
			}
			skill.BaseDir = filepath.Dir(skill.FilePath)
			return skill, nil
		}
	}
	return nil, &SkillNotFoundError{Name: name}
}

func (p *DefaultProvider) ReadFile(ctx context.Context, name, resource string) ([]byte, error) {
	resource, err := cleanResourcePath(resource)
	if err != nil {
		return nil, err
	}
	skill, err := p.LoadSkill(ctx, name)
	if err != nil {
		return nil, err
	}
	if skill.Source == SkillSourceInternal {
		return internalSkillsFS.ReadFile(filepath.ToSlash(filepath.Join(skill.BaseDir, resource)))
	}
	content, err := readWithin(skill.BaseDir, resource)
	if err == nil {
		return content, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	// Preserve the default filesystem provider's workspace fallback. Custom
	// providers are never given an implicit fallback into the local filesystem.
	workingDir, _ := toolctx.WorkingDir(ctx)
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	content, err = readWithin(workingDir, resource)
	if err != nil {
		return nil, fmt.Errorf("file %q not found or unreadable in skill directory %s or working directory %s: %w", resource, skill.BaseDir, workingDir, err)
	}
	return content, nil
}

func readWithin(base, resource string) ([]byte, error) {
	full, err := filepath.EvalSymlinks(filepath.Join(base, filepath.FromSlash(resource)))
	if err != nil {
		return nil, err
	}
	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return nil, err
	}
	if !isPathWithinBase(full, resolvedBase) {
		return nil, fmt.Errorf("resource escapes skill directory: %w", fs.ErrPermission)
	}
	return os.ReadFile(full)
}

func (p *DefaultProvider) ListFiles(ctx context.Context, name string) ([]string, error) {
	skill, err := p.LoadSkill(ctx, name)
	if err != nil {
		return nil, err
	}
	var root fs.FS
	if skill.Source == SkillSourceInternal {
		root, err = fs.Sub(internalSkillsFS, filepath.ToSlash(skill.BaseDir))
		if err != nil {
			return nil, err
		}
	} else {
		root = os.DirFS(skill.BaseDir)
	}
	var files []string
	err = fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// discoverAllSkills preserves the established source precedence.
func (p *DefaultProvider) discoverAllSkills(home string) (map[string]*SkillMetadata, error) {
	catalog := make(map[string]*SkillMetadata)
	if err := p.discoverInternalSkills(catalog); err != nil {
		return nil, err
	}
	if err := p.discoverFromDirectory(catalog, filepath.Join(p.userHome, ".genie", "skills"), SkillSourceUser); err != nil {
		return nil, err
	}
	if home != "" {
		for _, dir := range []string{".genie", ".claude"} {
			if err := p.discoverFromDirectory(catalog, filepath.Join(home, dir, "skills"), SkillSourceProject); err != nil {
				return nil, err
			}
		}
	}
	return catalog, nil
}

// discoverInternalSkills discovers embedded internal skills
func (m *DefaultProvider) discoverInternalSkills(catalog map[string]*SkillMetadata) error {
	// List directories in internal/skills
	entries, err := internalSkillsFS.ReadDir("internal/skills")
	if err != nil {
		// Internal skills directory might not exist yet
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()

		// Skip test skills (skills starting with "test-")
		if strings.HasPrefix(skillName, "test-") {
			continue
		}

		skillPath := filepath.Join("internal/skills", skillName, "SKILL.md")

		// Read SKILL.md from embedded filesystem
		content, err := internalSkillsFS.ReadFile(skillPath)
		if err != nil {
			continue // Skip if SKILL.md doesn't exist
		}

		// Parse metadata
		metadata, _, err := m.loader.parseFrontmatter(content)
		if err != nil {
			continue // Skip invalid skills
		}

		// Validate metadata
		if err := m.loader.validateMetadata(metadata); err != nil {
			continue // Skip invalid skills
		}

		metadata.Source = SkillSourceInternal
		metadata.FilePath = skillPath

		// Add to cache (only if not already present from higher priority source)
		if _, exists := catalog[metadata.Name]; !exists {
			catalog[metadata.Name] = metadata
		}
	}

	return nil
}

// discoverFromDirectory discovers skills from a filesystem directory
func (m *DefaultProvider) discoverFromDirectory(catalog map[string]*SkillMetadata, dir string, source SkillSource) error {
	skillFiles, err := m.loader.DiscoverSkills(dir)
	if err != nil {
		return err
	}

	for _, filePath := range skillFiles {
		// Load metadata only
		metadata, err := m.loader.LoadMetadata(filePath, source)
		if err != nil {
			// Skip invalid skills but don't fail the whole discovery
			continue
		}

		// Add to cache (overwriting lower priority sources)
		catalog[metadata.Name] = metadata
	}

	return nil
}

// loadInternalSkill loads an internal skill from embedded filesystem
func (m *DefaultProvider) loadInternalSkill(name string) (*Skill, error) {
	skillPath := filepath.Join("internal/skills", name, "SKILL.md")

	content, err := internalSkillsFS.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read internal skill: %w", err)
	}

	metadata, skillContent, err := m.loader.parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse internal skill: %w", err)
	}

	if err := m.loader.validateMetadata(metadata); err != nil {
		return nil, err
	}

	metadata.Source = SkillSourceInternal
	metadata.FilePath = skillPath

	skill := &Skill{
		SkillMetadata: *metadata,
		Content:       skillContent,
		BaseDir:       filepath.Dir(skillPath),
		LoadedFiles:   make(map[string]string),
	}

	return skill, nil
}
