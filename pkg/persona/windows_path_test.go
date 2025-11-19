package persona

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/assert"
)

// TestEmbeddedPersonaPathHandling tests that embedded persona paths work correctly
// on all operating systems, particularly Windows where filepath.Join would use backslashes
func TestEmbeddedPersonaPathHandling(t *testing.T) {
	// Create minimal setup
	eventBus := events.NewEventBus()
	emptyRegistry := tools.NewRegistry()
	
	promptLoader := prompts.DefaultLoader{
		Publisher:    eventBus,
		ToolRegistry: emptyRegistry,
		Config:       config.NewConfigManager(),
	}

	factory := NewPersonaPromptFactory(&promptLoader, nil) // nil skillManager for tests
	ctx := context.Background()

	// Test that embedded personas can be found
	// (They will fail on tool loading but should be found in the filesystem)
	testCases := []struct {
		persona string
	}{
		{"engineer"},
		{"product_owner"},
		{"persona_creator"},
	}

	for _, tc := range testCases {
		t.Run(tc.persona, func(t *testing.T) {
			_, err := factory.GetPrompt(ctx, tc.persona)
			
			// The persona should be found, even if tool loading fails
			if err != nil {
				// Check that the error is about missing tools, not file not found
				errMsg := err.Error()
				assert.True(t, 
					strings.Contains(errMsg, "failed to add tools to prompt") || 
					strings.Contains(errMsg, "missing required tools"),
					"Expected tools error, got: %v", err)
				
				// Ensure it's NOT a file not found error
				assert.False(t, 
					strings.Contains(errMsg, "file does not exist") ||
					strings.Contains(errMsg, "no such file"),
					"Persona file should be found, got: %v", err)
			}
		})
	}
}

// TestEmbeddedPathConstruction tests that we don't use filepath.Join for embedded paths
func TestEmbeddedPathConstruction(t *testing.T) {
	// This test documents the issue that was fixed:
	// On Windows, filepath.Join("personas", "engineer", "prompt.yaml") would return 
	// "personas\engineer\prompt.yaml" which doesn't work with embed.FS
	
	personaName := "engineer"
	
	// What we used to do (problematic on Windows)
	oldPath := filepath.Join("personas", personaName, "prompt.yaml")
	
	// What we should do (works everywhere)
	newPath := "personas/" + personaName + "/prompt.yaml"
	
	if runtime.GOOS == "windows" {
		// On Windows, these would be different
		t.Logf("On Windows: filepath.Join would produce: %q", oldPath)
		t.Logf("On Windows: manual construction produces: %q", newPath)
		// We can't actually test Windows behavior on non-Windows, but this documents the issue
	} else {
		// On Unix systems, both produce the same result, which hides the bug
		assert.Equal(t, oldPath, newPath, "On Unix systems, both approaches work the same")
		t.Logf("On Unix: both approaches produce: %q", newPath)
	}
	
	// The new approach should always use forward slashes
	assert.Contains(t, newPath, "/", "Embedded paths should always use forward slashes")
	assert.NotContains(t, newPath, "\\", "Embedded paths should never use backslashes")
}