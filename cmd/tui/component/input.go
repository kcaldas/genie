package component

import (
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/gdamore/tcell/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type InputComponent struct {
	*BaseComponent
	commandEventBus *events.CommandEventBus
	history         history.ChatHistory
	clipboard       *helpers.Clipboard
	lastContent     string // Track content changes to detect paste
}

func NewInputComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, clipboard *helpers.Clipboard, historyPath string) *InputComponent {
	ctx := &InputComponent{
		BaseComponent:   NewBaseComponent("input", "input", gui, configManager),
		commandEventBus: commandEventBus,
		history:         history.NewChatHistory(historyPath, true), // Enable saving
		clipboard:       clipboard,
	}

	// Load history on startup
	if err := ctx.LoadHistory(); err != nil {
		// Don't fail startup if history loading fails, just log it
		// Since we're discarding logs in TUI mode, this won't show up
	}

	// Configure InputComponent specific properties
	ctx.SetTitle("")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:  true,
		Editable:   true,
		Wrap:       false,
		Autoscroll: true,
		Highlight:  false,
		Frame:      true,
		Subtitle:   "F4/Ctrl+V Expand",
	})

	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			//v.SelFgColor = gocui.ColorBlack
		}
		return nil
	})

	ctx.SetOnFocusLost(func() error {
		if v := ctx.GetView(); v != nil {
			//v.Highlight = false
		}
		return nil
	})

	// Subscribe to theme changes
	commandEventBus.Subscribe("theme.changed", func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.RefreshThemeColors()
		})
	})

	return ctx
}

// SetView overrides the base SetView to configure the custom editor
func (c *InputComponent) SetView(view *gocui.View) {
	// Call parent SetView to store the view reference
	c.BaseComponent.SetView(view)

	// Set up custom editor to filter unbound keys
	if view != nil {
		view.Editor = NewCustomEditor()
	}
}

func (c *InputComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{
		{
			View:    c.viewName,
			Key:     gocui.KeyEnter,
			Handler: c.handleSubmit,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowUp,
			Handler: c.navigateHistoryUp,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowDown,
			Handler: c.navigateHistoryDown,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlC,
			Handler: c.clearInput,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlL,
			Handler: c.clearInput,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEsc,
			Handler: c.handleEsc,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlV,
			Handler: c.handlePaste,
		},
		{
			View:    c.viewName,
			Key:     'v',
			Mod:     gocui.Modifier(tcell.ModAlt), // Alt+V (may work for Command+V on Mac)
			Handler: c.handlePaste,
		},
		{
			View:    c.viewName,
			Key:     'v',
			Mod:     gocui.Modifier(tcell.ModMeta), // Try ModMeta as well
			Handler: c.handlePaste,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyInsert,
			Mod:     gocui.Modifier(tcell.ModShift), // Shift+Insert on Linux/Windows
			Handler: c.handlePaste,
		},
		{
			View:    c.viewName,
			Key:     'v',
			Mod:     gocui.Modifier(tcell.ModCtrl | tcell.ModShift), // Ctrl+Shift+V as fallback
			Handler: c.handlePasteForce,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyF4,      // Add F4 binding
			Handler: c.handleF4Expand, // Bind F4 to handleF4Expand
		},
	}
}

func (c *InputComponent) handleSubmit(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(v.Buffer())
	if input == "" {
		return nil
	}

	c.history.AddCommand(input)
	c.history.ResetNavigation()

	v.Clear()
	v.SetCursor(0, 0)

	// Determine if input is a command and emit appropriate event
	if strings.HasPrefix(input, ":") {
		// Emit command event
		c.commandEventBus.Emit("user.input.command", input)
	} else {
		// Emit text/chat message event
		c.commandEventBus.Emit("user.input.text", input)
	}

	return nil
}

func (c *InputComponent) handleEsc(g *gocui.Gui, v *gocui.View) error {
	c.commandEventBus.Emit("user.input.cancel", "")

	// Ensure the input field remains properly rendered after ESC
	// The renderMessages() call from the cancel event can interfere with input display
	c.gui.PostUIUpdate(func() {
		// Force a refresh of the input view to ensure it's properly rendered
		if inputView := c.GetView(); inputView != nil {
			// Redraw the input field by moving cursor to its current position
			cx, cy := inputView.Cursor()
			inputView.SetCursor(cx, cy)
		}
	})

	return nil
}

func (c *InputComponent) navigateHistoryUp(g *gocui.Gui, v *gocui.View) error {
	command := c.history.NavigatePrev()
	if command != "" {
		v.Clear()
		v.SetCursor(0, 0)
		v.Write([]byte(command))
		// Move cursor to end of text
		v.SetCursor(len(command), 0)
	}
	return nil
}

func (c *InputComponent) navigateHistoryDown(g *gocui.Gui, v *gocui.View) error {
	command := c.history.NavigateNext()
	v.Clear()
	v.SetCursor(0, 0)
	if command != "" {
		v.Write([]byte(command))
		// Move cursor to end of text
		v.SetCursor(len(command), 0)
	}
	return nil
}

func (c *InputComponent) clearInput(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(v.Buffer())

	// If input is empty, exit the application (Ctrl+C behavior)
	if input == "" {
		return gocui.ErrQuit
	}

	// Otherwise, clear the input (original behavior)
	v.Clear()
	v.SetCursor(0, 0)
	c.history.ResetNavigation()
	return nil
}

func (c *InputComponent) LoadHistory() error {
	return c.history.Load()
}

// combineAtPosition combines content at the specified string position.
// This is a pure function that can be easily unit tested.
func combineAtPosition(currentContent string, position int, newContent string) string {
	// Handle empty content
	if currentContent == "" {
		return newContent
	}

	// Clamp position to valid range
	if position < 0 {
		position = 0
	}
	if position > len(currentContent) {
		position = len(currentContent)
	}

	// Simple string concatenation: before + new + after
	before := currentContent[:position]
	after := currentContent[position:]

	return before + newContent + after
}

// cursorToStringPosition converts x,y cursor coordinates to string position
func cursorToStringPosition(content string, cursorX, cursorY int) int {
	if content == "" {
		return 0
	}

	lines := strings.Split(content, "\n")

	// Clamp cursor Y to valid range
	if cursorY < 0 {
		return 0
	}
	if cursorY >= len(lines) {
		return len(content)
	}

	// Calculate position by summing up line lengths
	position := 0
	for i := 0; i < cursorY; i++ {
		position += len(lines[i]) + 1 // +1 for newline character
	}

	// Add cursor X position within the current line
	line := lines[cursorY]
	if cursorX > len(line) {
		cursorX = len(line)
	}
	position += cursorX

	return position
}

// stringPositionToCursor converts string position back to x,y cursor coordinates
func stringPositionToCursor(content string, position int) (int, int) {
	if content == "" || position <= 0 {
		return 0, 0
	}

	// Clamp position to content length
	if position > len(content) {
		position = len(content)
	}

	lines := strings.Split(content, "\n")
	currentPos := 0

	for lineIndex, line := range lines {
		lineLength := len(line)

		// Check if position is within this line
		if currentPos+lineLength >= position {
			// Position is in this line
			x := position - currentPos
			return x, lineIndex
		}

		// Move past this line + newline character
		currentPos += lineLength + 1
	}

	// Position is at the very end
	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		return len(lastLine), len(lines) - 1
	}

	return 0, 0
}

// combineWithCurrentInput combines the current input content with new content at cursor position,
// clears the input field, and returns the combined result
func (c *InputComponent) combineWithCurrentInput(v *gocui.View, newContent string) string {
	// Get current input content and cursor position
	currentContent := v.Buffer()
	cx, cy := v.Cursor()

	// Convert cursor coordinates to string position
	position := cursorToStringPosition(currentContent, cx, cy)

	// Use the pure function to combine content
	combinedContent := combineAtPosition(currentContent, position, newContent)

	// Clear the input field since we're moving to write component
	v.Clear()
	v.SetCursor(0, 0)

	return strings.TrimSpace(combinedContent)
}

func (c *InputComponent) handlePaste(g *gocui.Gui, v *gocui.View) error {
	// Get clipboard content
	clipboardContent, err := c.clipboard.Paste()
	if err != nil {
		// If clipboard access fails, let the default paste behavior happen
		return nil
	}

	// Check if clipboard content contains newlines (multiline)
	if strings.Contains(clipboardContent, "\n") {
		// Combine with current input and trigger write component
		combinedContent := c.combineWithCurrentInput(v, clipboardContent)
		c.commandEventBus.Emit("paste.multiline", combinedContent)
		return nil
	}

	// For single-line content, paste it into the current input
	currentContent := v.Buffer()
	cx, cy := v.Cursor()

	// Convert cursor position to string position
	position := cursorToStringPosition(currentContent, cx, cy)

	// Combine content at position
	newContent := combineAtPosition(currentContent, position, clipboardContent)

	// Update the view content
	v.Clear()
	v.Write([]byte(newContent))

	// Move cursor to end of pasted content
	newPosition := position + len(clipboardContent)
	newCx, newCy := stringPositionToCursor(newContent, newPosition)
	v.SetCursor(newCx, newCy)

	return nil
}

func (c *InputComponent) handlePasteForce(g *gocui.Gui, v *gocui.View) error {
	// Get clipboard content
	clipboardContent, err := c.clipboard.Paste()
	if err != nil {
		// If clipboard access fails, just open empty write component
		c.commandEventBus.Emit("user.input.command", ":write")
		return nil
	}

	// Always trigger write command with combined content (force multiline mode)
	combinedContent := c.combineWithCurrentInput(v, clipboardContent)
	c.commandEventBus.Emit("paste.multiline", combinedContent)
	return nil
}

func (c *InputComponent) handleF4Expand(g *gocui.Gui, v *gocui.View) error {
	combinedContent := c.combineWithCurrentInput(v, "") // Pass empty string as new content to just get current buffer and clear
	c.commandEventBus.Emit("paste.multiline", combinedContent)
	return nil
}

