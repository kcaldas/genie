package tui2

import (
	"fmt"
	"time"

	"github.com/awesome-gocui/gocui"
)

// layout defines the UI layout
func (t *TUI) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	
	// Calculate layout dimensions based on debug panel state
	var messagesMaxX int
	if t.showDebug {
		messagesMaxX = (maxX / 2) - 1 // Split screen: messages take left half
	} else {
		messagesMaxX = maxX - 1 // Full width when debug is hidden
	}
	
	// Messages view (main area)
	if v, err := g.SetView(viewMessages, 0, 0, messagesMaxX, maxY-4, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Autoscroll = true
		v.Wrap = true
		
		// Update markdown renderer width when view size changes
		viewWidth, _ := v.Size()
		if viewWidth > 0 {
			t.markdownRenderer.UpdateWidth(viewWidth - 2) // Account for borders
		}
		
		// Render all messages
		t.renderMessages(v)
	}
	
	// Update messages view focus styling
	t.updateViewFocus(g, viewMessages)
	
	// Debug panel (right side when enabled)
	if t.showDebug {
		debugX := (maxX / 2) + 1
		if v, err := g.SetView(viewDebug, debugX, 0, maxX-1, maxY-4, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Autoscroll = true
			v.Wrap = true
			
			// Render debug messages
			t.renderDebugMessages(v)
		}
		
		// Update debug view focus styling
		t.updateViewFocus(g, viewDebug)
	} else {
		// Remove debug view if it exists
		g.DeleteView(viewDebug)
	}
	
	// Status bar (shows loading state AND notifications)
	if v, err := g.SetView(viewStatus, 0, maxY-4, maxX-1, maxY-3, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		t.renderStatus(v)
	}
	
	// Input view (back to original)
	if v, err := g.SetView(viewInput, 0, maxY-3, maxX-1, maxY-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = true
		v.Wrap = false
		
		// Set input as current view if no dialog is showing
		if !t.showDialog && t.focusManager.GetCurrentFocus() == FocusInput {
			g.SetCurrentView(viewInput)
		}
	}
	
	// Update input view focus styling
	t.updateViewFocus(g, viewInput)
	
	// Notification panel (overlay on input when needed)
	if t.notificationText != "" && time.Since(t.notificationTime) < 3*time.Second {
		if v, err := g.SetView(viewNotification, 0, maxY-3, maxX-1, maxY-1, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Frame = true  // Border makes it visible in your terminal
			t.renderNotification(v)
		}
	} else {
		// Remove notification view when not needed
		g.DeleteView(viewNotification)
	}
	
	// Dialog overlay (only when needed)
	if t.showDialog {
		dialogWidth := 50
		dialogHeight := 8
		dialogX := (maxX - dialogWidth) / 2
		dialogY := (maxY - dialogHeight) / 2
		
		if v, err := g.SetView(viewDialog, dialogX, dialogY, dialogX+dialogWidth, dialogY+dialogHeight, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = fmt.Sprintf(" %s ", t.dialogTitle)
			v.Wrap = true
			
			// Render dialog content
			fmt.Fprintln(v, t.dialogMessage)
			fmt.Fprintln(v, "")
			fmt.Fprintln(v, "Press 'y' for Yes, 'n' or ESC for No")
			
			// Make dialog the current view
			g.SetCurrentView(viewDialog)
		}
	} else {
		// Remove dialog if it exists
		g.DeleteView(viewDialog)
	}
	
	// Help overlay (only when needed)
	if t.showHelp {
		helpWidth := 70
		helpHeight := 25
		helpX := (maxX - helpWidth) / 2
		helpY := (maxY - helpHeight) / 2
		
		if v, err := g.SetView(viewHelp, helpX, helpY, helpX+helpWidth, helpY+helpHeight, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Help - Genie Shortcuts "
			v.Wrap = true
			
			// Render help content
			t.renderHelpContent(v)
			
			// Make help the current view
			g.SetCurrentView(viewHelp)
		}
	} else {
		// Remove help if it exists
		g.DeleteView(viewHelp)
	}
	
	return nil
}

// updateViewFocus updates a view's appearance based on focus state
func (t *TUI) updateViewFocus(g *gocui.Gui, viewName string) {
	if v, err := g.View(viewName); err == nil {
		isFocused := t.focusManager.GetCurrentFocus() == FocusablePanel(viewName)
		currentTheme := t.themeManager.GetCurrentTheme()
		
		// Update title and highlight based on focus using theme
		switch viewName {
		case viewMessages:
			v.Title = " Messages "
			if isFocused {
				v.Frame = true // Always show frame when focused
				t.themeManager.ApplyTheme(v, ElementFocused)
			} else {
				if currentTheme.IsMinimalTheme {
					v.Frame = false // Hide border entirely for minimal theme
					// Explicitly clear background for minimal theme
					v.BgColor = gocui.Attribute(0)
					v.FgColor = gocui.ColorDefault
				} else {
					v.Frame = true
					t.themeManager.ApplyTheme(v, ElementDefault)
				}
			}
		case viewInput:
			v.Title = " Input "
			if isFocused {
				v.Frame = true // Always show frame when focused
				t.themeManager.ApplyTheme(v, ElementFocused)
			} else {
				if currentTheme.IsMinimalTheme {
					v.Frame = false // Hide border entirely for minimal theme
				} else {
					v.Frame = true
					t.themeManager.ApplyTheme(v, ElementDefault)
				}
			}
		case viewDebug:
			v.Title = " Debug (F12 to hide) "
			if isFocused {
				v.Frame = true // Always show frame when focused
				t.themeManager.ApplyTheme(v, ElementFocused)
			} else {
				if currentTheme.IsMinimalTheme {
					v.Frame = false // Hide border entirely for minimal theme
					// Explicitly clear background for minimal theme
					v.BgColor = gocui.Attribute(0)
					v.FgColor = gocui.ColorDefault
				} else {
					v.Frame = true
					t.themeManager.ApplyTheme(v, ElementDefault)
				}
			}
		}
	}
}

// renderHelpContent renders the help panel content
func (t *TUI) renderHelpContent(v *gocui.View) {
	v.Clear()
	
	fmt.Fprintln(v, "\033[1m\033[36mCommands:\033[0m")
	fmt.Fprintln(v, "  /help            - Show this help panel")
	fmt.Fprintln(v, "  F1               - Toggle this help panel")
	fmt.Fprintln(v, "  /clear           - Clear chat messages")
	fmt.Fprintln(v, "  /debug           - Toggle debug panel")
	fmt.Fprintln(v, "  F12              - Toggle debug panel")
	fmt.Fprintln(v, "  /renderer [type] [theme] - Switch markdown renderer")
	fmt.Fprintln(v, "  /theme [name]    - Switch UI theme")
	fmt.Fprintln(v, "  /exit            - Exit application")
	fmt.Fprintln(v, "")
	
	fmt.Fprintln(v, "\033[1m\033[33mNavigation:\033[0m")
	fmt.Fprintln(v, "  Tab              - Cycle between panels")
	fmt.Fprintln(v, "  PgUp/PgDn        - Scroll focused panel")
	fmt.Fprintln(v, "  Ctrl+U/Ctrl+D    - Half-page scroll")
	fmt.Fprintln(v, "  Ctrl+B/Ctrl+F    - Page scroll (vi-style)")
	fmt.Fprintln(v, "  Home/End         - Jump to top/bottom")
	fmt.Fprintln(v, "  ESC              - Cancel current request")
	fmt.Fprintln(v, "")
	
	fmt.Fprintln(v, "\033[1m\033[32mClipboard:\033[0m")
	fmt.Fprintln(v, "  Alt+C            - Copy focused view to system clipboard")
	fmt.Fprintln(v, "  Ctrl+Y           - Copy (yank) current input")
	fmt.Fprintln(v, "  Ctrl+P           - Paste at cursor position")
	fmt.Fprintln(v, "  Ctrl+A           - Select all and copy")
	fmt.Fprintln(v, "")
	
	fmt.Fprintln(v, "\033[1m\033[35mFocus Indicators:\033[0m")
	fmt.Fprintln(v, "  Yellow border    - Currently focused panel")
	fmt.Fprintln(v, "  Default border   - Unfocused panels")
	fmt.Fprintln(v, "")
	
	fmt.Fprintln(v, "\033[1m\033[31mUI & Markdown Themes:\033[0m")
	fmt.Fprintln(v, "  /theme dark              - Dark UI theme")
	fmt.Fprintln(v, "  /theme dracula           - Dracula color scheme")
	fmt.Fprintln(v, "  /theme light             - Light UI theme")
	fmt.Fprintln(v, "  /renderer glamour [theme] - Rich markdown")
	fmt.Fprintln(v, "  /renderer plaintext       - Plain text")
	fmt.Fprintln(v, "")
	fmt.Fprintln(v, "  Glamour themes:")
	fmt.Fprintln(v, "  auto, dark, light, dracula, tokyo-night, notty")
	fmt.Fprintln(v, "")
	
	fmt.Fprintln(v, "\033[1m\033[37mHelp Panel Controls:\033[0m")
	fmt.Fprintln(v, "  ESC or 'q'       - Close this help panel")
	fmt.Fprintln(v, "  F1               - Toggle this help panel")
}