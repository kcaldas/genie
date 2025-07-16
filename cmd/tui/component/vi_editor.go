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
	VisualMode
)

// ViEditor implements the gocui.Editor interface to provide vi-like editing.
type ViEditor struct {
	mode ViMode
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
	case VisualMode:
		e.handleVisualMode(v, key, ch, mod)
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
		// No-op in normal mode
	case gocui.KeyCtrlC:
		// No-op in normal mode
	case gocui.KeyEnter:
		v.SetCursor(0, cy+1)
	default:
		switch ch {
		case 'i':
			e.mode = InsertMode
		case 'a':
			e.mode = InsertMode
			if cx < len(line) {
				v.SetCursor(cx+1, cy)
			}
		case 'o':
			e.mode = InsertMode
			gocui.DefaultEditor.Edit(v, gocui.KeyEnter, 0, gocui.ModNone)
			_, cy := v.Cursor()
			v.SetCursor(0, cy)
		case 'O':
			e.mode = InsertMode
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
		case 'k':
			if cy > 0 {
				v.SetCursor(cx, cy-1)
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
			// Delete command placeholder
		case 'u':
			// Undo placeholder
		case 'v':
			e.mode = VisualMode
		}
	}
}

// handleInsertMode processes key presses in Insert mode.
func (e *ViEditor) handleInsertMode(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if key == gocui.KeyEsc {
		e.mode = NormalMode
		cx, cy := v.Cursor()
		if cx > 0 {
			v.SetCursor(cx-1, cy)
		}
		return
	}
	gocui.DefaultEditor.Edit(v, key, ch, mod)
}

// handleVisualMode processes key presses in Visual mode.
func (e *ViEditor) handleVisualMode(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if key == gocui.KeyEsc {
		e.mode = NormalMode
		return
	}
	switch ch {
	case 'h':
		cx, cy := v.Cursor()
		if cx > 0 {
			v.SetCursor(cx-1, cy)
		}
	case 'l':
		cx, cy := v.Cursor()
		line, _ := v.Line(cy)
		if cx < len(line) {
			v.SetCursor(cx+1, cy)
		}
	case 'j':
		cx, cy := v.Cursor()
		v.SetCursor(cx, cy+1)
	case 'k':
		cx, cy := v.Cursor()
		if cy > 0 {
			v.SetCursor(cx, cy-1)
		}
	}
}

// Helper functions for word navigation

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
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
