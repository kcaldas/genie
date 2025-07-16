package component

import (
	"github.com/awesome-gocui/gocui"
	"github.com/gdamore/tcell/v2"
)

// CustomEditor implements the gocui.Editor interface to provide extended navigation.
type CustomEditor struct{}

// NewCustomEditor creates a new instance of CustomEditor.
func NewCustomEditor() gocui.Editor {
	return &CustomEditor{}
}

// Edit handles the editor's behavior for input.
func (e *CustomEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	ox, oy := v.Origin()
	cx, cy := v.Cursor()

	// Debug: Log what keys are being received
	// TODO: Remove this debug logging once we figure out the key mappings
	if key == gocui.KeyArrowLeft || key == gocui.KeyArrowRight || key == gocui.KeyArrowUp || key == gocui.KeyArrowDown {
		// Simple debug - just check if we're getting arrow keys at all
	}

	// Handle Ctrl+Arrow key combinations first (may not work on macOS due to system shortcuts)
	if mod&gocui.Modifier(tcell.ModCtrl) != 0 {
		switch key {
		case gocui.KeyArrowLeft:
			// Ctrl+Left: Move cursor to the beginning of the current line
			v.SetCursor(0, cy)
			return
		case gocui.KeyArrowRight:
			// Ctrl+Right: Move cursor to the end of the current line
			line, _ := v.Line(cy)
			v.SetCursor(len(line), cy)
			return
		}
	}

	// Handle Cmd+Arrow (Option+Arrow may work better on macOS)
	if mod&gocui.Modifier(tcell.ModAlt) != 0 {
		switch key {
		case gocui.KeyArrowLeft:
			// Alt+Left: Move cursor to the beginning of the current line
			v.SetCursor(0, cy)
			return
		case gocui.KeyArrowRight:
			// Alt+Right: Move cursor to the end of the current line
			line, _ := v.Line(cy)
			v.SetCursor(len(line), cy)
			return
		}
	}

	switch key {
	case gocui.KeyArrowDown:
		// Move cursor down, adjust origin if at the bottom of the view
		line, _ := v.Line(cy + 1)
		if line != "" {
			v.SetCursor(cx, cy+1)
		} else {
			// If there is no next line, try to move origin to show more content
			_, maxY := v.Size() // Use maxY from v.Size()
			if cy < oy+maxY-1 {
				v.SetCursor(cx, cy+1)
			} else {
				v.SetOrigin(ox, oy+1)
			}
		}
	case gocui.KeyArrowUp:
		// Move cursor up, adjust origin if at the top of the view
		if cy > 0 || oy > 0 {
			if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
				v.SetOrigin(ox, oy-1)
			}
		}
	case gocui.KeyArrowLeft:
		// Move cursor left, wrap to previous line if at beginning of line
		if cx > 0 {
			v.SetCursor(cx-1, cy)
		} else if cy > 0 {
			line, _ := v.Line(cy - 1)
			v.SetCursor(len(line), cy-1)
		}
	case gocui.KeyArrowRight:
		// Move cursor right, wrap to next line if at end of line
		line, _ := v.Line(cy)
		if cx < len(line) {
			v.SetCursor(cx+1, cy)
		} else {
			line, _ := v.Line(cy + 1)
			if line != "" {
				v.SetCursor(0, cy+1)
			}
		}
	default:
		// Default gocui editor behavior for character input and other keys
		gocui.DefaultEditor.Edit(v, key, ch, mod)
	}
}
