package tui2

import (
	"fmt"
	"time"

	"github.com/awesome-gocui/gocui"
)

// toggleDebugPanel toggles the debug panel visibility
func (t *TUI) toggleDebugPanel(g *gocui.Gui, v *gocui.View) error {
	// Toggle the state
	t.showDebug = !t.showDebug
	
	// Add a debug message to show the toggle action
	action := "shown"
	if !t.showDebug {
		action = "hidden"
	}
	t.addDebugMessage(fmt.Sprintf("Debug panel %s", action))
	
	return nil
}

// addDebugMessage adds a message to the debug panel
func (t *TUI) addDebugMessage(message string) {
	timestamp := time.Now().Format("15:04:05")
	debugEntry := fmt.Sprintf("[%s] %s", timestamp, message)
	t.debugMessages = append(t.debugMessages, debugEntry)
	
	// Keep only the last 100 debug messages to prevent memory issues
	if len(t.debugMessages) > 100 {
		t.debugMessages = t.debugMessages[len(t.debugMessages)-100:]
	}
	
	// Update debug view if it's currently showing
	if t.showDebug {
		if v, err := t.g.View(viewDebug); err == nil {
			t.renderDebugMessages(v)
		}
	}
}

// renderDebugMessages renders all debug messages to the debug view
func (t *TUI) renderDebugMessages(v *gocui.View) {
	v.Clear()
	
	for _, debugMsg := range t.debugMessages {
		// Use a dim gray color for debug messages
		fmt.Fprintf(v, "\033[90m%s\033[0m\n", debugMsg)
	}
}