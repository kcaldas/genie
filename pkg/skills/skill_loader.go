package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillLoader handles loading and parsing SKILL.md files
type SkillLoader struct{}

// NewSkillLoader creates a new skill loader
func NewSkillLoader() *SkillLoader {
	return &SkillLoader{}
}

// LoadSkillFile loads and parses a SKILL.md file
func (l *SkillLoader) LoadSkillFile(filePath string, source SkillSource) (*Skill, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	// Parse frontmatter and content
	metadata, skillContent, err := l.parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill file: %w", err)
	}

	// Validate required fields
	if err := l.validateMetadata(metadata); err != nil {
		return nil, err
	}

	// Set source and file path
	metadata.Source = source
	metadata.FilePath = filePath

	return &Skill{
		SkillMetadata: *metadata,
		Content:       skillContent,
	}, nil
}

// LoadMetadata loads only the metadata from a SKILL.md file without parsing the full content
func (l *SkillLoader) LoadMetadata(filePath string, source SkillSource) (*SkillMetadata, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	// Parse frontmatter only
	metadata, _, err := l.parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill metadata: %w", err)
	}

	// Validate required fields
	if err := l.validateMetadata(metadata); err != nil {
		return nil, err
	}

	// Set source and file path
	metadata.Source = source
	metadata.FilePath = filePath

	return metadata, nil
}

// parseFrontmatter extracts YAML frontmatter and content from a markdown file
// Expected format:
// ---
// name: skill-name
// description: What it does
// ---
// # Skill content here
func (l *SkillLoader) parseFrontmatter(content []byte) (*SkillMetadata, string, error) {
	// Check for frontmatter delimiter
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return nil, "", fmt.Errorf("skill file must start with YAML frontmatter (---)")
	}

	// Find the end of frontmatter
	lines := bytes.Split(content, []byte("\n"))
	var frontmatterEnd int
	for i := 1; i < len(lines); i++ {
		line := bytes.TrimSpace(lines[i])
		if bytes.Equal(line, []byte("---")) {
			frontmatterEnd = i
			break
		}
	}

	if frontmatterEnd == 0 {
		return nil, "", fmt.Errorf("skill file missing closing frontmatter delimiter (---)")
	}

	// Extract frontmatter content (skip first and last ---)
	frontmatterContent := bytes.Join(lines[1:frontmatterEnd], []byte("\n"))

	// Parse YAML frontmatter
	var metadata SkillMetadata
	if err := yaml.Unmarshal(frontmatterContent, &metadata); err != nil {
		return nil, "", fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	// Extract skill content (everything after the closing ---)
	skillContent := bytes.Join(lines[frontmatterEnd+1:], []byte("\n"))

	return &metadata, strings.TrimSpace(string(skillContent)), nil
}

// validateMetadata ensures required fields are present
func (l *SkillLoader) validateMetadata(metadata *SkillMetadata) error {
	if metadata.Name == "" {
		return fmt.Errorf("skill metadata missing required field: name")
	}

	if metadata.Description == "" {
		return fmt.Errorf("skill metadata missing required field: description")
	}

	// Validate name format (lowercase alphanumeric with hyphens)
	if !isValidSkillName(metadata.Name) {
		return fmt.Errorf("invalid skill name '%s': must be lowercase alphanumeric with hyphens", metadata.Name)
	}

	return nil
}

// isValidSkillName checks if a skill name follows the required format
func isValidSkillName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}

	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}

	return true
}

// DiscoverSkills finds all SKILL.md files in a directory
// Returns a map of skill name -> file path
func (l *SkillLoader) DiscoverSkills(baseDir string) (map[string]string, error) {
	skills := make(map[string]string)

	// Check if directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return skills, nil // Return empty map if directory doesn't exist
	}

	// Read directory entries
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	// Look for SKILL.md in each subdirectory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillFile := filepath.Join(baseDir, skillName, "SKILL.md")

		// Check if SKILL.md exists
		if _, err := os.Stat(skillFile); err == nil {
			skills[skillName] = skillFile
		}
	}

	return skills, nil
}
