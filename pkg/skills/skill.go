package skills

import (
	"context"
	"fmt"
)

// SkillSource indicates where a skill was loaded from
type SkillSource string

const (
	// SkillSourceInternal indicates a built-in skill embedded in the binary
	SkillSourceInternal SkillSource = "internal"
	// SkillSourceProject indicates a skill from the project's .genie/skills or .claude/skills directory
	SkillSourceProject SkillSource = "project"
	// SkillSourceUser indicates a skill from the user's ~/.genie/skills directory
	SkillSourceUser SkillSource = "user"
)

// SkillMetadata contains the basic information about a skill without its full content.
// This is used for discovery and presenting available skills to the AI.
type SkillMetadata struct {
	Name        string      `yaml:"name"`        // Unique identifier for the skill
	Description string      `yaml:"description"` // What the skill does and when to use it
	Source      SkillSource `yaml:"-"`           // Where the skill was loaded from
	FilePath    string      `yaml:"-"`           // Path to the SKILL.md file
}

// Skill represents a fully loaded skill with its content
type Skill struct {
	SkillMetadata
	Content     string            // Full SKILL.md content (without frontmatter)
	BaseDir     string            // Absolute path to directory containing SKILL.md
	LoadedFiles map[string]string // Maps relative file paths to their content
}

// String returns a human-readable representation of the skill
func (s *Skill) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.Source)
}

// SkillManager manages the lifecycle of skills
type SkillManager interface {
	// ListSkills returns metadata for all available skills across all sources
	ListSkills(ctx context.Context) ([]SkillMetadata, error)

	// GetSkillMetadata returns metadata for a specific skill by name
	GetSkillMetadata(ctx context.Context, name string) (*SkillMetadata, error)

	// LoadSkill loads the full content of a skill by name
	LoadSkill(ctx context.Context, name string) (*Skill, error)

	// LoadSkillFile loads an additional file from the active skill's directory into context
	// The filePath should be relative to the skill's BaseDir
	LoadSkillFile(ctx context.Context, filePath string) error

	// GetActiveSkill returns the currently active skill, if any
	GetActiveSkill(ctx context.Context) (*Skill, error)

	// SetActiveSkill sets the active skill for the current session
	SetActiveSkill(ctx context.Context, skill *Skill) error

	// ClearActiveSkill removes the active skill from the current session
	ClearActiveSkill(ctx context.Context) error
}

// SkillNotFoundError is returned when a requested skill cannot be found
type SkillNotFoundError struct {
	Name string
}

func (e *SkillNotFoundError) Error() string {
	return fmt.Sprintf("skill not found: %s", e.Name)
}

// SkillLoadError is returned when a skill file cannot be loaded or parsed
type SkillLoadError struct {
	Name  string
	Cause error
}

func (e *SkillLoadError) Error() string {
	return fmt.Sprintf("failed to load skill %s: %v", e.Name, e.Cause)
}
