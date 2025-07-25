package tools

import (
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestAllTools_HaveUniqueNames(t *testing.T) {
	publisher := &events.NoOpPublisher{}
	tools := []Tool{
		NewLsTool(publisher),
		NewFindTool(publisher),
		NewReadFileTool(publisher),
		NewGrepTool(publisher),
		NewGitStatusTool(),
		NewBashTool(nil, nil, false),
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		name := tool.Declaration().Name
		assert.False(t, names[name], "Tool name '%s' is not unique", name)
		names[name] = true
	}

	// Verify we have all expected tools
	expectedNames := []string{
		"listFiles",
		"findFiles", 
		"readFile",
		"searchInFiles",
		"gitStatus",
		"bash",
	}

	for _, expectedName := range expectedNames {
		assert.True(t, names[expectedName], "Expected tool '%s' not found", expectedName)
	}
}

func TestAllTools_HaveDescriptions(t *testing.T) {
	publisher := &events.NoOpPublisher{}
	tools := []Tool{
		NewLsTool(publisher),
		NewFindTool(publisher),
		NewReadFileTool(publisher),
		NewGrepTool(publisher),
		NewGitStatusTool(),
		NewBashTool(nil, nil, false),
	}

	for _, tool := range tools {
		decl := tool.Declaration()
		assert.NotEmpty(t, decl.Name, "Tool name should not be empty")
		assert.NotEmpty(t, decl.Description, "Tool description should not be empty")
		assert.NotNil(t, decl.Parameters, "Tool parameters should not be nil")
	}
}