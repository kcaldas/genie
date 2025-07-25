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

// Edit handles the editor's behavior for input.
func (e *CustomEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	ox, oy := v.Origin()
	cx, cy := v.Cursor()


	if mod&gocui.Modifier(tcell.ModCtrl) != 0 {
		switch key {
		case gocui.KeyArrowLeft:
			v.SetCursor(0, cy)
			return
		case gocui.KeyArrowRight:
			line, _ := v.Line(cy)
			v.SetCursor(len(line), cy)
			return
		}
	}

	if mod&gocui.Modifier(tcell.ModAlt) != 0 {
		switch key {
		case gocui.KeyArrowLeft:
			v.SetCursor(0, cy)
			return
		case gocui.KeyArrowRight:
			line, _ := v.Line(cy)
			v.SetCursor(len(line), cy)
			return
		}
	}

	switch key {
	case gocui.KeyArrowDown:
		line, _ := v.Line(cy + 1)
		if line != "" {
			v.SetCursor(cx, cy+1)
		} else {
			_, maxY := v.Size()
			if cy < oy+maxY-1 {
				v.SetCursor(cx, cy+1)
			} else {
				v.SetOrigin(ox, oy+1)
			}
		}
	case gocui.KeyArrowUp:
		if cy > 0 || oy > 0 {
			if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
				v.SetOrigin(ox, oy-1)
			}
		}
	case gocui.KeyArrowLeft:
		if cx > 0 {
			v.SetCursor(cx-1, cy)
		} else if cy > 0 {
			line, _ := v.Line(cy - 1)
			v.SetCursor(len(line), cy-1)
		}
	case gocui.KeyArrowRight:
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
		if IsUnboundSpecialKey(key) {
			return
		}
		if ch != 0 || (key != 0 && !IsUnboundSpecialKey(key)) {
			gocui.DefaultEditor.Edit(v, key, ch, mod)
		}
	}
}
