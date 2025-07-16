package component

import (
	"github.com/awesome-gocui/gocui"
	"strings"
	"unicode"
)

// ViMode represents the current mode of the Vi editor.
type ViMode int

const (
	NormalMode ViMode = iota
	InsertMode
	CommandMode
)

// ViEditor implements the gocui.Editor interface to provide vi-like editing.
type ViEditor struct {
	mode           ViMode
	pendingCommand rune               // For commands that need a second key (like dd, d$, c$, etc.)
	commandBuffer  string             // Buffer for command-line mode commands
	onCommand      func(string) error // Callback for executing commands
	onModeChange   func()             // Callback for when mode changes
	pendingG       bool               // Track if we're waiting for second 'g' in 'gg' command
}

// NewViEditor creates a new instance of ViEditor.
func NewViEditor() gocui.Editor {
	return &ViEditor{
		mode: NormalMode,
	}
}

// Edit handles the editor's behavior for input based on the current vi mode.
func (e *ViEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch e.mode {
	case NormalMode:
		e.handleNormalMode(v, key, ch, mod)
	case InsertMode:
		e.handleInsertMode(v, key, ch, mod)
	case CommandMode:
		e.handleCommandMode(v, key, ch, mod)
	}
}

// handleNormalMode processes key presses in Normal mode.
func (e *ViEditor) handleNormalMode(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	cx, cy := v.Cursor()
	line, _ := v.Line(cy)

	// Get all lines from the view for multi-line navigation
	var allLines []string
	buf := v.Buffer()
	if buf != "" {
		allLines = strings.Split(buf, "\n")
	}

	maxX, maxY := v.Size()

	switch key {
	case gocui.KeyEsc:
		// Cancel pending command and pending g
		e.pendingCommand = 0
		e.pendingG = false
	case gocui.KeyCtrlC:
		// Cancel pending command and pending g
		e.pendingCommand = 0
		e.pendingG = false
	case gocui.KeyEnter:
		v.SetCursor(0, cy+1)
	default:
		switch ch {
		case 'i':
			e.setMode(InsertMode)
		case 'a':
			e.setMode(InsertMode)
			if cx < len(line) {
				v.SetCursor(cx+1, cy)
			}
		case 'A':
			e.setMode(InsertMode)
			// Move to end of line
			v.SetCursor(len(line), cy)
		case 'o':
			e.setMode(InsertMode)
			gocui.DefaultEditor.Edit(v, gocui.KeyEnter, 0, gocui.ModNone)
			_, cy := v.Cursor()
			v.SetCursor(0, cy)
		case 'O':
			e.setMode(InsertMode)
			_, cy := v.Cursor()
			v.SetCursor(0, cy)
			gocui.DefaultEditor.Edit(v, gocui.KeyEnter, 0, gocui.ModNone)
			v.SetCursor(0, cy)
		case 'h':
			if cx > 0 {
				v.SetCursor(cx-1, cy)
			}
		case 'l':
			if cx < len(line) {
				v.SetCursor(cx+1, cy)
			}
		case 'j':
			v.SetCursor(cx, cy+1)
			// Ensure cursor stays visible when moving down
			e.ensureCursorVisible(v)
		case 'k':
			if cy > 0 {
				v.SetCursor(cx, cy-1)
				// Ensure cursor stays visible when moving up
				e.ensureCursorVisible(v)
			}
		case 'w':
			newCx, newCy := findNextWordStart(line, cx, cy, allLines, maxX, maxY)
			v.SetCursor(newCx, newCy)
		case 'W':
			newCx, newCy := findNextWORDStart(line, cx, cy, allLines, maxX, maxY)
			v.SetCursor(newCx, newCy)
		case 'b':
			newCx, newCy := findPrevWordStart(line, cx, cy, allLines, maxX, maxY)
			v.SetCursor(newCx, newCy)
		case 'B':
			newCx, newCy := findPrevWORDStart(line, cx, cy, allLines, maxX, maxY)
			v.SetCursor(newCx, newCy)
		case 'x':
			gocui.DefaultEditor.Edit(v, gocui.KeyDelete, 0, gocui.ModNone)
		case 'd':
			if e.pendingCommand == 'd' {
				// dd - delete entire line
				e.deleteLine(v)
				e.pendingCommand = 0
			} else {
				// First d, wait for next character
				e.pendingCommand = 'd'
			}
		case 'c':
			// First c, wait for next character
			e.pendingCommand = 'c'
		case 'u':
			// Undo placeholder
		case ':':
			e.setMode(CommandMode)
			e.commandBuffer = ""
		case 'g':
			if e.pendingG {
				// gg - go to top of file
				v.SetCursor(0, 0)
				// Scroll to top to make cursor visible
				v.SetOrigin(0, 0)
				e.pendingG = false
			} else {
				// First 'g', wait for second 'g'
				e.pendingG = true
			}
		case 'G':
			// G - go to bottom of file
			e.goToBottom(v)
			e.pendingG = false
		case '$':
			if e.pendingCommand == 'd' {
				// d$ - delete to end of line
				e.deleteToEndOfLine(v)
				e.pendingCommand = 0
			} else if e.pendingCommand == 'c' {
				// c$ - change to end of line
				e.changeToEndOfLine(v)
				e.pendingCommand = 0
			} else {
				// Regular $ - move to end of line
				line, _ := v.Line(cy)
				if len(line) > 0 {
					v.SetCursor(len(line)-1, cy)
				}
			}
		case '0':
			if e.pendingCommand == 'd' {
				// d0 - delete to beginning of line
				e.deleteToBeginningOfLine(v)
				e.pendingCommand = 0
			} else if e.pendingCommand == 'c' {
				// c0 - change to beginning of line
				e.changeToBeginningOfLine(v)
				e.pendingCommand = 0
			} else {
				// Regular 0 - move to beginning of line
				v.SetCursor(0, cy)
			}
		default:
			// Cancel pending command and pending g if unrecognized key is pressed
			if e.pendingCommand != 0 {
				e.pendingCommand = 0
			}
			if e.pendingG {
				e.pendingG = false
			}
		}
	}
}

// handleInsertMode processes key presses in Insert mode.
func (e *ViEditor) handleInsertMode(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if key == gocui.KeyEsc {
		e.setMode(NormalMode)
		cx, cy := v.Cursor()
		if cx > 0 {
			v.SetCursor(cx-1, cy)
		}
		return
	}
	gocui.DefaultEditor.Edit(v, key, ch, mod)
}

// Helper functions for word navigation

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// Helper functions for delete and change operations

func (e *ViEditor) deleteLine(v *gocui.View) {
	_, cy := v.Cursor()
	line, _ := v.Line(cy)

	// Delete entire line content
	for range line {
		v.SetCursor(0, cy)
		gocui.DefaultEditor.Edit(v, gocui.KeyDelete, 0, gocui.ModNone)
	}

	// Delete the newline character to remove the line entirely
	gocui.DefaultEditor.Edit(v, gocui.KeyDelete, 0, gocui.ModNone)

	// Position cursor at beginning of current line
	v.SetCursor(0, cy)
}

func (e *ViEditor) deleteToEndOfLine(v *gocui.View) {
	cx, cy := v.Cursor()
	line, _ := v.Line(cy)

	// Delete from current position to end of line
	for i := cx; i < len(line); i++ {
		gocui.DefaultEditor.Edit(v, gocui.KeyDelete, 0, gocui.ModNone)
	}
}

func (e *ViEditor) deleteToBeginningOfLine(v *gocui.View) {
	cx, cy := v.Cursor()

	// Move cursor to beginning of line and delete to original position
	v.SetCursor(0, cy)
	for i := 0; i < cx; i++ {
		gocui.DefaultEditor.Edit(v, gocui.KeyDelete, 0, gocui.ModNone)
	}
	v.SetCursor(0, cy)
}

func (e *ViEditor) changeToEndOfLine(v *gocui.View) {
	e.deleteToEndOfLine(v)
	e.setMode(InsertMode)
}

func (e *ViEditor) changeToBeginningOfLine(v *gocui.View) {
	e.deleteToBeginningOfLine(v)
	e.setMode(InsertMode)
}

func isWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}

func findNextWordStart(currentLine string, cx, cy int, allLines []string, viewSizeX, viewSizeY int) (int, int) {
	line := currentLine

	for i := cx; i < len(line); i++ {
		if !isWordChar(rune(line[i])) && !isWhitespace(rune(line[i])) {
			cx++
		} else if isWhitespace(rune(line[i])) {
			cx++
		} else {
			break
		}
	}

	for i := cx; i < len(line); i++ {
		if isWordChar(rune(line[i])) {
			cx++
		} else {
			break
		}
	}

	for i := cx; i < len(line); i++ {
		if isWhitespace(rune(line[i])) {
			cx++
		} else {
			break
		}
	}

	if cx >= len(line) {
		for newCy := cy + 1; newCy < len(allLines); newCy++ {
			nextLine := allLines[newCy]
			if nextLine != "" {
				for i, r := range nextLine {
					if !isWhitespace(r) {
						return i, newCy
					}
				}
				return 0, newCy
			} else {
				return 0, newCy
			}
		}
		return viewSizeX, cy
	}

	return cx, cy
}

func findNextWORDStart(currentLine string, cx, cy int, allLines []string, viewSizeX, viewSizeY int) (int, int) {
	line := currentLine

	for i := cx; i < len(line); i++ {
		if !isWhitespace(rune(line[i])) {
			cx++
		} else {
			break
		}
	}

	for i := cx; i < len(line); i++ {
		if isWhitespace(rune(line[i])) {
			cx++
		} else {
			break
		}
	}

	if cx >= len(line) {
		for newCy := cy + 1; newCy < len(allLines); newCy++ {
			nextLine := allLines[newCy]
			if nextLine != "" {
				for i, r := range nextLine {
					if !isWhitespace(r) {
						return i, newCy
					}
				}
				return 0, newCy
			} else {
				return 0, newCy
			}
		}
		return viewSizeX, cy
	}

	return cx, cy
}

func findPrevWordStart(currentLine string, cx, cy int, allLines []string, viewSizeX, viewSizeY int) (int, int) {
	line := currentLine
	for {
		if len(line) == 0 && cy > 0 {
			cy--
			line = allLines[cy]
			cx = len(line)
			continue
		}

		if cx == 0 && cy == 0 {
			return 0, 0
		}

		if cx == 0 {
			cy--
			if cy < 0 {
				return 0, 0
			}
			line = allLines[cy]
			cx = len(line)
		}

		cx--

		for cx >= 0 && isWhitespace(rune(line[cx])) {
			cx--
		}

		if cx < 0 {
			continue
		}

		if isWordChar(rune(line[cx])) {
			for cx >= 0 && isWordChar(rune(line[cx])) {
				cx--
			}
			return cx + 1, cy
		} else {
			for cx >= 0 && !isWordChar(rune(line[cx])) && !isWhitespace(rune(line[cx])) {
				cx--
			}
			return cx + 1, cy
		}
	}
}

func findPrevWORDStart(currentLine string, cx, cy int, allLines []string, viewSizeX, viewSizeY int) (int, int) {
	line := currentLine
	for {
		if len(line) == 0 && cy > 0 {
			cy--
			line = allLines[cy]
			cx = len(line)
			continue
		}

		if cx == 0 && cy == 0 {
			return 0, 0
		}

		if cx == 0 {
			cy--
			if cy < 0 {
				return 0, 0
			}
			line = allLines[cy]
			cx = len(line)
		}

		cx--

		for cx >= 0 && isWhitespace(rune(line[cx])) {
			cx--
		}

		if cx < 0 {
			continue
		}

		for cx >= 0 && !isWhitespace(rune(line[cx])) {
			cx--
		}
		return cx + 1, cy
	}
}

// handleCommandMode processes key presses in Command mode (after pressing ':')
func (e *ViEditor) handleCommandMode(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch key {
	case gocui.KeyEsc:
		// Cancel command mode
		e.setMode(NormalMode)
		e.commandBuffer = ""
	case gocui.KeyEnter:
		// Execute command
		if e.onCommand != nil {
			e.onCommand(e.commandBuffer)
		}
		e.setMode(NormalMode)
		e.commandBuffer = ""
	case gocui.KeyBackspace, gocui.KeyBackspace2:
		// Remove last character from command buffer
		if len(e.commandBuffer) > 0 {
			e.commandBuffer = e.commandBuffer[:len(e.commandBuffer)-1]
			if e.onModeChange != nil {
				e.onModeChange()
			}
		}
	default:
		// Add character to command buffer
		if ch != 0 {
			e.commandBuffer += string(ch)
			if e.onModeChange != nil {
				e.onModeChange()
			}
		}
	}
}

// SetCommandHandler sets the callback function for handling vim commands
func (e *ViEditor) SetCommandHandler(handler func(string) error) {
	e.onCommand = handler
}

// GetCommandBuffer returns the current command buffer (for displaying command line)
func (e *ViEditor) GetCommandBuffer() string {
	return e.commandBuffer
}

// GetMode returns the current vim mode
func (e *ViEditor) GetMode() ViMode {
	return e.mode
}

// SetModeChangeHandler sets the callback function for mode changes
func (e *ViEditor) SetModeChangeHandler(handler func()) {
	e.onModeChange = handler
}

// setMode sets the mode and triggers the callback
func (e *ViEditor) setMode(mode ViMode) {
	e.mode = mode
	if e.onModeChange != nil {
		e.onModeChange()
	}
}

// goToBottom moves cursor to the bottom of the file
func (e *ViEditor) goToBottom(v *gocui.View) {
	buf := v.Buffer()
	if buf == "" {
		// Empty buffer, stay at 0,0
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
		return
	}

	// Count lines in buffer
	lines := strings.Split(buf, "\n")
	lastLineIndex := len(lines) - 1

	// Handle empty last line (common when buffer ends with newline)
	if lastLineIndex > 0 && lines[lastLineIndex] == "" {
		lastLineIndex--
	}

	// Move cursor to beginning of last line
	v.SetCursor(0, lastLineIndex)

	// Scroll view to make the bottom visible
	// Get view dimensions to calculate proper scroll position
	_, viewHeight := v.Size()
	if viewHeight > 0 {
		// Calculate the origin Y position to show the bottom
		// We want the last line to be visible, so scroll to show it
		originY := lastLineIndex - viewHeight + 1
		if originY < 0 {
			originY = 0
		}
		v.SetOrigin(0, originY)
	}
}

// ensureCursorVisible adjusts the view origin to keep the cursor visible
func (e *ViEditor) ensureCursorVisible(v *gocui.View) {
	_, cy := v.Cursor()
	ox, oy := v.Origin()
	_, viewHeight := v.Size()

	// Adjust vertical scrolling
	if cy < oy {
		// Cursor is above the view, scroll up
		v.SetOrigin(ox, cy)
	} else if cy >= oy+viewHeight {
		// Cursor is below the view, scroll down
		v.SetOrigin(ox, cy-viewHeight+1)
	}
}
