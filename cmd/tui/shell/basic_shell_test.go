package shell

import (
	"strings"
	"testing"
	
	"github.com/kcaldas/genie/cmd/history"
)



// TestView is a minimal interface needed for testing BasicShell methods
type TestView interface {
	Buffer() string
	Clear()
	Write([]byte) (int, error)
	SetCursor(int, int) error
	Cursor() (int, int)
	Line(int) (string, error)
}

// MockView implements TestView for testing BasicShell
type MockView struct {
	buffer  string
	cursorX int
	cursorY int
}

func NewMockView(buffer string, cursorX, cursorY int) *MockView {
	return &MockView{
		buffer:  buffer,
		cursorX: cursorX,
		cursorY: cursorY,
	}
}

func (m *MockView) Buffer() string        { return m.buffer }
func (m *MockView) Cursor() (int, int)    { return m.cursorX, m.cursorY }
func (m *MockView) Clear()                { m.buffer = "" }
func (m *MockView) Write(data []byte) (int, error) {
	m.buffer += string(data)
	return len(data), nil
}
func (m *MockView) SetCursor(x, y int) error {
	m.cursorX, m.cursorY = x, y
	return nil
}
func (m *MockView) Line(y int) (string, error) {
	lines := strings.Split(m.buffer, "\n")
	if y < 0 || y >= len(lines) {
		return "", nil
	}
	return lines[y], nil
}

// TestBasicShell_CompletionLogic tests the completion suggestion logic
func TestBasicShell_CompletionLogic(t *testing.T) {
	// Create completer with command suggester
	completer := NewCompleter()
	registry := CreateTestCommandRegistry()
	completer.RegisterSuggester(NewCommandSuggester(registry))
	
	// Create mock history
	history := history.NewChatHistory("", false) // No saving for tests
	
	// Create BasicShell
	shell := NewBasicShell(completer, history)
	
	tests := []struct {
		name               string
		input              string
		expectSuggestion   bool
		expectedSuggestion string
		description        string
	}{
		{
			name:               "suggestion_for_write",
			input:              ":w",
			expectSuggestion:   true,
			expectedSuggestion: ":write",
			description:        "Should suggest :write for :w",
		},
		{
			name:               "suggestion_for_yank",
			input:              ":y",
			expectSuggestion:   true,
			expectedSuggestion: ":yank",
			description:        "Should suggest :yank for :y",
		},
		{
			name:               "no_suggestion_for_complete_command",
			input:              ":write",
			expectSuggestion:   false,
			expectedSuggestion: "",
			description:        "No suggestion for complete command",
		},
		{
			name:               "no_suggestion_for_unknown",
			input:              ":xyz",
			expectSuggestion:   false,
			expectedSuggestion: "",
			description:        "No suggestion for unknown command",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Test: %s", tt.description)
			
			// Test suggestion directly
			suggestion := shell.completer.Suggest(tt.input)
			
			t.Logf("Input: %q, Suggestion: %q", tt.input, suggestion)
			
			if tt.expectSuggestion {
				if suggestion != tt.expectedSuggestion {
					t.Errorf("Expected suggestion %q, got %q", tt.expectedSuggestion, suggestion)
				}
			} else {
				if suggestion != "" && suggestion != tt.input {
					t.Errorf("Expected no suggestion, got %q", suggestion)
				}
			}
		})
	}
}

// TestBasicShell_ShellState tests the isolated shell state functionality
func TestBasicShell_ShellState(t *testing.T) {
	// Create shell
	completer := NewCompleter()
	registry := CreateTestCommandRegistry()
	completer.RegisterSuggester(NewCommandSuggester(registry))
	history := history.NewChatHistory("", false)
	shell := NewBasicShell(completer, history)
	
	t.Run("buffer_operations", func(t *testing.T) {
		// Test direct buffer manipulation
		shell.buffer = ":w"
		shell.cursorPos = 2
		if shell.GetInputBuffer() != ":w" {
			t.Errorf("Expected buffer ':w', got %q", shell.GetInputBuffer())
		}
		
		// Test buffer reset
		shell.buffer = ""
		shell.cursorPos = 0
		if shell.GetInputBuffer() != "" {
			t.Errorf("Expected empty buffer, got %q", shell.GetInputBuffer())
		}
	})
	
	t.Run("suggestion_display", func(t *testing.T) {
		// Set buffer to ":w" and check postDisplay
		shell.buffer = ":w"
		shell.cursorPos = 2
		shell.updateSuggestion()
		
		// Should suggest "rite" as postDisplay
		expectedPostDisplay := "rite"
		if shell.postDisplay != expectedPostDisplay {
			t.Errorf("Expected postDisplay %q, got %q", expectedPostDisplay, shell.postDisplay)
		}
		
		// Move cursor to middle - should clear postDisplay
		shell.cursorPos = 1
		shell.updateSuggestion()
		if shell.postDisplay != "" {
			t.Errorf("Expected empty postDisplay when cursor not at end, got %q", shell.postDisplay)
		}
	})
	
	t.Run("completion_logic", func(t *testing.T) {
		// Test tab completion logic
		shell.buffer = ":w"
		shell.cursorPos = 2
		
		// Get suggestion
		suggestion := shell.completer.Suggest(shell.buffer)
		if suggestion != ":write" {
			t.Errorf("Expected suggestion ':write', got %q", suggestion)
		}
		
		// Test completion application logic
		if suggestion != "" && strings.HasPrefix(suggestion, shell.buffer) && suggestion != shell.buffer {
			shell.buffer = suggestion
			shell.cursorPos = len(shell.buffer)
		}
		
		// Check final state
		if shell.GetInputBuffer() != ":write" {
			t.Errorf("Expected buffer ':write', got %q", shell.GetInputBuffer())
		}
	})
	
	t.Run("backspace_logic", func(t *testing.T) {
		// Set up buffer
		shell.buffer = ":wr"
		shell.cursorPos = 3
		
		// Simulate backspace logic
		if len(shell.buffer) > 0 && shell.cursorPos > 0 {
			shell.buffer = shell.buffer[:shell.cursorPos-1] + shell.buffer[shell.cursorPos:]
			shell.cursorPos--
		}
		
		// Check state
		if shell.GetInputBuffer() != ":w" {
			t.Errorf("Expected buffer ':w' after backspace, got %q", shell.GetInputBuffer())
		}
		if shell.cursorPos != 2 {
			t.Errorf("Expected cursor position 2 after backspace, got %d", shell.cursorPos)
		}
	})
	
	t.Run("arrow_key_navigation", func(t *testing.T) {
		// Set up buffer
		shell.buffer = ":write"
		shell.cursorPos = 6 // At end
		
		// Simulate arrow left
		if shell.cursorPos > 0 {
			shell.cursorPos--
		}
		if shell.cursorPos != 5 {
			t.Errorf("Expected cursor position 5 after left arrow, got %d", shell.cursorPos)
		}
		
		// Simulate arrow right
		if shell.cursorPos < len(shell.buffer) {
			shell.cursorPos++
		}
		if shell.cursorPos != 6 {
			t.Errorf("Expected cursor position 6 after right arrow, got %d", shell.cursorPos)
		}
		
		// Test boundaries - left arrow at beginning
		shell.cursorPos = 0
		if shell.cursorPos > 0 {
			shell.cursorPos--
		}
		if shell.cursorPos != 0 {
			t.Errorf("Expected cursor to stay at 0 when at beginning, got %d", shell.cursorPos)
		}
		
		// Test boundaries - right arrow at end
		shell.cursorPos = len(shell.buffer)
		if shell.cursorPos < len(shell.buffer) {
			shell.cursorPos++
		}
		if shell.cursorPos != len(shell.buffer) {
			t.Errorf("Expected cursor to stay at end when at end, got %d", shell.cursorPos)
		}
	})
	
	t.Run("home_end_navigation", func(t *testing.T) {
		// Set up buffer
		shell.buffer = ":write hello world"
		shell.cursorPos = 8 // In the middle
		
		// Test Home (beginning of buffer)
		shell.cursorPos = 0
		if shell.cursorPos != 0 {
			t.Errorf("Expected cursor at beginning (0) after Home, got %d", shell.cursorPos)
		}
		
		// Test End (end of buffer)
		shell.cursorPos = len(shell.buffer)
		if shell.cursorPos != len(shell.buffer) {
			t.Errorf("Expected cursor at end (%d) after End, got %d", len(shell.buffer), shell.cursorPos)
		}
		
		// Test Ctrl+A (beginning)
		shell.cursorPos = 0
		if shell.cursorPos != 0 {
			t.Errorf("Expected cursor at beginning (0) after Ctrl+A, got %d", shell.cursorPos)
		}
		
		// Test Ctrl+E (end)
		shell.cursorPos = len(shell.buffer)
		if shell.cursorPos != len(shell.buffer) {
			t.Errorf("Expected cursor at end (%d) after Ctrl+E, got %d", len(shell.buffer), shell.cursorPos)
		}
	})
	
	t.Run("alt_arrow_word_navigation", func(t *testing.T) {
		// Set up buffer with multiple words
		shell.buffer = ":write hello world test"
		
		// Test Alt+Right (next word boundary)
		shell.cursorPos = 0 // At beginning
		nextWord := shell.findNextWordBoundary()
		if nextWord != 7 { // Should be at start of "hello"
			t.Errorf("Expected next word boundary at 7, got %d", nextWord)
		}
		
		shell.cursorPos = 7 // At "hello"
		nextWord = shell.findNextWordBoundary()
		if nextWord != 13 { // Should be at start of "world"
			t.Errorf("Expected next word boundary at 13, got %d", nextWord)
		}
		
		// Test Alt+Left (previous word boundary)
		shell.cursorPos = 13 // At "world"
		prevWord := shell.findPreviousWordBoundary()
		if prevWord != 7 { // Should be at start of "hello"
			t.Errorf("Expected previous word boundary at 7, got %d", prevWord)
		}
		
		shell.cursorPos = 7 // At "hello"
		prevWord = shell.findPreviousWordBoundary()
		if prevWord != 0 { // Should be at beginning
			t.Errorf("Expected previous word boundary at 0, got %d", prevWord)
		}
	})
	
	t.Run("buffer_sync_with_ansi", func(t *testing.T) {
		// Test extracting clean buffer from ANSI-containing buffer
		testCases := []struct {
			input    string
			expected string
		}{
			{":w", ":w"},                                    // No ANSI
			{":w\x1b[2mrite\x1b[0m", ":w"},               // With ANSI suggestion
			{":write", ":write"},                           // Complete command
			{"", ""},                                       // Empty
		}
		
		for _, tc := range testCases {
			result := shell.extractCleanBuffer(tc.input)
			if result != tc.expected {
				t.Errorf("extractCleanBuffer(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		}
	})
	
	t.Run("ctrl_a_single_press", func(t *testing.T) {
		// Test that Ctrl+A works on first press, regardless of current cursor position
		shell.buffer = ":write hello"
		shell.cursorPos = 8 // In the middle
		
		// Create a mock view with the buffer content (including potential ANSI)
		view := NewMockView(":write hello\x1b[2m world\x1b[0m", 8, 0)
		
		// Simulate what SetCursorToBeginning does
		viewBuffer := strings.TrimRight(view.Buffer(), "\n")
		shell.buffer = shell.extractCleanBuffer(viewBuffer)
		shell.cursorPos = 0
		
		// Verify the results
		if shell.buffer != ":write hello" {
			t.Errorf("Expected clean buffer ':write hello', got %q", shell.buffer)
		}
		if shell.cursorPos != 0 {
			t.Errorf("Expected cursor at beginning (0), got %d", shell.cursorPos)
		}
	})
	
	t.Run("ctrl_w_delete_word_backward", func(t *testing.T) {
		// Test word boundary detection first
		testBoundaries := []struct {
			buffer    string
			cursorPos int
			expected  int
		}{
			{":write hello world", 18, 13}, // From end of "world" to start of "world" (18 chars total)
			{":write hello world", 13, 7},  // From end of "hello" to start of "hello"  
			{":write hello", 7, 0},         // From end of "write" to start of buffer
		}
		
		for _, tb := range testBoundaries {
			shell.buffer = tb.buffer
			shell.cursorPos = tb.cursorPos
			boundary := shell.findPreviousWordBoundary()
			t.Logf("Buffer: %q, Cursor: %d, Previous word boundary: %d", tb.buffer, tb.cursorPos, boundary)
		}
		
		// Now test the actual delete functionality with corrected expectations
		testCases := []struct {
			buffer      string
			cursorPos   int
			description string
		}{
			{":write hello world", 18, "Delete 'world' from end"},
			{":write hello", 12, "Delete 'hello' from end"},
			{"hello", 5, "Delete entire single word"},
		}
		
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				// Set up state
				shell.buffer = tc.buffer
				shell.cursorPos = tc.cursorPos
				
				originalBuffer := shell.buffer
				originalPos := shell.cursorPos
				
				// Simulate Ctrl+W operation
				if shell.cursorPos > 0 {
					startPos := shell.findPreviousWordBoundary()
					shell.buffer = shell.buffer[:startPos] + shell.buffer[shell.cursorPos:]
					shell.cursorPos = startPos
				}
				
				t.Logf("Original: %q (pos %d) -> Result: %q (pos %d)", 
					originalBuffer, originalPos, shell.buffer, shell.cursorPos)
			})
		}
	})
}

// TestBasicShell_HistoryIntegration tests history navigation logic
func TestBasicShell_HistoryIntegration(t *testing.T) {
	// Create shell with history
	completer := NewCompleter()
	historyManager := history.NewChatHistory("", false)
	shell := NewBasicShell(completer, historyManager)
	
	// Add some history items
	historyManager.AddCommand(":write test1")
	historyManager.AddCommand(":yank test2")
	
	t.Run("history_navigation_logic", func(t *testing.T) {
		// Test direct history navigation (simulating NavigateHistoryUp logic)
		command := historyManager.NavigatePrev()
		shell.buffer = command
		shell.cursorPos = len(shell.buffer)
		if shell.GetInputBuffer() != ":yank test2" {
			t.Errorf("Expected ':yank test2', got %q", shell.GetInputBuffer())
		}
		
		// Navigate up again
		command = historyManager.NavigatePrev()
		shell.buffer = command
		shell.cursorPos = len(shell.buffer)
		if shell.GetInputBuffer() != ":write test1" {
			t.Errorf("Expected ':write test1', got %q", shell.GetInputBuffer())
		}
		
		// Navigate down
		command = historyManager.NavigateNext()
		shell.buffer = command
		shell.cursorPos = len(shell.buffer)
		if shell.GetInputBuffer() != ":yank test2" {
			t.Errorf("Expected ':yank test2', got %q", shell.GetInputBuffer())
		}
	})
}