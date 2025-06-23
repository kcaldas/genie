package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestNewScrollableConfirmation(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "File Changes",
		Content:     "Line 1\nLine 2\nLine 3",
		ContentType: "diff",
		FilePath:    "/path/to/file.go",
		ConfirmText: "Apply",
		CancelText:  "Reject",
	}
	width, height := 80, 25

	model := NewScrollableConfirmation(request, width, height)

	assert.Equal(t, "File Changes", model.title)
	assert.Equal(t, "/path/to/file.go", model.filePath)
	assert.Equal(t, "Line 1\nLine 2\nLine 3", model.diffContent)
	assert.Equal(t, "test-123", model.executionID)
	assert.Equal(t, 0, model.selectedIndex) // Default to "Yes"
	assert.Equal(t, width, model.width)
	assert.Equal(t, height, model.height)
	assert.Equal(t, "diff", model.contentType)
	assert.Equal(t, "Apply", model.confirmText)
	assert.Equal(t, "Reject", model.cancelText)
	assert.Equal(t, 0, model.scrollOffset)
}

func TestNewScrollableConfirmation_DefaultTexts(t *testing.T) {
	testCases := []struct {
		name               string
		contentType        string
		expectedConfirm    string
		expectedCancel     string
	}{
		{
			name:            "Plan content type",
			contentType:     "plan",
			expectedConfirm: "Proceed with implementation",
			expectedCancel:  "Revise plan",
		},
		{
			name:            "Diff content type",
			contentType:     "diff",
			expectedConfirm: "Apply changes",
			expectedCancel:  "Cancel",
		},
		{
			name:            "Other content type",
			contentType:     "custom",
			expectedConfirm: "Apply changes",
			expectedCancel:  "Cancel",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := events.UserConfirmationRequest{
				ExecutionID: "test-123",
				Title:       "Test",
				Content:     "content",
				ContentType: tc.contentType,
				// Leave ConfirmText and CancelText empty to test defaults
			}

			model := NewScrollableConfirmation(request, 80, 25)

			assert.Equal(t, tc.expectedConfirm, model.confirmText)
			assert.Equal(t, tc.expectedCancel, model.cancelText)
		})
	}
}

func TestNewDiffConfirmation_BackwardCompatibility(t *testing.T) {
	title := "File Changes"
	filePath := "/path/to/file.go"
	diffContent := "+added line\n-removed line"
	executionID := "test-123"

	model := NewDiffConfirmation(title, filePath, diffContent, executionID, 80, 25)

	assert.Equal(t, title, model.title)
	assert.Equal(t, filePath, model.filePath)
	assert.Equal(t, diffContent, model.diffContent)
	assert.Equal(t, executionID, model.executionID)
	assert.Equal(t, "diff", model.contentType)
}

func TestNewPlanConfirmation_BackwardCompatibility(t *testing.T) {
	title := "Implementation Plan"
	planContent := "## Step 1\n- Do something\n## Step 2\n- Do something else"
	executionID := "test-123"

	model := NewPlanConfirmation(title, planContent, executionID, 80, 25)

	assert.Equal(t, title, model.title)
	assert.Equal(t, planContent, model.diffContent)
	assert.Equal(t, executionID, model.executionID)
	assert.Equal(t, "plan", model.contentType)
}

func TestScrollableConfirmation_Init(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     "content",
		ContentType: "diff",
	}
	model := NewScrollableConfirmation(request, 80, 25)
	
	cmd := model.Init()
	assert.Nil(t, cmd, "Init should return nil command")
}

func TestScrollableConfirmation_BasicNavigation(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     "short content",
		ContentType: "diff",
	}
	model := NewScrollableConfirmation(request, 80, 25)

	// Test up arrow (navigate to Yes when at No)
	model.selectedIndex = 1
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	newModelInterface, cmd := model.Update(upMsg)
	newModel := newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 0, newModel.selectedIndex)
	assert.Nil(t, cmd)

	// Test down arrow (navigate to No when at Yes)
	model.selectedIndex = 0
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	newModelInterface, cmd = model.Update(downMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 1, newModel.selectedIndex)
	assert.Nil(t, cmd)

	// Test left/right navigation
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	newModelInterface, cmd = model.Update(leftMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 0, newModel.selectedIndex) // Should go to Yes

	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	newModelInterface, cmd = model.Update(rightMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 1, newModel.selectedIndex) // Should go to No
}

func TestScrollableConfirmation_ContentScrolling(t *testing.T) {
	// Create content that requires scrolling
	longContent := ""
	for i := 0; i < 50; i++ {
		longContent += "Line " + string(rune('A'+i%26)) + "\n"
	}

	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     longContent,
		ContentType: "diff",
	}
	model := NewScrollableConfirmation(request, 80, 25)

	// Initially at top
	assert.Equal(t, 0, model.scrollOffset)

	// Test up arrow when at Yes - should scroll content up
	model.selectedIndex = 0
	model.scrollOffset = 5
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	newModelInterface, cmd := model.Update(upMsg)
	newModel := newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 4, newModel.scrollOffset) // Should scroll up
	assert.Equal(t, 0, newModel.selectedIndex) // Should stay at Yes
	assert.Nil(t, cmd)

	// Test down arrow when at No - should scroll content down
	model.selectedIndex = 1
	model.scrollOffset = 5
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	newModelInterface, cmd = model.Update(downMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 6, newModel.scrollOffset) // Should scroll down
	assert.Equal(t, 1, newModel.selectedIndex) // Should stay at No
	assert.Nil(t, cmd)

	// Test scroll bounds - can't scroll above 0
	model.scrollOffset = 0
	newModelInterface, cmd = model.Update(upMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 0, newModel.scrollOffset) // Should stay at 0

	// Test scroll bounds - can't scroll below maxScroll
	model.scrollOffset = model.maxScroll
	newModelInterface, cmd = model.Update(downMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, model.maxScroll, newModel.scrollOffset) // Should stay at max
}

func TestScrollableConfirmation_PageScrolling(t *testing.T) {
	// Create content that requires scrolling - exactly 50 lines
	longContent := ""
	for i := 0; i < 50; i++ {
		longContent += "Line " + string(rune('A'+i%26))
		if i < 49 { // Don't add newline to the last line
			longContent += "\n"
		}
	}

	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     longContent,
		ContentType: "diff",
	}
	model := NewScrollableConfirmation(request, 80, 25)
	model.scrollOffset = 10

	// Test page up
	pgUpMsg := tea.KeyMsg{Type: tea.KeyPgUp}
	newModelInterface, cmd := model.Update(pgUpMsg)
	newModel := newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 5, newModel.scrollOffset) // Should scroll up by 5
	assert.Nil(t, cmd)

	// Test page down
	pgDownMsg := tea.KeyMsg{Type: tea.KeyPgDown}
	newModelInterface, cmd = model.Update(pgDownMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 15, newModel.scrollOffset) // Should scroll down by 5
	assert.Nil(t, cmd)

	// Test another page down - should hit maxScroll or continue scrolling  
	newModelInterface, cmd = model.Update(pgDownMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	// Should either scroll to 20 or stay at 15 if that's the maxScroll
	assert.True(t, newModel.scrollOffset >= 15, "Should be at least at current position")
	assert.True(t, newModel.scrollOffset <= model.maxScroll, "Should not exceed maxScroll")

	// Test page up at top - should not go below 0
	model.scrollOffset = 2
	newModelInterface, cmd = model.Update(pgUpMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 0, newModel.scrollOffset) // Should be clamped to 0

	// Test page down at bottom - should not exceed maxScroll
	model.scrollOffset = model.maxScroll - 2
	newModelInterface, cmd = model.Update(pgDownMsg)
	newModel = newModelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, model.maxScroll, newModel.scrollOffset) // Should be clamped to max
}

func TestScrollableConfirmation_DirectSelection(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     "content",
		ContentType: "diff",
	}
	model := NewScrollableConfirmation(request, 80, 25)

	testCases := []struct {
		name        string
		key         string
		expectedYes bool
	}{
		{"Key 1 selects Yes", "1", true},
		{"Key 2 selects No", "2", false},
		{"ESC selects No", "esc", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var keyMsg tea.KeyMsg
			if tc.key == "esc" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
			}

			newModelInterface, cmd := model.Update(keyMsg)
			newModel := newModelInterface.(ScrollableConfirmationModel)
			require.NotNil(t, cmd, "Should return a command")

			// Execute the command to get the message
			msg := cmd()
			responseMsg, ok := msg.(scrollableConfirmationResponseMsg)
			require.True(t, ok, "Should return scrollableConfirmationResponseMsg")

			assert.Equal(t, "test-123", responseMsg.executionID)
			assert.Equal(t, tc.expectedYes, responseMsg.confirmed)

			// Model should remain unchanged
			assert.Equal(t, model.selectedIndex, newModel.selectedIndex)
		})
	}
}

func TestScrollableConfirmation_EnterConfirmsSelection(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-456",
		Title:       "Test",
		Content:     "content",
		ContentType: "diff",
	}
	model := NewScrollableConfirmation(request, 80, 25)

	testCases := []struct {
		name           string
		selectedIndex  int
		expectedResult bool
	}{
		{"Enter on Yes (index 0)", 0, true},
		{"Enter on No (index 1)", 1, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model.selectedIndex = tc.selectedIndex

			enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
			newModelInterface, cmd := model.Update(enterMsg)
			newModel := newModelInterface.(ScrollableConfirmationModel)
			require.NotNil(t, cmd, "Should return a command")

			// Execute the command to get the message
			msg := cmd()
			responseMsg, ok := msg.(scrollableConfirmationResponseMsg)
			require.True(t, ok, "Should return scrollableConfirmationResponseMsg")

			assert.Equal(t, "test-456", responseMsg.executionID)
			assert.Equal(t, tc.expectedResult, responseMsg.confirmed)

			// Model should remain unchanged
			assert.Equal(t, tc.selectedIndex, newModel.selectedIndex)
		})
	}
}

func TestScrollableConfirmation_ContentTypeRendering(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		content     string
		expectLabel string
	}{
		{
			name:        "Diff content type",
			contentType: "diff",
			content:     "+added\n-removed",
			expectLabel: "Changes to be made:",
		},
		{
			name:        "Plan content type", 
			contentType: "plan",
			content:     "## Step 1\n- Do something",
			expectLabel: "Implementation Plan:",
		},
		{
			name:        "Other content type",
			contentType: "custom",
			content:     "custom content",
			expectLabel: "Changes to be made:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := events.UserConfirmationRequest{
				ExecutionID: "test-123",
				Title:       "Test",
				Content:     tc.content,
				ContentType: tc.contentType,
			}
			model := NewScrollableConfirmation(request, 80, 25)

			view := model.View()
			assert.Contains(t, view, tc.expectLabel)
			assert.Contains(t, view, "Test") // Title should always be present
		})
	}
}

func TestScrollableConfirmation_CustomButtonText(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Custom Buttons",
		Content:     "content",
		ContentType: "diff",
		ConfirmText: "Deploy Now",
		CancelText:  "Abort Mission",
	}
	model := NewScrollableConfirmation(request, 80, 25)

	view := model.View()
	assert.Contains(t, view, "Yes - Deploy Now")
	assert.Contains(t, view, "No  - Abort Mission")
}

func TestScrollableConfirmation_DiffSyntaxHighlighting(t *testing.T) {
	diffContent := `--- file.go
+++ file.go
@@ -1,3 +1,4 @@
 func main() {
+    fmt.Println("added")
-    fmt.Println("removed")
     return
 }`

	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Diff",
		Content:     diffContent,
		ContentType: "diff",
	}
	model := NewScrollableConfirmation(request, 80, 25)

	// Test that renderStyledContent doesn't crash with diff content
	styledContent := model.renderStyledContent()
	assert.NotEmpty(t, styledContent)

	// The actual styling is handled by lipgloss, but we can verify
	// the content is processed correctly
	view := model.View()
	assert.Contains(t, view, "file.go")
	assert.Contains(t, view, "main()")
}

func TestScrollableConfirmation_PlanSyntaxHighlighting(t *testing.T) {
	planContent := `## Implementation Plan

### Step 1: Setup
- Initialize project
- Configure dependencies

### Step 2: Implementation  
- Write core logic
- Add error handling

### Step 3: Testing
- Unit tests
- Integration tests`

	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Plan",
		Content:     planContent,
		ContentType: "plan",
	}
	model := NewScrollableConfirmation(request, 80, 25)

	// Test that renderStyledContent handles plan content
	styledContent := model.renderStyledContent()
	assert.NotEmpty(t, styledContent)

	view := model.View()
	assert.Contains(t, view, "Implementation Plan")
	assert.Contains(t, view, "Step 1")
}

func TestScrollableConfirmation_EmptyContent(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		expectMsg   string
	}{
		{
			name:        "Empty diff content",
			contentType: "diff",
			expectMsg:   "No changes to display",
		},
		{
			name:        "Empty plan content",
			contentType: "plan", 
			expectMsg:   "No plan to display",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := events.UserConfirmationRequest{
				ExecutionID: "test-123",
				Title:       "Empty",
				Content:     "", // Empty content
				ContentType: tc.contentType,
			}
			model := NewScrollableConfirmation(request, 80, 25)

			styledContent := model.renderStyledContent()
			assert.Contains(t, styledContent, tc.expectMsg)
		})
	}
}

func TestScrollableConfirmation_HeightCalculations(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     "content",
		ContentType: "diff",
	}

	testCases := []struct {
		name           string
		height         int
		expectedHeight int
	}{
		{"Normal height", 25, 14}, // 25 - 11 reserved lines = 14
		{"Small height", 15, 5},   // Should clamp to minimum of 5
		{"Very small height", 5, 5}, // Should clamp to minimum of 5
		{"Large height", 50, 39},   // 50 - 11 reserved lines = 39
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := NewScrollableConfirmation(request, 80, tc.height)
			actualHeight := model.getContentDisplayHeight()
			assert.Equal(t, tc.expectedHeight, actualHeight)
		})
	}
}

func TestScrollableConfirmation_ScrollLimits(t *testing.T) {
	// Create content with known number of lines
	lines := []string{}
	for i := 0; i < 30; i++ {
		lines = append(lines, "Line "+string(rune('A'+i%26)))
	}
	content := strings.Join(lines, "\n")

	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     content,
		ContentType: "diff",
	}
	
	// Use height that will require scrolling
	model := NewScrollableConfirmation(request, 80, 20)
	
	// The constructor uses height - 12 for maxContentHeight calculation
	// With height 20, maxContentHeight = 20 - 12 = 8
	// With 30 lines of content, maxScroll should be 30 - 8 = 22
	expectedMaxScroll := 30 - (20 - 12) // 30 - 8 = 22
	assert.Equal(t, expectedMaxScroll, model.maxScroll)

	// Test that scroll offset is properly initialized
	assert.Equal(t, 0, model.scrollOffset)

	// Test scroll limits in scrolling
	model.scrollOffset = model.maxScroll + 10 // Try to go beyond max
	pgDownMsg := tea.KeyMsg{Type: tea.KeyPgDown}
	newModelInterface, _ := model.Update(pgDownMsg)
	newModel := newModelInterface.(ScrollableConfirmationModel)
	assert.LessOrEqual(t, newModel.scrollOffset, model.maxScroll)
}

func TestScrollableConfirmation_WithFilePath(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "File Changes",
		Content:     "+new line",
		ContentType: "diff",
		FilePath:    "/path/to/important/file.go",
	}
	model := NewScrollableConfirmation(request, 80, 25)

	view := model.View()
	
	// For diff content type, file path should be displayed
	assert.Contains(t, view, "/path/to/important/file.go")
	assert.Contains(t, view, "File Changes")
	assert.Contains(t, view, "Changes to be made:")
}

func TestScrollableConfirmation_WithoutFilePath(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Plan Review",
		Content:     "## Step 1\n- Do something",
		ContentType: "plan",
		// No FilePath for plans
	}
	model := NewScrollableConfirmation(request, 80, 25)

	view := model.View()
	
	// For plan content type, no file path should be shown
	assert.Contains(t, view, "Plan Review")
	assert.Contains(t, view, "Implementation Plan:")
	assert.Contains(t, view, "Step 1")
}

func TestScrollableConfirmation_ScrollIndicators(t *testing.T) {
	// Create long content that requires scrolling
	longContent := ""
	for i := 0; i < 50; i++ {
		longContent += "Line " + string(rune('0'+i%10)) + "\n"
	}

	request := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     longContent,
		ContentType: "diff",
	}
	model := NewScrollableConfirmation(request, 80, 20)

	// When content is scrollable, help text should include scroll indicators
	view := model.View()
	if model.maxScroll > 0 {
		assert.Contains(t, view, "PgUp/PgDn to scroll")
		assert.Contains(t, view, "showing") // Part of "showing X-Y of Z lines"
	}

	// Test with short content that doesn't need scrolling
	shortRequest := events.UserConfirmationRequest{
		ExecutionID: "test-123",
		Title:       "Test",
		Content:     "Short content",
		ContentType: "diff",
	}
	shortModel := NewScrollableConfirmation(shortRequest, 80, 20)
	shortView := shortModel.View()
	
	// Should not show scroll indicators for short content
	if shortModel.maxScroll == 0 {
		assert.NotContains(t, shortView, "PgUp/PgDn to scroll")
	}
}

// Integration test - verifies the component can be used in a tea.Program
func TestScrollableConfirmation_TeaIntegration(t *testing.T) {
	request := events.UserConfirmationRequest{
		ExecutionID: "integration-test",
		Title:       "Integration Test",
		Content:     "Line 1\nLine 2\nLine 3",
		ContentType: "diff",
		FilePath:    "/test/file.go",
	}
	model := NewScrollableConfirmation(request, 80, 25)
	
	// Verify it satisfies tea.Model interface
	var _ tea.Model = model
	
	// Test a full interaction cycle
	// 1. Init
	cmd := model.Init()
	assert.Nil(t, cmd)
	
	// 2. Navigate down to No
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	modelInterface, cmd := model.Update(downMsg)
	model = modelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 1, model.selectedIndex)
	assert.Nil(t, cmd)
	
	// 3. Use left arrow to go back to Yes
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	modelInterface, cmd = model.Update(leftMsg)
	model = modelInterface.(ScrollableConfirmationModel)
	assert.Equal(t, 0, model.selectedIndex)
	assert.Nil(t, cmd)
	
	// 4. Press 2 for direct No selection
	keyTwoMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	modelInterface, cmd = model.Update(keyTwoMsg)
	model = modelInterface.(ScrollableConfirmationModel)
	require.NotNil(t, cmd)
	
	// 5. Execute command
	msg := cmd()
	responseMsg, ok := msg.(scrollableConfirmationResponseMsg)
	require.True(t, ok)
	assert.Equal(t, "integration-test", responseMsg.executionID)
	assert.False(t, responseMsg.confirmed) // Selected No
	
	// 6. Render
	view := model.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Integration Test")
	assert.Contains(t, view, "/test/file.go")
}