package genie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The skills system loads SKILL.md content into the "active_skill"
// context part — it must actually reach the model. Regression: it was
// assembled by the skill provider and then silently dropped because
// nothing lifted it into the prompt.
func TestBuildSystemContextIncludesActiveSkill(t *testing.T) {
	promptData := map[string]string{
		"project":      "project facts",
		"files":        "file contents",
		"active_skill": "# Active Skill: pdf-builder\ninstructions here",
		"message":      "hello",
	}

	files, userCtx := buildSystemContext(promptData, "host memory")

	assert.Equal(t, "file contents", files)
	assert.Contains(t, userCtx, "project facts")
	assert.Contains(t, userCtx, "pdf-builder", "active skill content must reach the model's system context")
	assert.Contains(t, userCtx, "instructions here")
	assert.Contains(t, userCtx, "host memory")

	// The lifted parts must not also flow through the template data.
	assert.NotContains(t, promptData, "files")
	assert.NotContains(t, promptData, "project")
	assert.NotContains(t, promptData, "active_skill")
	assert.Contains(t, promptData, "message")
}

func TestBuildSystemContextEmptyParts(t *testing.T) {
	promptData := map[string]string{"message": "hello"}

	files, userCtx := buildSystemContext(promptData, "")

	assert.Empty(t, files)
	assert.Empty(t, userCtx)
}
