package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kcaldas/genie/pkg/toolctx"
)

// newTestManager creates a skill manager whose user home and project
// (genie home) point at temp directories. It returns the manager plus the
// user home and project root paths.
func newTestManager(t *testing.T) (*DefaultSkillManager, string, string) {
	t.Helper()
	userHome := t.TempDir()
	projectRoot := t.TempDir()

	t.Setenv("HOME", userHome)
	manager, err := NewDefaultSkillManager()
	if err != nil {
		t.Fatalf("NewDefaultSkillManager returned error: %v", err)
	}
	manager.SetGenieHome(projectRoot)

	return manager, userHome, projectRoot
}

func skillNames(skills []SkillMetadata) map[string]SkillMetadata {
	byName := make(map[string]SkillMetadata, len(skills))
	for _, s := range skills {
		byName[s.Name] = s
	}
	return byName
}

func TestListSkillsDiscoversAllSources(t *testing.T) {
	manager, userHome, projectRoot := newTestManager(t)

	writeSkillDir(t, filepath.Join(projectRoot, ".genie", "skills"), "project-skill", "project-skill", "From project", "# P")
	writeSkillDir(t, filepath.Join(projectRoot, ".claude", "skills"), "claude-skill", "claude-skill", "From claude dir", "# C")
	writeSkillDir(t, filepath.Join(userHome, ".genie", "skills"), "user-skill", "user-skill", "From user home", "# U")

	skills, err := manager.ListSkills(context.Background())
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	byName := skillNames(skills)

	if got := byName["project-skill"]; got.Source != SkillSourceProject {
		t.Errorf("project-skill source = %q, want %q", got.Source, SkillSourceProject)
	}
	if got := byName["claude-skill"]; got.Source != SkillSourceProject {
		t.Errorf("claude-skill source = %q, want %q (found via .claude/skills)", got.Source, SkillSourceProject)
	}
	if got := byName["user-skill"]; got.Source != SkillSourceUser {
		t.Errorf("user-skill source = %q, want %q", got.Source, SkillSourceUser)
	}

	// Built-in skills are embedded in the binary
	if got, ok := byName["skill-creator"]; !ok {
		t.Error("built-in skill-creator not discovered")
	} else if got.Source != SkillSourceInternal {
		t.Errorf("skill-creator source = %q, want %q", got.Source, SkillSourceInternal)
	}

	// Internal skills prefixed with "test-" are excluded from discovery
	for name := range byName {
		if strings.HasPrefix(name, "test-") && byName[name].Source == SkillSourceInternal {
			t.Errorf("internal test skill %q should not be discovered", name)
		}
	}
}

func TestProjectSkillTakesPriorityOverUser(t *testing.T) {
	manager, userHome, projectRoot := newTestManager(t)

	writeSkillDir(t, filepath.Join(userHome, ".genie", "skills"), "shared", "shared", "User version", "# User")
	projectSkillDir := writeSkillDir(t, filepath.Join(projectRoot, ".genie", "skills"), "shared", "shared", "Project version", "# Project")

	metadata, err := manager.GetSkillMetadata(context.Background(), "shared")
	if err != nil {
		t.Fatalf("GetSkillMetadata returned error: %v", err)
	}

	if metadata.Source != SkillSourceProject {
		t.Errorf("Source = %q, want %q (project must win over user)", metadata.Source, SkillSourceProject)
	}
	if metadata.Description != "Project version" {
		t.Errorf("Description = %q, want %q", metadata.Description, "Project version")
	}
	if metadata.FilePath != filepath.Join(projectSkillDir, "SKILL.md") {
		t.Errorf("FilePath = %q, want project path %q", metadata.FilePath, filepath.Join(projectSkillDir, "SKILL.md"))
	}
}

func TestUserSkillTakesPriorityOverInternal(t *testing.T) {
	manager, userHome, _ := newTestManager(t)

	// Shadow the built-in skill-creator with a user-level skill
	writeSkillDir(t, filepath.Join(userHome, ".genie", "skills"), "skill-creator", "skill-creator", "User override", "# Override")

	metadata, err := manager.GetSkillMetadata(context.Background(), "skill-creator")
	if err != nil {
		t.Fatalf("GetSkillMetadata returned error: %v", err)
	}

	if metadata.Source != SkillSourceUser {
		t.Errorf("Source = %q, want %q (user must win over internal)", metadata.Source, SkillSourceUser)
	}
	if metadata.Description != "User override" {
		t.Errorf("Description = %q, want %q", metadata.Description, "User override")
	}
}

func TestGenieHomeFromContextInvalidatesDiscovery(t *testing.T) {
	manager, _, _ := newTestManager(t)

	otherProject := t.TempDir()
	writeSkillDir(t, filepath.Join(otherProject, ".genie", "skills"), "other-skill", "other-skill", "From other project", "# O")

	// First discovery with the original genie home: skill absent
	if _, err := manager.GetSkillMetadata(context.Background(), "other-skill"); err == nil {
		t.Fatal("expected other-skill to be missing before genie home switch")
	}

	// Passing a different genie_home via context must trigger rediscovery
	ctx := toolctx.WithGenieHome(context.Background(), otherProject)
	metadata, err := manager.GetSkillMetadata(ctx, "other-skill")
	if err != nil {
		t.Fatalf("GetSkillMetadata after genie home switch returned error: %v", err)
	}
	if metadata.Source != SkillSourceProject {
		t.Errorf("Source = %q, want %q", metadata.Source, SkillSourceProject)
	}

	// ListSkills honors the same context override
	skills, err := manager.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	if _, ok := skillNames(skills)["other-skill"]; !ok {
		t.Error("ListSkills should include skills from the context genie_home")
	}
}

func TestGetSkillMetadataNotFound(t *testing.T) {
	manager, _, _ := newTestManager(t)

	_, err := manager.GetSkillMetadata(context.Background(), "no-such-skill")
	if err == nil {
		t.Fatal("expected error for unknown skill, got nil")
	}

	var notFound *SkillNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("error type = %T, want *SkillNotFoundError", err)
	}
	if notFound.Name != "no-such-skill" {
		t.Errorf("Name = %q, want %q", notFound.Name, "no-such-skill")
	}
	if !strings.Contains(err.Error(), "no-such-skill") {
		t.Errorf("error message should name the skill: %v", err)
	}
}

func TestDiscoverySkipsInvalidSkills(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)

	skillsDir := filepath.Join(projectRoot, ".genie", "skills")
	writeSkillDir(t, skillsDir, "good-skill", "good-skill", "Valid skill", "# Good")
	// Malformed sibling must be skipped without failing discovery
	writeFile(t, filepath.Join(skillsDir, "broken", "SKILL.md"), "no frontmatter here")

	skills, err := manager.ListSkills(context.Background())
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	byName := skillNames(skills)

	if _, ok := byName["good-skill"]; !ok {
		t.Error("good-skill should be discovered despite broken sibling")
	}
	if _, ok := byName["broken"]; ok {
		t.Error("broken skill should be skipped")
	}
}

func TestLoadSkillFromFilesystem(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)

	skillDir := writeSkillDir(t, filepath.Join(projectRoot, ".genie", "skills"), "loadable", "loadable", "Loads fully", "# Loadable\n\nBody text.")

	skill, err := manager.LoadSkill(context.Background(), "loadable")
	if err != nil {
		t.Fatalf("LoadSkill returned error: %v", err)
	}

	if skill.Name != "loadable" {
		t.Errorf("Name = %q, want %q", skill.Name, "loadable")
	}
	if !strings.Contains(skill.Content, "Body text.") {
		t.Errorf("Content missing body: %q", skill.Content)
	}
	if skill.BaseDir != skillDir {
		t.Errorf("BaseDir = %q, want %q", skill.BaseDir, skillDir)
	}
	if skill.LoadedFiles == nil {
		t.Error("LoadedFiles must be initialized")
	}
}

func TestLoadSkillInternal(t *testing.T) {
	manager, _, _ := newTestManager(t)

	skill, err := manager.LoadSkill(context.Background(), "skill-creator")
	if err != nil {
		t.Fatalf("LoadSkill(skill-creator) returned error: %v", err)
	}

	if skill.Source != SkillSourceInternal {
		t.Errorf("Source = %q, want %q", skill.Source, SkillSourceInternal)
	}
	if skill.Content == "" {
		t.Error("internal skill content should not be empty")
	}
	if skill.BaseDir == "" {
		t.Error("internal skill BaseDir should be set")
	}
}

func TestLoadSkillUnknown(t *testing.T) {
	manager, _, _ := newTestManager(t)

	_, err := manager.LoadSkill(context.Background(), "ghost")
	var notFound *SkillNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("error type = %T, want *SkillNotFoundError", err)
	}
}

func TestLoadSkillFileRemovedAfterDiscovery(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)

	skillDir := writeSkillDir(t, filepath.Join(projectRoot, ".genie", "skills"), "vanishing", "vanishing", "Will disappear", "# V")

	// Discover first, then delete the file behind the cache's back
	if _, err := manager.GetSkillMetadata(context.Background(), "vanishing"); err != nil {
		t.Fatalf("GetSkillMetadata returned error: %v", err)
	}
	if err := os.Remove(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	_, err := manager.LoadSkill(context.Background(), "vanishing")
	if err == nil {
		t.Fatal("expected error loading removed skill, got nil")
	}
	var loadErr *SkillLoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("error type = %T, want *SkillLoadError", err)
	}
	if loadErr.Name != "vanishing" {
		t.Errorf("Name = %q, want %q", loadErr.Name, "vanishing")
	}
	if loadErr.Cause == nil {
		t.Error("Cause should carry the underlying error")
	}
}

func TestActiveSkillLifecycle(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)
	writeSkillDir(t, filepath.Join(projectRoot, ".genie", "skills"), "life", "life", "Lifecycle skill", "# L")

	ctx := context.Background()

	// No active skill initially
	active, err := manager.GetActiveSkill(ctx)
	if err != nil {
		t.Fatalf("GetActiveSkill returned error: %v", err)
	}
	if active != nil {
		t.Fatalf("expected no active skill, got %v", active)
	}

	skill, err := manager.LoadSkill(ctx, "life")
	if err != nil {
		t.Fatalf("LoadSkill returned error: %v", err)
	}

	if err := manager.SetActiveSkill(ctx, skill); err != nil {
		t.Fatalf("SetActiveSkill returned error: %v", err)
	}
	active, err = manager.GetActiveSkill(ctx)
	if err != nil {
		t.Fatalf("GetActiveSkill returned error: %v", err)
	}
	if active == nil || active.Name != "life" {
		t.Fatalf("active skill = %v, want life", active)
	}

	if err := manager.ClearActiveSkill(ctx); err != nil {
		t.Fatalf("ClearActiveSkill returned error: %v", err)
	}
	active, err = manager.GetActiveSkill(ctx)
	if err != nil {
		t.Fatalf("GetActiveSkill returned error: %v", err)
	}
	if active != nil {
		t.Errorf("expected active skill cleared, got %v", active)
	}
}

func TestActiveSkillIsPerSession(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)
	writeSkillDir(t, filepath.Join(projectRoot, ".genie", "skills"), "session-skill", "session-skill", "Session bound", "# S")

	sessionA := toolctx.WithSessionID(context.Background(), "session-a")
	sessionB := toolctx.WithSessionID(context.Background(), "session-b")

	skill, err := manager.LoadSkill(sessionA, "session-skill")
	if err != nil {
		t.Fatalf("LoadSkill returned error: %v", err)
	}
	if err := manager.SetActiveSkill(sessionA, skill); err != nil {
		t.Fatalf("SetActiveSkill returned error: %v", err)
	}

	activeA, _ := manager.GetActiveSkill(sessionA)
	if activeA == nil || activeA.Name != "session-skill" {
		t.Errorf("session A active skill = %v, want session-skill", activeA)
	}
	activeB, _ := manager.GetActiveSkill(sessionB)
	if activeB != nil {
		t.Errorf("session B should have no active skill, got %v", activeB)
	}
}

func TestLoadSkillFileRequiresActiveSkill(t *testing.T) {
	manager, _, _ := newTestManager(t)

	err := manager.LoadSkillFile(context.Background(), "extra.md")
	if err == nil {
		t.Fatal("expected error without an active skill, got nil")
	}
	if !strings.Contains(err.Error(), "no active skill") {
		t.Errorf("error should explain no skill is active: %v", err)
	}
}

// activateSkill loads and activates a project skill named "host" and
// returns its directory.
func activateSkill(t *testing.T, manager *DefaultSkillManager, ctx context.Context, projectRoot string) (*Skill, string) {
	t.Helper()
	skillDir := writeSkillDir(t, filepath.Join(projectRoot, ".genie", "skills"), "host", "host", "Hosts extra files", "# Host")

	skill, err := manager.LoadSkill(ctx, "host")
	if err != nil {
		t.Fatalf("LoadSkill returned error: %v", err)
	}
	if err := manager.SetActiveSkill(ctx, skill); err != nil {
		t.Fatalf("SetActiveSkill returned error: %v", err)
	}
	return skill, skillDir
}

func TestLoadSkillFileFromSkillDirectory(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)
	ctx := context.Background()
	skill, skillDir := activateSkill(t, manager, ctx, projectRoot)

	writeFile(t, filepath.Join(skillDir, "docs", "extra.md"), "extra content")

	if err := manager.LoadSkillFile(ctx, "docs/extra.md"); err != nil {
		t.Fatalf("LoadSkillFile returned error: %v", err)
	}

	content, ok := skill.LoadedFiles[filepath.Join("docs", "extra.md")]
	if !ok {
		t.Fatalf("docs/extra.md not in LoadedFiles: %v", skill.LoadedFiles)
	}
	if content != "extra content" {
		t.Errorf("loaded content = %q, want %q", content, "extra content")
	}
}

func TestLoadSkillFileFallsBackToWorkingDirectory(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)

	workingDir := t.TempDir()
	ctx := toolctx.WithWorkingDir(context.Background(), workingDir)
	skill, _ := activateSkill(t, manager, ctx, projectRoot)

	writeFile(t, filepath.Join(workingDir, "notes.txt"), "from cwd")

	if err := manager.LoadSkillFile(ctx, "notes.txt"); err != nil {
		t.Fatalf("LoadSkillFile returned error: %v", err)
	}
	if skill.LoadedFiles["notes.txt"] != "from cwd" {
		t.Errorf("loaded content = %q, want %q", skill.LoadedFiles["notes.txt"], "from cwd")
	}
}

func TestLoadSkillFileRejectsUnsafePaths(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)
	ctx := toolctx.WithWorkingDir(context.Background(), t.TempDir())
	activateSkill(t, manager, ctx, projectRoot)

	t.Run("absolute path", func(t *testing.T) {
		err := manager.LoadSkillFile(ctx, "/etc/passwd")
		if err == nil || !strings.Contains(err.Error(), "must be relative") {
			t.Errorf("expected relative-path error, got: %v", err)
		}
	})

	t.Run("path traversal", func(t *testing.T) {
		err := manager.LoadSkillFile(ctx, "../secrets.txt")
		if err == nil || !strings.Contains(err.Error(), "cannot start with ..") {
			t.Errorf("expected traversal rejection, got: %v", err)
		}
	})
}

func TestLoadSkillFileNotFoundListsSearchedLocations(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)

	workingDir := t.TempDir()
	ctx := toolctx.WithWorkingDir(context.Background(), workingDir)
	skill, _ := activateSkill(t, manager, ctx, projectRoot)

	err := manager.LoadSkillFile(ctx, "missing.md")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), skill.BaseDir) {
		t.Errorf("error should mention skill directory %q: %v", skill.BaseDir, err)
	}
	if !strings.Contains(err.Error(), workingDir) {
		t.Errorf("error should mention working directory %q: %v", workingDir, err)
	}
}

func TestManagerConcurrentAccessIsRaceFree(t *testing.T) {
	manager, _, projectRoot := newTestManager(t)
	writeSkillDir(t, filepath.Join(projectRoot, ".genie", "skills"), "racer", "racer", "Raced skill", "# R")

	ctx := context.Background()
	skill, err := manager.LoadSkill(ctx, "racer")
	if err != nil {
		t.Fatalf("LoadSkill returned error: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = manager.SetActiveSkill(ctx, skill)
				_ = manager.ClearActiveSkill(ctx)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = manager.GetActiveSkill(ctx)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = manager.ListSkills(ctx)
			}
		}()
	}
	wg.Wait()
}
