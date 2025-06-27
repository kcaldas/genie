package tui2

import (
	"fmt"

	"github.com/awesome-gocui/gocui"
)

// navigateHistoryUp moves to older commands in history
func (t *TUI) navigateHistoryUp(g *gocui.Gui, v *gocui.View) error {
	command := t.chatHistory.NavigatePrev()
	t.addDebugMessage(fmt.Sprintf("History navigation: up to '%s'", command))
	return t.setInputText(v, command)
}

// navigateHistoryDown moves to newer commands in history
func (t *TUI) navigateHistoryDown(g *gocui.Gui, v *gocui.View) error {
	command := t.chatHistory.NavigateNext()
	t.addDebugMessage(fmt.Sprintf("History navigation: down to '%s'", command))
	return t.setInputText(v, command)
}

// setInputText sets the text in the input view and positions cursor
func (t *TUI) setInputText(v *gocui.View, text string) error {
	v.Clear()
	v.SetCursor(0, 0)
	fmt.Fprint(v, text)
	
	// Move cursor to end of text
	if text != "" {
		v.SetCursor(len(text), 0)
	}
	
	return nil
}