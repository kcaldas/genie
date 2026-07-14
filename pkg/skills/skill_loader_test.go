package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkillDir creates a skill directory containing a SKILL.md with the
// given frontmatter name/description and markdown body. It returns the
// skill directory path.
func writeSkillDir(t *testing.T, baseDir, dirName, name, description, body string) string {
	t.Helper()
	skillDir := filepath.Join(baseDir, dirName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s\n", name, description, body)
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), content)
	return skillDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func TestLoadSkillFileParsesFrontmatterAndContent(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeSkillDir(t, dir, "my-skill", "my-skill", "Does useful things", "# My Skill\n\nInstructions here.")

	loader := NewSkillLoader()
	skill, err := loader.LoadSkillFile(filepath.Join(skillDir, "SKILL.md"), SkillSourceProject)
	if err != nil {
		t.Fatalf("LoadSkillFile returned error: %v", err)
	}

	if skill.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", skill.Name, "my-skill")
	}
	if skill.Description != "Does useful things" {
		t.Errorf("Description = %q, want %q", skill.Description, "Does useful things")
	}
	if skill.Source != SkillSourceProject {
		t.Errorf("Source = %q, want %q", skill.Source, SkillSourceProject)
	}
	if skill.FilePath != filepath.Join(skillDir, "SKILL.md") {
		t.Errorf("FilePath = %q, want %q", skill.FilePath, filepath.Join(skillDir, "SKILL.md"))
	}
	if !strings.Contains(skill.Content, "# My Skill") || !strings.Contains(skill.Content, "Instructions here.") {
		t.Errorf("Content missing markdown body: %q", skill.Content)
	}
	if strings.Contains(skill.Content, "name: my-skill") {
		t.Errorf("Content should not include frontmatter: %q", skill.Content)
	}
}

func TestLoadSkillFileMissingFile(t *testing.T) {
	loader := NewSkillLoader()
	_, err := loader.LoadSkillFile(filepath.Join(t.TempDir(), "nope", "SKILL.md"), SkillSourceProject)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read skill file") {
		t.Errorf("error should mention read failure, got: %v", err)
	}
}

func TestLoadSkillFileMalformedContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "no frontmatter",
			content: "# Just Markdown\n\nNo frontmatter at all.\n",
			wantErr: "must start with YAML frontmatter",
		},
		{
			name:    "missing closing delimiter",
			content: "---\nname: broken\ndescription: never closed\n\n# Content\n",
			wantErr: "missing closing frontmatter delimiter",
		},
		{
			name:    "invalid yaml",
			content: "---\nname: [unclosed\n---\n# Content\n",
			wantErr: "failed to parse YAML frontmatter",
		},
		{
			name:    "missing name",
			content: "---\ndescription: has no name\n---\n# Content\n",
			wantErr: "missing required field: name",
		},
		{
			name:    "missing description",
			content: "---\nname: no-description\n---\n# Content\n",
			wantErr: "missing required field: description",
		},
		{
			name:    "invalid name format",
			content: "---\nname: Bad_Name\ndescription: invalid characters\n---\n# Content\n",
			wantErr: "invalid skill name",
		},
	}

	loader := NewSkillLoader()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "SKILL.md")
			writeFile(t, path, tt.content)

			_, err := loader.LoadSkillFile(path, SkillSourceProject)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantErr)
			}

			// LoadMetadata must reject the same malformed input
			_, err = loader.LoadMetadata(path, SkillSourceProject)
			if err == nil {
				t.Fatal("LoadMetadata: expected error, got nil")
			}
		})
	}
}

func TestLoadMetadataReturnsMetadataOnly(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeSkillDir(t, dir, "meta-skill", "meta-skill", "Metadata only", "# Body")

	loader := NewSkillLoader()
	metadata, err := loader.LoadMetadata(filepath.Join(skillDir, "SKILL.md"), SkillSourceUser)
	if err != nil {
		t.Fatalf("LoadMetadata returned error: %v", err)
	}

	if metadata.Name != "meta-skill" {
		t.Errorf("Name = %q, want %q", metadata.Name, "meta-skill")
	}
	if metadata.Description != "Metadata only" {
		t.Errorf("Description = %q, want %q", metadata.Description, "Metadata only")
	}
	if metadata.Source != SkillSourceUser {
		t.Errorf("Source = %q, want %q", metadata.Source, SkillSourceUser)
	}
}

func TestIsValidSkillName(t *testing.T) {
	valid := []string{"a", "skill", "my-skill", "skill-2", "abc-123-def"}
	for _, name := range valid {
		if !isValidSkillName(name) {
			t.Errorf("isValidSkillName(%q) = false, want true", name)
		}
	}

	invalid := []string{"", "UPPER", "has space", "under_score", "dot.name", strings.Repeat("a", 65)}
	for _, name := range invalid {
		if isValidSkillName(name) {
			t.Errorf("isValidSkillName(%q) = true, want false", name)
		}
	}
}

func TestDiscoverSkills(t *testing.T) {
	loader := NewSkillLoader()

	t.Run("nonexistent directory returns empty map", func(t *testing.T) {
		skills, err := loader.DiscoverSkills(filepath.Join(t.TempDir(), "does-not-exist"))
		if err != nil {
			t.Fatalf("DiscoverSkills returned error: %v", err)
		}
		if len(skills) != 0 {
			t.Errorf("expected empty map, got %v", skills)
		}
	})

	t.Run("finds SKILL.md in subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		writeSkillDir(t, dir, "alpha", "alpha", "First skill", "# A")
		writeSkillDir(t, dir, "beta", "beta", "Second skill", "# B")

		// A directory without SKILL.md must be ignored
		if err := os.MkdirAll(filepath.Join(dir, "empty-dir"), 0o755); err != nil {
			t.Fatal(err)
		}
		// A plain file at the top level must be ignored
		writeFile(t, filepath.Join(dir, "stray.md"), "not a skill")

		skills, err := loader.DiscoverSkills(dir)
		if err != nil {
			t.Fatalf("DiscoverSkills returned error: %v", err)
		}
		if len(skills) != 2 {
			t.Fatalf("expected 2 skills, got %d: %v", len(skills), skills)
		}
		if skills["alpha"] != filepath.Join(dir, "alpha", "SKILL.md") {
			t.Errorf("alpha path = %q", skills["alpha"])
		}
		if skills["beta"] != filepath.Join(dir, "beta", "SKILL.md") {
			t.Errorf("beta path = %q", skills["beta"])
		}
	})
}
