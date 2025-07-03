package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFileManager is a mock implementation of fileops.Manager
type MockFileManager struct {
	mock.Mock
}

func (m *MockFileManager) ReadFile(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileManager) WriteFile(path string, content []byte) error {
	args := m.Called(path, content)
	return args.Error(0)
}

func (m *MockFileManager) FileExists(path string) bool {
	args := m.Called(path)
	return args.Bool(0)
}

func (m *MockFileManager) EnsureDir(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockFileManager) WriteObjectAsYAML(path string, object interface{}) error {
	args := m.Called(path, object)
	return args.Error(0)
}

func TestGenerateUnifiedDiff_ExistingFileModification(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		oldContent  string
		newContent  string
		wantDiffLen int
		wantErr     bool
	}{
		{
			name:       "simple one line change",
			filePath:   "/path/to/file.txt",
			oldContent: "Hello World",
			newContent: "Hello Genie",
			wantErr:    false,
		},
		{
			name:       "multi-line file with changes",
			filePath:   "/path/to/code.go",
			oldContent: "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}\n",
			newContent: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n",
			wantErr:    false,
		},
		{
			name:       "identical content should error",
			filePath:   "/path/to/same.txt",
			oldContent: "Same content",
			newContent: "Same content",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := new(MockFileManager)
			mockFS.On("FileExists", tt.filePath).Return(true)
			mockFS.On("ReadFile", tt.filePath).Return([]byte(tt.oldContent), nil)

			dg := NewDiffGenerator(mockFS)
			diff, err := dg.GenerateUnifiedDiff(tt.filePath, tt.newContent)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, diff)
				assert.Contains(t, diff, "---")
				assert.Contains(t, diff, "+++")
				assert.Contains(t, diff, "@@")
			}

			mockFS.AssertExpectations(t)
		})
	}
}

func TestGenerateUnifiedDiff_NewFile(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		newContent  string
		wantErr     bool
		checkNewFile bool
	}{
		{
			name:         "create new simple file",
			filePath:     "/path/to/new.txt",
			newContent:   "This is a new file\n",
			checkNewFile: true,
		},
		{
			name:         "create new multi-line file",
			filePath:     "/path/to/new.go",
			newContent:   "package main\n\nfunc main() {\n\tprintln(\"New file\")\n}\n",
			checkNewFile: true,
		},
		{
			name:       "create empty file",
			filePath:   "/path/to/empty.txt",
			newContent: "",
			wantErr:    true, // Empty new file is same as empty old file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := new(MockFileManager)
			mockFS.On("FileExists", tt.filePath).Return(false)

			dg := NewDiffGenerator(mockFS)
			diff, err := dg.GenerateUnifiedDiff(tt.filePath, tt.newContent)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, diff)
				
				// The library generates a regular diff when file doesn't exist
				// It treats it as comparing empty string to new content
				assert.Contains(t, diff, "---")
				assert.Contains(t, diff, "+++")
				assert.Contains(t, diff, tt.filePath)
				
				// Check that new content lines are marked as additions
				if tt.newContent != "" {
					lines := strings.Split(strings.TrimSuffix(tt.newContent, "\n"), "\n")
					for _, line := range lines {
						if line != "" {
							assert.Contains(t, diff, "+"+line)
						}
					}
				}
			}

			mockFS.AssertExpectations(t)
		})
	}
}

func TestGenerateNewFileDiff(t *testing.T) {
	dg := NewDiffGenerator(nil) // No file manager needed for this test

	tests := []struct {
		name     string
		filePath string
		content  string
		wantDiff string
	}{
		{
			name:     "single line file",
			filePath: "/test.txt",
			content:  "Hello",
			wantDiff: "--- /dev/null\n+++ /test.txt\n@@ -0,0 +1,1 @@\n+Hello\n",
		},
		{
			name:     "multi-line file with trailing newline",
			filePath: "/multi.txt",
			content:  "Line 1\nLine 2\n",
			wantDiff: "--- /dev/null\n+++ /multi.txt\n@@ -0,0 +1,3 @@\n+Line 1\n+Line 2\n", // Split creates 3 elements
		},
		{
			name:     "empty file",
			filePath: "/empty.txt",
			content:  "",
			wantDiff: "--- /dev/null\n+++ /empty.txt\n@@ -0,0 +1,1 @@\n+\n", // Split creates 1 element for empty string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dg.generateNewFileDiff(tt.filePath, tt.content)
			assert.Equal(t, tt.wantDiff, result)
		})
	}
}

func TestAnalyzeDiff(t *testing.T) {
	dg := NewDiffGenerator(nil)

	tests := []struct {
		name     string
		diff     string
		expected DiffSummary
	}{
		{
			name: "new file diff",
			diff: `--- /dev/null
+++ /path/to/new.txt
@@ -0,0 +1,3 @@
+Line 1
+Line 2
+Line 3`,
			expected: DiffSummary{
				FilePath:     "/path/to/new.txt",
				IsNewFile:    true,
				IsModified:   false,
				LinesAdded:   3,
				LinesRemoved: 0,
				TotalLines:   3,
			},
		},
		{
			name: "modified file diff",
			diff: `--- /path/to/file.txt
+++ /path/to/file.txt
@@ -1,4 +1,4 @@
 Line 1
-Line 2 old
+Line 2 new
 Line 3
-Line 4 old
+Line 4 new`,
			expected: DiffSummary{
				FilePath:     "/path/to/file.txt",
				IsNewFile:    false,
				IsModified:   true,
				LinesAdded:   2,
				LinesRemoved: 2,
				TotalLines:   4,
			},
		},
		{
			name: "file with only additions",
			diff: `--- /path/to/file.txt
+++ /path/to/file.txt
@@ -1,2 +1,5 @@
 Line 1
 Line 2
+Line 3
+Line 4
+Line 5`,
			expected: DiffSummary{
				FilePath:     "/path/to/file.txt",
				IsNewFile:    false,
				IsModified:   true,
				LinesAdded:   3,
				LinesRemoved: 0,
				TotalLines:   3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dg.AnalyzeDiff(tt.diff)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDiffForDisplay(t *testing.T) {
	dg := NewDiffGenerator(nil)

	tests := []struct {
		name     string
		diff     string
		expected string
	}{
		{
			name:     "empty diff",
			diff:     "",
			expected: "No changes to display",
		},
		{
			name: "standard diff preserves structure",
			diff: `--- /path/to/file.txt
+++ /path/to/file.txt
@@ -1,3 +1,3 @@
 Line 1
-Line 2 old
+Line 2 new
 Line 3`,
			expected: `--- /path/to/file.txt
+++ /path/to/file.txt
@@ -1,3 +1,3 @@
 Line 1
-Line 2 old
+Line 2 new
 Line 3`,
		},
		{
			name: "diff with empty lines",
			diff: `--- /path/to/file.txt
+++ /path/to/file.txt
@@ -1,3 +1,4 @@
 Line 1

+Line 2
 Line 3`,
			expected: `--- /path/to/file.txt
+++ /path/to/file.txt
@@ -1,3 +1,4 @@
 Line 1

+Line 2
 Line 3`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dg.FormatDiffForDisplay(tt.diff)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateUnifiedDiff_ErrorCases(t *testing.T) {
	t.Run("file read error", func(t *testing.T) {
		mockFS := new(MockFileManager)
		mockFS.On("FileExists", "/error/file.txt").Return(true)
		mockFS.On("ReadFile", "/error/file.txt").Return([]byte{}, assert.AnError)

		dg := NewDiffGenerator(mockFS)
		_, err := dg.GenerateUnifiedDiff("/error/file.txt", "new content")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error reading existing file")

		mockFS.AssertExpectations(t)
	})
}