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
	Name        string            `yaml:"name"`               // Unique identifier for the skill
	Description string            `yaml:"description"`        // What the skill does and when to use it
	Source      SkillSource       `yaml:"-"`                  // Where the skill was loaded from
	FilePath    string            `yaml:"-"`                  // Path to the SKILL.md file
	Metadata    map[string]string `yaml:"metadata,omitempty"` // Host-defined Agent Skills metadata; Genie does not interpret it.
}

// Provider supplies skill definitions and resources, independently of session
// state. Implementations must be safe for concurrent use and enforce their
// availability policy on every operation, using the supplied context. Resource
// paths are relative to the skill and use forward slashes. A custom provider
// replaces default discovery; composition and precedence belong to the host.
type Provider interface {
	ListSkills(context.Context) ([]SkillMetadata, error)
	LoadSkill(context.Context, string) (*Skill, error)
	ReadFile(ctx context.Context, name, path string) ([]byte, error)
	ListFiles(ctx context.Context, name string) ([]string, error)
}

// Skill represents a fully loaded skill with its content
type Skill struct {
	SkillMetadata
	Content     string            // Full SKILL.md content (without frontmatter)
	BaseDir     string            // Skill directory, if local; otherwise a provider-defined display location.
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

	// LoadSkillFile loads an additional file from an active skill's directory
	// into that skill's context. skillName selects which active skill; empty
	// means the most recently loaded one. filePath is relative to the skill.
	LoadSkillFile(ctx context.Context, skillName, filePath string) error

	// ReadSkillFile reads a skill resource through the provider without
	// touching session state, returning the cleaned resource path (the
	// Skill.LoadedFiles key) and the content.
	ReadSkillFile(ctx context.Context, skillName, filePath string) (resource, content string, err error)

	// ListSkillFiles lists resources through the same provider used for loading.
	ListSkillFiles(ctx context.Context, name string) ([]string, error)

	// GetActiveSkills returns the session's active skills in load order.
	GetActiveSkills(ctx context.Context) ([]*Skill, error)

	// GetActiveSkill returns the most recently loaded active skill, if any.
	GetActiveSkill(ctx context.Context) (*Skill, error)

	// ActivateSkill adds a skill to the session's active set; an already
	// active skill is replaced in place. When the session cap is exceeded
	// the oldest active skill is evicted and its name returned.
	ActivateSkill(ctx context.Context, skill *Skill) (evicted string, err error)

	// MaxActiveSkills returns the per-session cap on active skills.
	MaxActiveSkills() int

	// DeactivateSkill removes one skill from the session's active set and
	// reports whether it was active.
	DeactivateSkill(ctx context.Context, name string) (bool, error)

	// ClearActiveSkills removes every active skill from the current session.
	ClearActiveSkills(ctx context.Context) error

	// ClearAllActiveSkills resets this manager's session state.
	ClearAllActiveSkills()
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

func (e *SkillLoadError) Unwrap() error { return e.Cause }
