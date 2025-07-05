package component

import (
	"fmt"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/kcaldas/genie/cmd/tui/types"
)

// ConfirmationDialogComponent provides a dialog for confirming tool execution
// with optional content preview (like file diffs, command details, etc.)
type ConfirmationDialogComponent struct {
	*DialogComponent
	title       string
	message     string
	content     string
	contentType string // "diff", "command", "plan", etc.
	confirmText string
	cancelText  string
	onConfirm   func() error
	onCancel    func() error
	selectedBtn int // 0 = confirm, 1 = cancel
	scrollY     int // For content scrolling
}

func NewConfirmationDialogComponent(
	title, message, content, contentType string,
	confirmText, cancelText string,
	guiCommon types.IGuiCommon,
	onConfirm, onCancel func() error,
	onClose func() error,
) *ConfirmationDialogComponent {
	
	dialog := NewDialogComponent("confirm-dialog", "confirm-dialog", guiCommon, onClose)
	dialog.SetTitle(" " + title + " ")
	
	// Default button texts (though we use fixed format for display)
	if confirmText == "" {
		confirmText = "Yes"
	}
	if cancelText == "" {
		cancelText = "No"
	}
	
	component := &ConfirmationDialogComponent{
		DialogComponent: dialog,
		title:           title,
		message:         message,
		content:         content,
		contentType:     contentType,
		confirmText:     confirmText,
		cancelText:      cancelText,
		onConfirm:       onConfirm,
		onCancel:        onCancel,
		selectedBtn:     1, // Default to Cancel for safety
		scrollY:         0,
	}
	
	// Set up internal layout
	component.setupInternalLayout()
	
	return component
}

// setupInternalLayout configures the dialog layout:
// - Top: Message
// - Middle: Content (scrollable if present)
// - Bottom: Buttons
func (c *ConfirmationDialogComponent) setupInternalLayout() {
	children := []*boxlayout.Box{
		{
			Window: "message",
			Size:   3, // Fixed height for message
		},
	}
	
	// Add content panel if content exists
	if c.content != "" {
		children = append(children, &boxlayout.Box{
			Window: "content",
			Weight: 1, // Takes remaining space
		})
	}
	
	// Add buttons panel
	children = append(children, &boxlayout.Box{
		Window: "buttons",
		Size:   5, // Increased height for buttons visibility
	})
	
	layout := &boxlayout.Box{
		Direction: boxlayout.ROW, // Vertical layout
		Children:  children,
	}
	
	c.SetInternalLayout(layout)
}

func (c *ConfirmationDialogComponent) GetKeybindings() []*types.KeyBinding {
	// Start with base dialog keybindings (Esc, q)
	keybindings := c.GetCloseKeybindings()
	
	// Use GLOBAL keybindings that work when this dialog is active
	confirmBindings := []*types.KeyBinding{
		{
			View:    "", // Global
			Key:     '1',
			Mod:     gocui.ModNone,
			Handler: func(g *gocui.Gui, v *gocui.View) error {
				// Only handle if this dialog is visible
				if c.DialogComponent.IsVisible() {
					return c.handleConfirm(g, v)
				}
				return nil
			},
		},
		{
			View:    "", // Global
			Key:     '2',
			Mod:     gocui.ModNone,
			Handler: func(g *gocui.Gui, v *gocui.View) error {
				// Only handle if this dialog is visible
				if c.DialogComponent.IsVisible() {
					return c.handleCancel(g, v)
				}
				return nil
			},
		},
	}
	
	// Add scrolling if content exists
	if c.content != "" {
		scrollBindings := []*types.KeyBinding{
			{
				View:    c.viewName,
				Key:     gocui.KeyArrowUp,
				Mod:     gocui.ModNone,
				Handler: c.handleScrollUp,
			},
			{
				View:    c.viewName,
				Key:     gocui.KeyArrowDown,
				Mod:     gocui.ModNone,
				Handler: c.handleScrollDown,
			},
			{
				View:    c.viewName,
				Key:     gocui.KeyPgup,
				Mod:     gocui.ModNone,
				Handler: c.handlePageUp,
			},
			{
				View:    c.viewName,
				Key:     gocui.KeyPgdn,
				Mod:     gocui.ModNone,
				Handler: c.handlePageDown,
			},
			{
				View:    c.viewName,
				Key:     'k',
				Mod:     gocui.ModNone,
				Handler: c.handleScrollUp,
			},
			{
				View:    c.viewName,
				Key:     'j',
				Mod:     gocui.ModNone,
				Handler: c.handleScrollDown,
			},
		}
		confirmBindings = append(confirmBindings, scrollBindings...)
	}
	
	// Only bind to main dialog view - internal view binding can cause issues
	
	return keybindings
}

func (c *ConfirmationDialogComponent) handleLeft(g *gocui.Gui, v *gocui.View) error {
	c.selectedBtn = 0 // Confirm
	return c.Render()
}

func (c *ConfirmationDialogComponent) handleRight(g *gocui.Gui, v *gocui.View) error {
	c.selectedBtn = 1 // Cancel
	return c.Render()
}

func (c *ConfirmationDialogComponent) handleTab(g *gocui.Gui, v *gocui.View) error {
	c.selectedBtn = 1 - c.selectedBtn // Toggle between 0 and 1
	return c.Render()
}

func (c *ConfirmationDialogComponent) handleEnter(g *gocui.Gui, v *gocui.View) error {
	if c.selectedBtn == 0 {
		return c.handleConfirm(g, v)
	}
	return c.handleCancel(g, v)
}

func (c *ConfirmationDialogComponent) handleConfirm(g *gocui.Gui, v *gocui.View) error {
	if c.onConfirm != nil {
		return c.onConfirm()
	}
	return c.Close()
}

func (c *ConfirmationDialogComponent) handleCancel(g *gocui.Gui, v *gocui.View) error {
	if c.onCancel != nil {
		return c.onCancel()
	}
	return c.Close()
}

func (c *ConfirmationDialogComponent) handleScrollUp(g *gocui.Gui, v *gocui.View) error {
	if c.scrollY > 0 {
		c.scrollY--
		return c.Render()
	}
	return nil
}

func (c *ConfirmationDialogComponent) handleScrollDown(g *gocui.Gui, v *gocui.View) error {
	if c.content != "" {
		lines := strings.Split(c.content, "\n")
		if c.scrollY < len(lines)-1 {
			c.scrollY++
			return c.Render()
		}
	}
	return nil
}

func (c *ConfirmationDialogComponent) handlePageUp(g *gocui.Gui, v *gocui.View) error {
	c.scrollY -= 10
	if c.scrollY < 0 {
		c.scrollY = 0
	}
	return c.Render()
}

func (c *ConfirmationDialogComponent) handlePageDown(g *gocui.Gui, v *gocui.View) error {
	if c.content != "" {
		lines := strings.Split(c.content, "\n")
		c.scrollY += 10
		if c.scrollY >= len(lines) {
			c.scrollY = len(lines) - 1
		}
		if c.scrollY < 0 {
			c.scrollY = 0
		}
		return c.Render()
	}
	return nil
}

// Show displays the confirmation dialog with appropriate size based on content
func (c *ConfirmationDialogComponent) Show() error {
	// Start with working fixed sizes, then make slightly dynamic
	widthPercent := 60
	heightPercent := 40
	minWidth := 50
	minHeight := 15
	maxWidth := 120
	maxHeight := 50
	
	// Adjust size if we have content to display
	if c.content != "" {
		widthPercent = 80
		heightPercent = 60
	}
	
	bounds := c.CalculateDialogBounds(widthPercent, heightPercent, minWidth, minHeight, maxWidth, maxHeight)
	err := c.DialogComponent.Show(bounds)
	if err != nil {
		return err
	}
	
	// Disable cursor globally for clean appearance
	gui := c.BaseComponent.gui.GetGui()
	gui.Update(func(g *gocui.Gui) error {
		g.Cursor = false
		return nil
	})
	
	return nil
}

// Close restores cursor and closes the dialog
func (c *ConfirmationDialogComponent) Close() error {
	// Restore cursor
	gui := c.BaseComponent.gui.GetGui()
	gui.Update(func(g *gocui.Gui) error {
		g.Cursor = true
		return nil
	})
	
	return c.DialogComponent.Close()
}

func (c *ConfirmationDialogComponent) Render() error {
	// Render message panel
	if err := c.renderMessagePanel(); err != nil {
		return err
	}
	
	// Render content panel if present
	if c.content != "" {
		if err := c.renderContentPanel(); err != nil {
			return err
		}
	}
	
	// Render buttons panel
	if err := c.renderButtonsPanel(); err != nil {
		return err
	}
	
	return nil
}

func (c *ConfirmationDialogComponent) renderMessagePanel() error {
	view := c.GetInternalView("message")
	if view == nil {
		return nil
	}
	
	view.Clear()
	view.Highlight = false
	view.Editable = false
	
	// Word wrap the message to fit the view width
	maxX, _ := view.Size()
	lines := c.wrapText(c.message, maxX-2) // Leave space for borders
	
	for _, line := range lines {
		fmt.Fprintln(view, line)
	}
	
	return nil
}

func (c *ConfirmationDialogComponent) renderContentPanel() error {
	view := c.GetInternalView("content")
	if view == nil {
		return nil
	}
	
	view.Clear()
	view.Highlight = false
	view.Editable = false
	view.Title = c.getContentTitle()
	
	lines := strings.Split(c.content, "\n")
	_, maxY := view.Size()
	
	// Render visible lines starting from scrollY
	visibleLines := maxY - 2 // Account for borders
	for i := 0; i < visibleLines && (c.scrollY+i) < len(lines); i++ {
		lineIndex := c.scrollY + i
		if lineIndex >= 0 && lineIndex < len(lines) {
			fmt.Fprintln(view, lines[lineIndex])
		}
	}
	
	// Show scroll indicator if content is scrollable
	if len(lines) > visibleLines {
		scrollPercent := float64(c.scrollY) / float64(len(lines)-visibleLines) * 100
		view.Subtitle = fmt.Sprintf(" (%d%%) ", int(scrollPercent))
	}
	
	return nil
}

func (c *ConfirmationDialogComponent) renderButtonsPanel() error {
	view := c.GetInternalView("buttons")
	if view == nil {
		return nil
	}
	
	view.Clear()
	view.Highlight = false
	view.Editable = true  // Make buttons view editable so it can receive focus and keys
	
	// Add visible content with multiple lines to ensure visibility (like we had working before)
	buttonText := c.getButtonText()
	
	// Add multiple lines to ensure visibility - this is what worked before
	fmt.Fprintln(view, "")
	fmt.Fprintln(view, buttonText)
	fmt.Fprintln(view, "")
	
	return nil
}

func (c *ConfirmationDialogComponent) getContentTitle() string {
	switch c.contentType {
	case "diff":
		return " File Changes "
	case "command":
		return " Command Details "
	case "plan":
		return " Implementation Plan "
	default:
		return " Details "
	}
}

func (c *ConfirmationDialogComponent) getButtonText() string {
	// Use standard format: "1 - Yes, 2 - No (Esc)"
	yesBtn := "1 - Yes"
	noBtn := "2 - No (Esc)"
	
	// Highlight selected button with brackets
	if c.selectedBtn == 0 {
		yesBtn = "[" + yesBtn + "]"
	} else {
		noBtn = "[" + noBtn + "]"
	}
	
	return fmt.Sprintf("%s, %s", yesBtn, noBtn)
}

func (c *ConfirmationDialogComponent) wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	
	var lines []string
	var currentLine string
	
	for _, word := range words {
		if len(currentLine)+len(word)+1 <= width {
			if currentLine == "" {
				currentLine = word
			} else {
				currentLine += " " + word
			}
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}
	
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	
	return lines
}