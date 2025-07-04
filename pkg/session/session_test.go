package session

import (
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestSession_WorkingDirectory(t *testing.T) {
	publisher := events.NewEventBus()
	workingDir := "/path/to/project"

	// Test creating session with working directory parameter
	session := NewSession(workingDir, "engineer", publisher)
	assert.Equal(t, workingDir, session.GetWorkingDirectory())
	assert.Equal(t, "engineer", session.GetPersona())
}

func TestSession_WorkingDirectoryDifferentPaths(t *testing.T) {
	publisher := events.NewEventBus()

	// Test different working directories
	testCases := []string{
		"/home/user/project",
		"/tmp/workspace",
		".",
		"/absolute/path/to/project",
	}

	for _, expectedPath := range testCases {
		session := NewSession(expectedPath, "product_owner", publisher)
		assert.Equal(t, expectedPath, session.GetWorkingDirectory())
		assert.Equal(t, "product_owner", session.GetPersona())
	}
}

func TestSession_EmptyPersona(t *testing.T) {
	publisher := events.NewEventBus()
	workingDir := "/test/dir"

	// Test creating session with empty persona
	session := NewSession(workingDir, "", publisher)
	assert.Equal(t, workingDir, session.GetWorkingDirectory())
	assert.Equal(t, "", session.GetPersona())
}
