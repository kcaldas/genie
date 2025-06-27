package tui2

import (
	"github.com/awesome-gocui/gocui"
)

// MiniLayoutManager uses the mini UI system
type MiniLayoutManager struct {
	tui  *TUI
	root Component
}

// NewMiniLayoutManager creates a layout using the mini UI system
func NewMiniLayoutManager(tui *TUI) *MiniLayoutManager {
	mlm := &MiniLayoutManager{tui: tui}
	mlm.buildLayout()
	return mlm
}

// buildLayout constructs the UI tree
func (mlm *MiniLayoutManager) buildLayout() {
	// Create views with the same setup as the original layout
	messages := NewView("messages").
		WithSetup(func(v *gocui.View) error {
			v.Autoscroll = true
			v.Wrap = true
			v.Title = " Messages "
			
			// Update markdown renderer width when view size changes
			viewWidth, _ := v.Size()
			if viewWidth > 0 {
				mlm.tui.markdownRenderer.UpdateWidth(viewWidth - 2) // Account for borders
			}
			
			return nil
		}).
		WithRender(func(v *gocui.View) error {
			mlm.tui.renderMessages(v)
			mlm.tui.updateViewFocus(mlm.tui.g, "messages")
			return nil
		})

	debug := NewView("debug").
		WithSetup(func(v *gocui.View) error {
			v.Autoscroll = true
			v.Wrap = true
			v.Title = " Debug (F12 to hide) "
			return nil
		}).
		WithRender(func(v *gocui.View) error {
			mlm.tui.renderDebugMessages(v)
			mlm.tui.updateViewFocus(mlm.tui.g, "debug")
			return nil
		})

	input := NewView("input").
		WithSetup(func(v *gocui.View) error {
			v.Editable = true
			v.Wrap = false
			v.Title = " Input "
			return nil
		}).
		WithRender(func(v *gocui.View) error {
			if !mlm.tui.showDialog && mlm.tui.focusManager.GetCurrentFocus() == FocusInput {
				mlm.tui.g.SetCurrentView("input")
			}
			mlm.tui.updateViewFocus(mlm.tui.g, "input")
			return nil
		})

	status := NewView("status").
		WithSetup(func(v *gocui.View) error {
			v.Frame = false
			return nil
		}).
		WithRender(func(v *gocui.View) error {
			mlm.tui.renderStatus(v)
			return nil
		})

	// Build layout tree
	// Content area: messages with optional debug on right
	var contentArea Component
	if mlm.tui.showDebug {
		contentArea = NewHPanel(messages, debug, 0.7) // 70% messages, 30% debug
	} else {
		contentArea = messages
	}

	// Bottom area: input above status
	bottomArea := NewVPanel(input, status, 0.8) // 80% input, 20% status

	// Main layout: content area above bottom area
	mlm.root = NewVPanel(contentArea, bottomArea, 0.85) // 85% content, 15% bottom
}

// Layout performs the layout
func (mlm *MiniLayoutManager) Layout(g *gocui.Gui) error {
	termWidth, termHeight := g.Size()

	// Safety bounds
	if termWidth < 10 || termHeight < 5 {
		return nil
	}

	// Clean up debug view if debug is off
	if !mlm.tui.showDebug {
		g.DeleteView("debug")
	}

	// Rebuild layout if debug state changed
	mlm.buildLayout()

	// Render the entire tree
	bounds := Bounds{
		X:      0,
		Y:      0,
		Width:  termWidth - 1, // Account for gocui's reserved space
		Height: termHeight - 1,
	}

	// Render the tree
	if err := mlm.root.Render(g, bounds); err != nil {
		return err
	}
	
	// Handle overlays (notification, dialog, help) using the original logic
	if err := mlm.renderOverlays(g, termWidth, termHeight); err != nil {
		return err
	}
	
	return nil
}

// renderOverlays handles notification, dialog, and help overlays
func (mlm *MiniLayoutManager) renderOverlays(g *gocui.Gui, maxX, maxY int) error {
	// Notification panel (overlay on input when needed) - keep original logic
	if mlm.tui.notificationText != "" && mlm.tui.isNotificationVisible() {
		if v, err := g.SetView(viewNotification, 0, maxY-3, maxX-1, maxY-1, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Frame = true  // Border makes it visible in your terminal
			mlm.tui.renderNotification(v)
		}
	} else {
		// Remove notification view when not needed
		g.DeleteView(viewNotification)
	}
	
	// Dialog overlay (only when needed) - keep original logic
	if mlm.tui.showDialog {
		dialogWidth := 50
		dialogHeight := 8
		dialogX := (maxX - dialogWidth) / 2
		dialogY := (maxY - dialogHeight) / 2
		
		if v, err := g.SetView(viewDialog, dialogX, dialogY, dialogX+dialogWidth, dialogY+dialogHeight, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " " + mlm.tui.dialogTitle + " "
			v.Wrap = true
			
			// Render dialog content
			v.Clear()
			v.Write([]byte(mlm.tui.dialogMessage + "\n\nPress 'y' for Yes, 'n' or ESC for No"))
			
			// Make dialog the current view
			g.SetCurrentView(viewDialog)
		}
	} else {
		// Remove dialog if it exists
		g.DeleteView(viewDialog)
	}
	
	// Help overlay (only when needed) - keep original logic
	if mlm.tui.showHelp {
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
			mlm.tui.renderHelpContent(v)
			
			// Make help the current view
			g.SetCurrentView(viewHelp)
		}
	} else {
		// Remove help if it exists
		g.DeleteView(viewHelp)
	}
	
	return nil
}

// ToggleDebug toggles debug and rebuilds layout
func (mlm *MiniLayoutManager) ToggleDebug() {
	mlm.buildLayout() // Rebuild the tree
}