package shell

import (
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/history"
)

// BasicShell implements the gocui.Editor interface and provides core shell functionalities.
type BasicShell struct {
	completer *Completer
	history   history.ChatHistory
	
	historyNavigating bool
	currentInput      string
	
	buffer      string // Clean command buffer
	postDisplay string // Suggestion text after cursor
	cursorPos   int
	scrollOffset int   // Horizontal scroll offset for long input
}

// NewBasicShell creates a new instance of BasicShell.
func NewBasicShell(completer *Completer, historyManager history.ChatHistory) *BasicShell {
	return &BasicShell{
		completer: completer,
		history:   historyManager,
	}
}

// Edit handles key presses for the shell.
func (s *BasicShell) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {

	if key == gocui.KeyTab {
		s.triggerCompletion(v)
		return
	}

	if key == gocui.KeyArrowRight {
		if s.applyCompletionIfPossible(v) {
			return
		}
	}

	if key == gocui.KeyBackspace || key == gocui.KeyBackspace2 {
		s.handleBackspace(v)
		return
	}

	switch key {
	case gocui.KeyArrowLeft:
		if s.cursorPos > 0 {
			s.cursorPos--
		}
	case gocui.KeyArrowRight:
		if s.cursorPos < len(s.buffer) {
			s.cursorPos++
		}
	case gocui.KeyArrowUp:
	case gocui.KeyArrowDown:
	case gocui.KeyDelete:
		if s.cursorPos < len(s.buffer) {
			s.buffer = s.buffer[:s.cursorPos] + s.buffer[s.cursorPos+1:]
		}
	default:
		if IsUnboundSpecialKey(key) {
			return
		}
		if ch != 0 {
			s.buffer = s.buffer[:s.cursorPos] + string(ch) + s.buffer[s.cursorPos:]
			s.cursorPos++
		} else if key == gocui.KeySpace {
			s.buffer = s.buffer[:s.cursorPos] + " " + s.buffer[s.cursorPos:]
			s.cursorPos++
		}
	}

	if s.historyNavigating && (ch != 0 || key == gocui.KeyBackspace || key == gocui.KeyBackspace2 || key == gocui.KeyDelete) {
		s.history.ResetNavigation()
		s.historyNavigating = false
	}

	s.render(v)
}

// updateSuggestion updates the postDisplay based on current buffer
func (s *BasicShell) updateSuggestion() {
	if s.cursorPos == len(s.buffer) {
		suggestion := s.completer.Suggest(s.buffer)
		if suggestion != "" && suggestion != s.buffer {
			s.postDisplay = suggestion[len(s.buffer):]
		} else {
			s.postDisplay = ""
		}
	} else {
		s.postDisplay = ""
	}
}

// render displays our shell state to the gocui view
func (s *BasicShell) render(v *gocui.View) {
	s.updateSuggestion()
	
	// Get view dimensions
	width, _ := v.Size()
	if width <= 0 {
		return
	}
	
	// Calculate scroll offset to keep cursor visible
	if s.cursorPos < s.scrollOffset {
		// Cursor moved left of visible area
		s.scrollOffset = s.cursorPos
	} else if s.cursorPos >= s.scrollOffset + width - 1 {
		// Cursor moved right of visible area (leave 1 char margin)
		s.scrollOffset = s.cursorPos - width + 2
		if s.scrollOffset < 0 {
			s.scrollOffset = 0
		}
	}
	
	// Clear and render visible portion of buffer
	v.Clear()
	
	// Calculate visible text
	visibleBuffer := s.buffer
	if s.scrollOffset > 0 && s.scrollOffset < len(visibleBuffer) {
		visibleBuffer = visibleBuffer[s.scrollOffset:]
	}
	
	// Truncate if still too long
	if len(visibleBuffer) > width {
		visibleBuffer = visibleBuffer[:width]
	}
	
	v.Write([]byte(visibleBuffer))
	
	// Add suggestion if at end of buffer and have room
	if s.postDisplay != "" && s.cursorPos == len(s.buffer) {
		remainingSpace := width - (s.cursorPos - s.scrollOffset)
		if remainingSpace > 0 {
			suggestionText := s.postDisplay
			if len(suggestionText) > remainingSpace {
				suggestionText = suggestionText[:remainingSpace]
			}
			v.Write([]byte("\x1b[2m" + suggestionText + "\x1b[0m"))
		}
	}
	
	// Set cursor position relative to scroll offset
	cursorX := s.cursorPos - s.scrollOffset
	if cursorX < 0 {
		cursorX = 0
	} else if cursorX >= width {
		cursorX = width - 1
	}
	v.SetCursor(cursorX, 0)
}

// GetInputBuffer returns the current content of the input buffer.
func (s *BasicShell) GetInputBuffer() string {
	return s.buffer
}

// SetInputBuffer sets the content of the input buffer and updates the view.
func (s *BasicShell) SetInputBuffer(content string, v *gocui.View) {
	s.buffer = content
	s.cursorPos = len(s.buffer)
	s.currentInput = content
	s.scrollOffset = 0 // Reset scroll when setting new content
	s.render(v)
}

// ClearInput clears the input buffer and resets the view.
func (s *BasicShell) ClearInput(v *gocui.View) {
	s.buffer = ""
	s.cursorPos = 0
	s.currentInput = ""
	s.scrollOffset = 0 // Reset scroll when clearing
	s.render(v)
}

// NavigateHistoryUp moves backward in history (towards older commands).
func (s *BasicShell) NavigateHistoryUp(v *gocui.View) {
	if !s.historyNavigating {
		s.currentInput = strings.TrimRight(v.Buffer(), "\n")
		s.historyNavigating = true
	}
	command := s.history.NavigatePrev()
	// Convert multiline commands to single line for display in input field
	// This preserves the original in history but shows a single-line version
	if strings.Contains(command, "\n") {
		// Replace newlines with spaces and clean up extra whitespace
		command = strings.ReplaceAll(command, "\n", " ")
		command = strings.Join(strings.Fields(command), " ")
	}
	s.buffer = command
	s.cursorPos = len(s.buffer)
	s.scrollOffset = 0 // Reset scroll when navigating history
	s.render(v)
}

// NavigateHistoryDown moves forward in history (towards newer commands).
func (s *BasicShell) NavigateHistoryDown(v *gocui.View) {
	command := s.history.NavigateNext()
	if command == "" && s.historyNavigating {
		command = s.currentInput
		s.historyNavigating = false
	}
	// Convert multiline commands to single line for display in input field
	if strings.Contains(command, "\n") {
		command = strings.ReplaceAll(command, "\n", " ")
		command = strings.Join(strings.Fields(command), " ")
	}
	s.buffer = command
	s.cursorPos = len(s.buffer)
	s.scrollOffset = 0 // Reset scroll when navigating history
	s.render(v)
}

// ResetHistoryNavigation resets the history navigation state.
func (s *BasicShell) ResetHistoryNavigation() {
	s.history.ResetNavigation()
	s.historyNavigating = false
	s.currentInput = ""
}

// triggerCompletion attempts to trigger and display a completion.
func (s *BasicShell) triggerCompletion(v *gocui.View) {
	suggestion := s.completer.Suggest(s.buffer)
	if suggestion != "" && strings.HasPrefix(suggestion, s.buffer) && suggestion != s.buffer {
		s.buffer = suggestion
		s.cursorPos = len(s.buffer)
		s.render(v)
	}
}

// applyCompletionIfPossible applies the current suggestion if available
func (s *BasicShell) applyCompletionIfPossible(v *gocui.View) bool {
	s.updateSuggestion()
	
	if s.postDisplay != "" {
		s.buffer = s.buffer + s.postDisplay
		s.cursorPos = len(s.buffer)
		s.postDisplay = ""
		s.render(v)
		return true
	}
	
	return false
}

// handleBackspace performs backspace operation on our shell abstraction
func (s *BasicShell) handleBackspace(v *gocui.View) {
	if len(s.buffer) > 0 && s.cursorPos > 0 {
		s.buffer = s.buffer[:s.cursorPos-1] + s.buffer[s.cursorPos:]
		s.cursorPos--
	}
	
	s.render(v)
}

// findPreviousWordBoundary finds the start of the previous word
func (s *BasicShell) findPreviousWordBoundary() int {
	if s.cursorPos == 0 || len(s.buffer) == 0 {
		return 0
	}
	
	pos := s.cursorPos - 1
	if pos >= len(s.buffer) {
		pos = len(s.buffer) - 1
	}
	
	for pos > 0 && pos < len(s.buffer) && s.isWhitespace(s.buffer[pos]) {
		pos--
	}
	
	for pos > 0 && pos < len(s.buffer) && !s.isWhitespace(s.buffer[pos]) {
		pos--
	}
	
	if pos > 0 && pos < len(s.buffer) && s.isWhitespace(s.buffer[pos]) {
		pos++
	}
	
	return pos
}

// findNextWordBoundary finds the start of the next word
func (s *BasicShell) findNextWordBoundary() int {
	if s.cursorPos >= len(s.buffer) {
		return len(s.buffer)
	}
	
	pos := s.cursorPos
	
	for pos < len(s.buffer) && !s.isWhitespace(s.buffer[pos]) {
		pos++
	}
	
	for pos < len(s.buffer) && s.isWhitespace(s.buffer[pos]) {
		pos++
	}
	
	return pos
}

// isWhitespace checks if a character is whitespace
func (s *BasicShell) isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// SetCursorToBeginning moves cursor to the beginning of the buffer
func (s *BasicShell) SetCursorToBeginning(v *gocui.View) {
	viewBuffer := strings.TrimRight(v.Buffer(), "\n")
	s.buffer = s.extractCleanBuffer(viewBuffer)
	s.cursorPos = 0
	s.render(v)
}

// SetCursorToEnd moves cursor to the end of the buffer
func (s *BasicShell) SetCursorToEnd(v *gocui.View) {
	viewBuffer := strings.TrimRight(v.Buffer(), "\n")
	s.buffer = s.extractCleanBuffer(viewBuffer)
	
	s.cursorPos = len(s.buffer)
	s.render(v)
}

// MoveToPreviousWord moves cursor to the previous word boundary
func (s *BasicShell) MoveToPreviousWord(v *gocui.View) {
	viewBuffer := strings.TrimRight(v.Buffer(), "\n")
	s.buffer = s.extractCleanBuffer(viewBuffer)
	
	s.cursorPos = s.findPreviousWordBoundary()
	s.render(v)
}

// MoveToNextWord moves cursor to the next word boundary
func (s *BasicShell) MoveToNextWord(v *gocui.View) {
	viewBuffer := strings.TrimRight(v.Buffer(), "\n")
	s.buffer = s.extractCleanBuffer(viewBuffer)
	
	s.cursorPos = s.findNextWordBoundary()
	s.render(v)
}

// MoveBackwardChar moves cursor backward one character (Ctrl+B)
func (s *BasicShell) MoveBackwardChar(v *gocui.View) {
	viewBuffer := strings.TrimRight(v.Buffer(), "\n")
	s.buffer = s.extractCleanBuffer(viewBuffer)
	
	if s.cursorPos > 0 {
		s.cursorPos--
	}
	s.render(v)
}

// DeleteWordBackward deletes the word before the cursor (Ctrl+W)
func (s *BasicShell) DeleteWordBackward(v *gocui.View) {
	viewBuffer := strings.TrimRight(v.Buffer(), "\n")
	s.buffer = s.extractCleanBuffer(viewBuffer)
	
	if s.cursorPos == 0 {
		return // Nothing to delete
	}
	
	startPos := s.findPreviousWordBoundary()
	
	s.buffer = s.buffer[:startPos] + s.buffer[s.cursorPos:]
	s.cursorPos = startPos
	
	s.render(v)
}

// extractCleanBuffer removes ANSI escape sequences from the buffer
func (s *BasicShell) extractCleanBuffer(buffer string) string {
	cleaned := buffer
	
	if ansiStart := strings.Index(cleaned, "\x1b[2m"); ansiStart >= 0 {
		cleaned = cleaned[:ansiStart]
	}
	
	return cleaned
}


// IsUnboundSpecialKey checks if a key is a special key that should be ignored.
func IsUnboundSpecialKey(key gocui.Key) bool {
	switch key {
	case gocui.KeyF1, gocui.KeyF2, gocui.KeyF3, gocui.KeyF4,
		gocui.KeyF5, gocui.KeyF6, gocui.KeyF7, gocui.KeyF8,
		gocui.KeyF9, gocui.KeyF10, gocui.KeyF11, gocui.KeyF12:
		return true
	case gocui.KeyPgup, gocui.KeyPgdn:
		return true
	case gocui.KeyHome, gocui.KeyEnd:
		return true
	case gocui.KeyInsert:
		return false
	default:
		if key < 0 {
			switch key {
			case gocui.KeySpace, gocui.KeyBackspace, gocui.KeyBackspace2,
				gocui.KeyEnter, gocui.KeyArrowDown, gocui.KeyArrowUp,
				gocui.KeyArrowLeft, gocui.KeyArrowRight, gocui.KeyDelete,
				gocui.KeyTab, gocui.KeyEsc, gocui.KeyCtrlA, gocui.KeyCtrlB,
				gocui.KeyCtrlD, gocui.KeyCtrlE, gocui.KeyCtrlF, gocui.KeyCtrlG,
				gocui.KeyCtrlJ, gocui.KeyCtrlK, gocui.KeyCtrlL, gocui.KeyCtrlN,
				gocui.KeyCtrlO, gocui.KeyCtrlP, gocui.KeyCtrlQ, gocui.KeyCtrlR,
				gocui.KeyCtrlS, gocui.KeyCtrlT, gocui.KeyCtrlU, gocui.KeyCtrlV,
				gocui.KeyCtrlW, gocui.KeyCtrlX, gocui.KeyCtrlY, gocui.KeyCtrlZ,
				gocui.KeyCtrlUnderscore, gocui.KeyCtrlSpace, gocui.KeyCtrlBackslash,
				gocui.KeyCtrlRsqBracket:
				return false
			default:
				return true
			}
		}
		return false
	}
}
