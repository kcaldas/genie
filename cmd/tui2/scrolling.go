package tui2

import (
	"fmt"

	"github.com/awesome-gocui/gocui"
)

// setupScrollingKeyBindings sets up scrolling controls for focused panels
func (t *TUI) setupScrollingKeyBindings() error {
	// Standard scrolling controls that work based on focused panel
	scrollBindings := []struct {
		key gocui.Key
		mod gocui.Modifier
		fn  func(*gocui.Gui, *gocui.View) error
	}{
		{gocui.KeyPgup, gocui.ModNone, t.pageUp},
		{gocui.KeyPgdn, gocui.ModNone, t.pageDown},
		{gocui.KeyCtrlU, gocui.ModNone, t.halfPageUp},
		{gocui.KeyCtrlD, gocui.ModNone, t.halfPageDown},
		{gocui.KeyCtrlB, gocui.ModNone, t.pageUp},     // Vi-style
		{gocui.KeyCtrlF, gocui.ModNone, t.pageDown},   // Vi-style
	}
	
	// Set up global scrolling bindings
	for _, binding := range scrollBindings {
		if err := t.g.SetKeybinding("", binding.key, binding.mod, binding.fn); err != nil {
			return err
		}
	}
	
	// Home/End for jumping to start/end
	if err := t.g.SetKeybinding("", gocui.KeyHome, gocui.ModNone, t.scrollToTop); err != nil {
		return err
	}
	
	if err := t.g.SetKeybinding("", gocui.KeyEnd, gocui.ModNone, t.scrollToBottom); err != nil {
		return err
	}
	
	return nil
}

// pageUp scrolls up one page in the focused scrollable panel
func (t *TUI) pageUp(g *gocui.Gui, v *gocui.View) error {
	return t.scrollFocusedPanel(g, func(view *gocui.View) error {
		return t.scrollViewUp(view, t.getPageSize(view))
	})
}

// pageDown scrolls down one page in the focused scrollable panel
func (t *TUI) pageDown(g *gocui.Gui, v *gocui.View) error {
	return t.scrollFocusedPanel(g, func(view *gocui.View) error {
		return t.scrollViewDown(view, t.getPageSize(view))
	})
}

// halfPageUp scrolls up half a page (Ctrl+U)
func (t *TUI) halfPageUp(g *gocui.Gui, v *gocui.View) error {
	return t.scrollFocusedPanel(g, func(view *gocui.View) error {
		return t.scrollViewUp(view, t.getPageSize(view)/2)
	})
}

// halfPageDown scrolls down half a page (Ctrl+D)
func (t *TUI) halfPageDown(g *gocui.Gui, v *gocui.View) error {
	return t.scrollFocusedPanel(g, func(view *gocui.View) error {
		return t.scrollViewDown(view, t.getPageSize(view)/2)
	})
}

// scrollToTop jumps to the beginning of content
func (t *TUI) scrollToTop(g *gocui.Gui, v *gocui.View) error {
	return t.scrollFocusedPanel(g, func(view *gocui.View) error {
		view.SetOrigin(0, 0)
		view.Autoscroll = false
		return nil
	})
}

// scrollToBottom jumps to the end of content
func (t *TUI) scrollToBottom(g *gocui.Gui, v *gocui.View) error {
	return t.scrollFocusedPanel(g, func(view *gocui.View) error {
		view.Autoscroll = true
		return nil
	})
}

// scrollFocusedPanel applies a scroll action to the currently focused scrollable panel
func (t *TUI) scrollFocusedPanel(g *gocui.Gui, scrollFn func(*gocui.View) error) error {
	focused := t.focusManager.GetCurrentFocus()
	
	// Only allow scrolling on panels that support it
	var targetView *gocui.View
	var err error
	
	switch focused {
	case FocusMessages:
		targetView, err = g.View(viewMessages)
	case FocusDebug:
		if t.showDebug {
			targetView, err = g.View(viewDebug)
		}
	case FocusInput:
		// Input view doesn't support scrolling
		return nil
	case FocusDialog:
		// Dialog might support scrolling in the future
		return nil
	default:
		return nil
	}
	
	if err != nil || targetView == nil {
		return err
	}
	
	return scrollFn(targetView)
}

// scrollViewUp scrolls a view up by the specified number of lines
func (t *TUI) scrollViewUp(view *gocui.View, lines int) error {
	ox, oy := view.Origin()
	newY := oy - lines
	if newY < 0 {
		newY = 0
	}
	
	view.SetOrigin(ox, newY)
	view.Autoscroll = false // Disable autoscroll when manually scrolling
	
	t.addDebugMessage(fmt.Sprintf("Scrolled up %d lines", lines))
	return nil
}

// scrollViewDown scrolls a view down by the specified number of lines
func (t *TUI) scrollViewDown(view *gocui.View, lines int) error {
	ox, oy := view.Origin()
	_, viewHeight := view.Size()
	contentHeight := len(view.ViewBufferLines())
	
	newY := oy + lines
	
	// Check if we're scrolling to the bottom
	if newY+viewHeight >= contentHeight {
		// Enable autoscroll and jump to bottom
		view.Autoscroll = true
		t.addDebugMessage("Scrolled to bottom, autoscroll enabled")
	} else {
		view.SetOrigin(ox, newY)
		view.Autoscroll = false
		t.addDebugMessage(fmt.Sprintf("Scrolled down %d lines", lines))
	}
	
	return nil
}

// getPageSize returns the number of lines to scroll for a "page"
func (t *TUI) getPageSize(view *gocui.View) int {
	_, height := view.Size()
	if height < 3 {
		return 1
	}
	return height - 2 // Leave some overlap
}

// isViewScrollable returns true if the view supports scrolling
func (t *TUI) isViewScrollable(viewName string) bool {
	switch viewName {
	case viewMessages, viewDebug:
		return true
	case viewInput, viewStatus, viewDialog:
		return false
	default:
		return false
	}
}