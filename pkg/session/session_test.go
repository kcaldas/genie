package session

import (
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestSession_GetID(t *testing.T) {
	publisher := events.NewEventBus()
	session := NewSession("my-session-id", "/test/workdir", publisher)
	assert.Equal(t, "my-session-id", session.GetID())
}

func TestSession_WorkingDirectory(t *testing.T) {
	publisher := events.NewEventBus()
	workingDir := "/path/to/project"

	// Test creating session with working directory parameter
	session := NewSession("test-session", workingDir, publisher)
	assert.Equal(t, workingDir, session.GetWorkingDirectory())
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
		session := NewSession("test-session", expectedPath, publisher)
		assert.Equal(t, expectedPath, session.GetWorkingDirectory())
	}
}
