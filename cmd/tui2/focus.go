package tui2

import (
	"fmt"

	"github.com/awesome-gocui/gocui"
)

// FocusablePanel represents a panel that can receive focus
type FocusablePanel string

const (
	FocusMessages FocusablePanel = viewMessages
	FocusInput    FocusablePanel = viewInput
	FocusDebug    FocusablePanel = viewDebug
	FocusDialog   FocusablePanel = viewDialog
)

// FocusManager handles focus state and navigation between panels
type FocusManager struct {
	currentFocus FocusablePanel
	focusStack   []FocusablePanel // For dialog management
}

// NewFocusManager creates a new focus manager with input as default focus
func NewFocusManager() *FocusManager {
	return &FocusManager{
		currentFocus: FocusInput,
		focusStack:   make([]FocusablePanel, 0),
	}
}

// GetCurrentFocus returns the currently focused panel
func (fm *FocusManager) GetCurrentFocus() FocusablePanel {
	return fm.currentFocus
}

// SetFocus changes focus to the specified panel
func (fm *FocusManager) SetFocus(panel FocusablePanel) {
	fm.currentFocus = panel
}

// PushFocus saves current focus and sets new focus (for dialogs)
func (fm *FocusManager) PushFocus(panel FocusablePanel) {
	fm.focusStack = append(fm.focusStack, fm.currentFocus)
	fm.currentFocus = panel
}

// PopFocus restores previous focus (when closing dialogs)
func (fm *FocusManager) PopFocus() {
	if len(fm.focusStack) > 0 {
		fm.currentFocus = fm.focusStack[len(fm.focusStack)-1]
		fm.focusStack = fm.focusStack[:len(fm.focusStack)-1]
	}
}

// GetAvailablePanels returns list of panels that can receive focus
func (fm *FocusManager) GetAvailablePanels(showDebug bool) []FocusablePanel {
	panels := []FocusablePanel{FocusMessages, FocusInput}
	if showDebug {
		panels = append(panels, FocusDebug)
	}
	return panels
}

// CycleFocus moves focus to the next available panel
func (fm *FocusManager) CycleFocus(showDebug bool) FocusablePanel {
	available := fm.GetAvailablePanels(showDebug)
	
	// Find current panel index
	currentIndex := -1
	for i, panel := range available {
		if panel == fm.currentFocus {
			currentIndex = i
			break
		}
	}
	
	// Move to next panel (wrap around)
	nextIndex := (currentIndex + 1) % len(available)
	fm.currentFocus = available[nextIndex]
	
	return fm.currentFocus
}

// Add focus manager to TUI struct
func (t *TUI) initializeFocusManager() {
	t.focusManager = NewFocusManager()
}

// setupFocusKeyBindings sets up global focus navigation key bindings
func (t *TUI) setupFocusKeyBindings() error {
	// Tab to cycle between panels
	if err := t.g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, t.cycleFocus); err != nil {
		return err
	}
	
	// Shift+Tab to cycle backwards (if we want to implement it)
	// Currently just cycles forward
	
	return nil
}

// cycleFocus handles Tab key to cycle between focusable panels
func (t *TUI) cycleFocus(g *gocui.Gui, v *gocui.View) error {
	// Don't cycle focus when dialog is open
	if t.showDialog {
		return nil
	}
	
	newFocus := t.focusManager.CycleFocus(t.showDebug)
	t.addDebugMessage(fmt.Sprintf("Focus cycled to: %s", string(newFocus)))
	
	// Update gocui current view
	_, err := g.SetCurrentView(string(newFocus))
	return err
}

// setFocusedPanel sets focus to a specific panel and updates gocui
func (t *TUI) setFocusedPanel(panel FocusablePanel) error {
	t.focusManager.SetFocus(panel)
	t.addDebugMessage(fmt.Sprintf("Focus set to: %s", string(panel)))
	
	// Update gocui current view
	_, err := t.g.SetCurrentView(string(panel))
	return err
}

// getFocusedPanel returns the currently focused panel
func (t *TUI) getFocusedPanel() FocusablePanel {
	return t.focusManager.GetCurrentFocus()
}