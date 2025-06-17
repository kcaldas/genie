package fileops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_EnsureDir(t *testing.T) {
	manager := NewFileOpsManager()
	testDir := filepath.Join(os.TempDir(), "fileops_test_dir")

	// Clean up before test
	os.RemoveAll(testDir)

	err := manager.EnsureDir(testDir)
	require.NoError(t, err)

	// Verify directory exists
	stat, err := os.Stat(testDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())

	// Clean up after test
	os.RemoveAll(testDir)
}

func TestManager_WriteFile(t *testing.T) {
	manager := NewFileOpsManager()
	testDir := filepath.Join(os.TempDir(), "fileops_test_write")
	testFile := filepath.Join(testDir, "test.txt")
	content := []byte("Hello World")

	// Clean up before test
	os.RemoveAll(testDir)

	err := manager.WriteFile(testFile, content)
	require.NoError(t, err)

	// Verify file exists and has correct content
	readContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, content, readContent)

	// Clean up after test
	os.RemoveAll(testDir)
}

func TestManager_ReadFile(t *testing.T) {
	manager := NewFileOpsManager()
	testDir := filepath.Join(os.TempDir(), "fileops_test_read")
	testFile := filepath.Join(testDir, "test.txt")
	content := []byte("Test Content")

	// Setup test file
	os.MkdirAll(testDir, 0755)
	os.WriteFile(testFile, content, 0644)

	result, err := manager.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, content, result)

	// Clean up after test
	os.RemoveAll(testDir)
}

func TestManager_FileExists(t *testing.T) {
	manager := NewFileOpsManager()
	testDir := filepath.Join(os.TempDir(), "fileops_test_exists")
	existingFile := filepath.Join(testDir, "exists.txt")
	nonExistingFile := filepath.Join(testDir, "not_exists.txt")

	// Setup test file
	os.MkdirAll(testDir, 0755)
	os.WriteFile(existingFile, []byte("content"), 0644)

	assert.True(t, manager.FileExists(existingFile))
	assert.False(t, manager.FileExists(nonExistingFile))

	// Clean up after test
	os.RemoveAll(testDir)
}

func TestManager_WriteObjectAsYAML(t *testing.T) {
	manager := NewFileOpsManager()
	testDir := filepath.Join(os.TempDir(), "fileops_test_yaml")
	testFile := filepath.Join(testDir, "test.yaml")

	testObject := map[string]string{
		"name":  "test",
		"value": "123",
	}

	// Clean up before test
	os.RemoveAll(testDir)

	err := manager.WriteObjectAsYAML(testFile, testObject)
	require.NoError(t, err)

	// Verify file exists
	assert.True(t, manager.FileExists(testFile))

	// Clean up after test
	os.RemoveAll(testDir)
}
