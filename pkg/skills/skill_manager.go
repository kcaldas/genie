package skills

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//go:embed internal/skills
var internalSkillsFS embed.FS

// DefaultSkillManager is the default implementation of SkillManager
type DefaultSkillManager struct {
	loader         *SkillLoader
	genieHome      string
	userHome       string
	skillsCache    map[string]*SkillMetadata // Cache of discovered skills
	activeSkills   map[string]*Skill         // Active skills per session ID
	mu             sync.RWMutex
	cacheMu        sync.RWMutex
	discoveryDone  bool
}

// NewDefaultSkillManager creates a new skill manager
func NewDefaultSkillManager() (*DefaultSkillManager, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	return &DefaultSkillManager{
		loader:       NewSkillLoader(),
		userHome:     userHome,
		skillsCache:  make(map[string]*SkillMetadata),
		activeSkills: make(map[string]*Skill),
	}, nil
}

// SetGenieHome sets the genie home directory for project-level skills
func (m *DefaultSkillManager) SetGenieHome(genieHome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.genieHome = genieHome
	m.discoveryDone = false // Invalidate cache
}

// ListSkills returns metadata for all available skills across all sources
func (m *DefaultSkillManager) ListSkills(ctx context.Context) ([]SkillMetadata, error) {
	if err := m.ensureDiscovery(); err != nil {
		return nil, err
	}

	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	skills := make([]SkillMetadata, 0, len(m.skillsCache))
	for _, metadata := range m.skillsCache {
		skills = append(skills, *metadata)
	}

	return skills, nil
}

// GetSkillMetadata returns metadata for a specific skill by name
func (m *DefaultSkillManager) GetSkillMetadata(ctx context.Context, name string) (*SkillMetadata, error) {
	if err := m.ensureDiscovery(); err != nil {
		return nil, err
	}

	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	metadata, exists := m.skillsCache[name]
	if !exists {
		return nil, &SkillNotFoundError{Name: name}
	}

	return metadata, nil
}

// LoadSkill loads the full content of a skill by name
func (m *DefaultSkillManager) LoadSkill(ctx context.Context, name string) (*Skill, error) {
	// Get metadata to find file path
	metadata, err := m.GetSkillMetadata(ctx, name)
	if err != nil {
		return nil, err
	}

	// Load skill from file
	var skill *Skill
	if metadata.Source == SkillSourceInternal {
		skill, err = m.loadInternalSkill(name)
	} else {
		skill, err = m.loader.LoadSkillFile(metadata.FilePath, metadata.Source)
	}

	if err != nil {
		return nil, &SkillLoadError{Name: name, Cause: err}
	}

	// Set BaseDir from the SKILL.md file path
	skill.BaseDir = filepath.Dir(skill.FilePath)

	// Initialize LoadedFiles map
	if skill.LoadedFiles == nil {
		skill.LoadedFiles = make(map[string]string)
	}

	return skill, nil
}

// LoadSkillFile loads an additional file from the active skill's directory into context
// The filePath should be relative to the skill's BaseDir
func (m *DefaultSkillManager) LoadSkillFile(ctx context.Context, filePath string) error {
	sessionID := m.getSessionID(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get active skill
	skill, exists := m.activeSkills[sessionID]
	if !exists {
		return fmt.Errorf("no active skill to load file into")
	}

	// Security: Clean the file path and ensure it's relative
	cleanPath := filepath.Clean(filePath)
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("file path must be relative to skill directory: %s", filePath)
	}

	// Security: Ensure the file is within the skill's BaseDir (no path traversal)
	fullPath := filepath.Join(skill.BaseDir, cleanPath)
	if !isPathWithinBase(fullPath, skill.BaseDir) {
		return fmt.Errorf("file path escapes skill directory: %s", filePath)
	}

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Add to loaded files
	if skill.LoadedFiles == nil {
		skill.LoadedFiles = make(map[string]string)
	}
	skill.LoadedFiles[cleanPath] = string(content)

	return nil
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

// GetActiveSkill returns the currently active skill for the current session
func (m *DefaultSkillManager) GetActiveSkill(ctx context.Context) (*Skill, error) {
	sessionID := m.getSessionID(ctx)

	m.mu.RLock()
	defer m.mu.RUnlock()

	skill, exists := m.activeSkills[sessionID]
	if !exists {
		return nil, nil // No active skill
	}

	return skill, nil
}

// SetActiveSkill sets the active skill for the current session
func (m *DefaultSkillManager) SetActiveSkill(ctx context.Context, skill *Skill) error {
	sessionID := m.getSessionID(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.activeSkills[sessionID] = skill
	return nil
}

// ClearActiveSkill removes the active skill from the current session
func (m *DefaultSkillManager) ClearActiveSkill(ctx context.Context) error {
	sessionID := m.getSessionID(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.activeSkills, sessionID)
	return nil
}

// ensureDiscovery ensures skills have been discovered and cached
func (m *DefaultSkillManager) ensureDiscovery() error {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	if m.discoveryDone {
		return nil
	}

	// Discover from all sources
	if err := m.discoverAllSkills(); err != nil {
		return err
	}

	m.discoveryDone = true
	return nil
}

// discoverAllSkills discovers skills from all sources with proper priority
func (m *DefaultSkillManager) discoverAllSkills() error {
	m.skillsCache = make(map[string]*SkillMetadata)

	// 1. Discover internal skills (lowest priority)
	if err := m.discoverInternalSkills(); err != nil {
		return fmt.Errorf("failed to discover internal skills: %w", err)
	}

	// 2. Discover user skills (medium priority)
	if err := m.discoverUserSkills(); err != nil {
		return fmt.Errorf("failed to discover user skills: %w", err)
	}

	// 3. Discover project skills (highest priority) from both .genie and .claude
	if err := m.discoverProjectSkills(); err != nil {
		return fmt.Errorf("failed to discover project skills: %w", err)
	}

	return nil
}

// discoverProjectSkills discovers skills from project directories (.genie/skills and .claude/skills)
func (m *DefaultSkillManager) discoverProjectSkills() error {
	if m.genieHome == "" {
		return nil
	}

	// Try .genie/skills first
	genieSkillsDir := filepath.Join(m.genieHome, ".genie", "skills")
	if err := m.discoverFromDirectory(genieSkillsDir, SkillSourceProject); err != nil {
		return err
	}

	// Try .claude/skills for compatibility
	claudeSkillsDir := filepath.Join(m.genieHome, ".claude", "skills")
	if err := m.discoverFromDirectory(claudeSkillsDir, SkillSourceProject); err != nil {
		return err
	}

	return nil
}

// discoverUserSkills discovers skills from user's home directory
func (m *DefaultSkillManager) discoverUserSkills() error {
	userSkillsDir := filepath.Join(m.userHome, ".genie", "skills")
	return m.discoverFromDirectory(userSkillsDir, SkillSourceUser)
}

// discoverInternalSkills discovers embedded internal skills
func (m *DefaultSkillManager) discoverInternalSkills() error {
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
		if _, exists := m.skillsCache[metadata.Name]; !exists {
			m.skillsCache[metadata.Name] = metadata
		}
	}

	return nil
}

// discoverFromDirectory discovers skills from a filesystem directory
func (m *DefaultSkillManager) discoverFromDirectory(dir string, source SkillSource) error {
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
		m.skillsCache[metadata.Name] = metadata
	}

	return nil
}

// loadInternalSkill loads an internal skill from embedded filesystem
func (m *DefaultSkillManager) loadInternalSkill(name string) (*Skill, error) {
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

// getSessionID extracts session ID from context
func (m *DefaultSkillManager) getSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}
	return "default" // Fallback for contexts without session ID
}
