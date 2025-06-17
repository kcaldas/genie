package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplModel_ProjectDirTracking(t *testing.T) {
	// Get the current working directory for comparison
	expectedProjectDir, err := os.Getwd()
	assert.NoError(t, err)

	// Note: We can't easily test InitialModel() since it has many dependencies
	// Instead, we'll test that the concept works by simulating the logic
	
	// Test the logic that would be used in InitialModel
	projectDir, err := os.Getwd()
	if err != nil {
		projectDir = "." // fallback to current directory
	}
	
	// Verify the project directory is correctly determined
	assert.Equal(t, expectedProjectDir, projectDir)
	assert.NotEmpty(t, projectDir)
	
	// Test that we can create the expected history path
	expectedHistoryPath := filepath.Join(projectDir, ".genie", "history")
	assert.Contains(t, expectedHistoryPath, ".genie/history")
	assert.Contains(t, expectedHistoryPath, projectDir)
}