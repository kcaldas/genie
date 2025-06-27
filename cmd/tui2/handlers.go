package tui2

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/awesome-gocui/gocui"
)

// setupKeyBindings sets up all key bindings
func (t *TUI) setupKeyBindings() error {
	// Global bindings
	if err := t.g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, t.quit); err != nil {
		return err
	}
	
	// Debug panel toggle (global) - F12 is universal for debug panels
	if err := t.g.SetKeybinding("", gocui.KeyF12, gocui.ModNone, t.toggleDebugPanel); err != nil {
		return err
	}
	
	// Help panel toggle (global) - F1 is universal for help
	if err := t.g.SetKeybinding("", gocui.KeyF1, gocui.ModNone, t.toggleHelpPanel); err != nil {
		return err
	}
	
	// System clipboard support - copy current view content (macOS Cmd+C is mapped to a special key combination in gocui)
	// On macOS terminals, Cmd+C often translates to a specific sequence we can catch
	if err := t.g.SetKeybinding("", 'c', gocui.ModAlt, t.copyToSystemClipboard); err != nil {
		return err
	}
	
	// Focus management
	if err := t.setupFocusKeyBindings(); err != nil {
		return err
	}
	
	// Scrolling controls
	if err := t.setupScrollingKeyBindings(); err != nil {
		return err
	}
	
	// Input view bindings
	if err := t.g.SetKeybinding(viewInput, gocui.KeyEnter, gocui.ModNone, t.handleInput); err != nil {
		return err
	}
	
	if err := t.g.SetKeybinding(viewInput, gocui.KeyEsc, gocui.ModNone, t.handleEscape); err != nil {
		return err
	}
	
	// Input-specific bindings (only active when input is focused)
	if err := t.g.SetKeybinding(viewInput, gocui.KeyArrowUp, gocui.ModNone, t.navigateHistoryUp); err != nil {
		return err
	}
	
	if err := t.g.SetKeybinding(viewInput, gocui.KeyArrowDown, gocui.ModNone, t.navigateHistoryDown); err != nil {
		return err
	}
	
	
	// Clipboard operations (input view only)
	if err := t.g.SetKeybinding(viewInput, gocui.KeyCtrlY, gocui.ModNone, t.yankSelection); err != nil {
		return err
	}
	
	if err := t.g.SetKeybinding(viewInput, gocui.KeyCtrlP, gocui.ModNone, t.pasteFromClipboard); err != nil {
		return err
	}
	
	if err := t.g.SetKeybinding(viewInput, gocui.KeyCtrlA, gocui.ModNone, t.selectAll); err != nil {
		return err
	}
	
	// Dialog bindings
	if err := t.g.SetKeybinding(viewDialog, 'y', gocui.ModNone, t.confirmDialog); err != nil {
		return err
	}
	
	if err := t.g.SetKeybinding(viewDialog, 'n', gocui.ModNone, t.cancelDialog); err != nil {
		return err
	}
	
	if err := t.g.SetKeybinding(viewDialog, gocui.KeyEsc, gocui.ModNone, t.cancelDialog); err != nil {
		return err
	}
	
	// Help panel bindings
	if err := t.g.SetKeybinding(viewHelp, gocui.KeyEsc, gocui.ModNone, t.closeHelpPanel); err != nil {
		return err
	}
	
	if err := t.g.SetKeybinding(viewHelp, 'q', gocui.ModNone, t.closeHelpPanel); err != nil {
		return err
	}
	
	return nil
}

// Key binding handlers

func (t *TUI) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (t *TUI) handleInput(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(v.Buffer())
	if input == "" {
		return nil
	}
	
	// Add to history
	t.chatHistory.AddCommand(input)
	t.addDebugMessage(fmt.Sprintf("Added command to history: %s", input))
	
	// Clear input
	v.Clear()
	v.SetCursor(0, 0)
	
	// Add user message
	t.addMessage(UserMessage, input)
	t.addDebugMessage(fmt.Sprintf("User message added: %s", input))
	
	// Handle commands
	if strings.HasPrefix(input, "/") {
		return t.handleCommand(input)
	}
	
	// Handle as chat message
	return t.handleChatMessage(input)
}

func (t *TUI) handleEscape(g *gocui.Gui, v *gocui.View) error {
	if t.loading && t.cancelCurrentRequest != nil {
		t.cancelCurrentRequest()
		t.loading = false
		t.addMessage(SystemMessage, "Request cancelled")
		t.addDebugMessage("Request cancelled by user (ESC)")
	}
	return nil
}


func (t *TUI) confirmDialog(g *gocui.Gui, v *gocui.View) error {
	if t.dialogCallback != nil {
		t.dialogCallback(true)
	}
	t.hideDialog()
	return nil
}

func (t *TUI) cancelDialog(g *gocui.Gui, v *gocui.View) error {
	if t.dialogCallback != nil {
		t.dialogCallback(false)
	}
	t.hideDialog()
	return nil
}

// Helper methods

func (t *TUI) handleCommand(input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}
	
	switch parts[0] {
	case "/help":
		t.showHelp = true
		t.addDebugMessage("Help panel opened via /help command")
		
	case "/debug":
		t.showDebug = !t.showDebug
		action := "enabled"
		if !t.showDebug {
			action = "disabled"
		}
		t.addMessage(SystemMessage, fmt.Sprintf("Debug mode %s", action))
		t.addDebugMessage(fmt.Sprintf("Debug toggled via command to: %s", action))
		
	case "/renderer":
		// Show current renderer info or switch if argument provided
		if len(parts) > 1 {
			// Check if theme is specified: /renderer glamour dark
			theme := "auto"
			if len(parts) > 2 && strings.ToLower(parts[1]) == "glamour" {
				theme = parts[2]
			}
			t.switchRenderer(parts[1], theme)
		} else {
			t.showRendererInfo()
		}
		
	case "/theme":
		// Show current theme or switch if argument provided
		if len(parts) > 1 {
			t.switchTheme(parts[1])
		} else {
			t.showThemeInfo()
		}
		
	case "/clear":
		t.messages = []Message{}
		if v, err := t.g.View(viewMessages); err == nil {
			v.Clear()
		}
		t.addDebugMessage("Messages cleared")
		
	case "/exit", "/quit":
		return gocui.ErrQuit
		
	default:
		t.addMessage(ErrorMessage, fmt.Sprintf("Unknown command: %s", parts[0]))
		t.addDebugMessage(fmt.Sprintf("Unknown command attempted: %s", parts[0]))
	}
	
	return nil
}

func (t *TUI) handleChatMessage(input string) error {
	// Set loading state
	t.loading = true
	t.requestTime = time.Now()
	t.addDebugMessage(fmt.Sprintf("Starting chat request: %s", input))
	
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	t.cancelCurrentRequest = cancel
	
	// Make async request
	go func() {
		err := t.genieService.Chat(ctx, input)
		if err != nil && err != context.Canceled {
			t.g.Update(func(g *gocui.Gui) error {
				t.loading = false
				t.addMessage(ErrorMessage, fmt.Sprintf("Error: %v", err))
				t.addDebugMessage(fmt.Sprintf("Chat request failed: %v", err))
				return nil
			})
		}
	}()
	
	return nil
}

func (t *TUI) showConfirmationDialog(title, message string, callback func(bool)) {
	// Save current focus and switch to dialog
	t.focusManager.PushFocus(FocusDialog)
	
	t.showDialog = true
	t.dialogTitle = title
	t.dialogMessage = message
	t.dialogCallback = callback
	t.addDebugMessage(fmt.Sprintf("Showing confirmation dialog: %s", title))
}

func (t *TUI) hideDialog() {
	t.showDialog = false
	t.dialogTitle = ""
	t.dialogMessage = ""
	t.dialogCallback = nil
	
	// Restore previous focus
	t.focusManager.PopFocus()
	
	t.addDebugMessage("Dialog hidden")
	
	// Update gocui current view to match focus manager
	currentFocus := t.focusManager.GetCurrentFocus()
	t.g.SetCurrentView(string(currentFocus))
}

// switchRenderer changes the markdown renderer type and theme
func (t *TUI) switchRenderer(rendererType string, theme string) {
	var newType RendererType
	
	switch strings.ToLower(rendererType) {
	case "glamour":
		newType = GlamourRendererType
	case "plain", "plaintext":
		newType = PlainTextRendererType
	case "custom":
		newType = CustomRendererType
	default:
		t.addMessage(ErrorMessage, fmt.Sprintf("Unknown renderer type: %s", rendererType))
		t.addMessage(SystemMessage, "Available types: glamour, plaintext, custom")
		return
	}
	
	// Get current view width for new renderer
	viewWidth := 80 // Default
	if v, err := t.g.View(viewMessages); err == nil {
		viewWidth, _ = v.Size()
		if viewWidth > 2 {
			viewWidth -= 2 // Account for borders
		}
	}
	
	// Create new renderer
	var newRenderer MarkdownRenderer
	if newType == GlamourRendererType {
		// Use specific theme for Glamour renderer
		t.addDebugMessage(fmt.Sprintf("Creating Glamour renderer with theme: %s", theme))
		newRenderer = NewGlamourRendererWithTheme(viewWidth, theme)
		if !newRenderer.IsEnabled() {
			// Fallback to auto theme if specified theme fails
			t.addDebugMessage(fmt.Sprintf("Theme %s failed, falling back to auto", theme))
			newRenderer = NewGlamourRendererWithTheme(viewWidth, "auto")
		} else {
			t.addDebugMessage(fmt.Sprintf("Successfully created renderer with theme: %s", theme))
		}
	} else {
		newRenderer = NewMarkdownRendererWithFallback(newType, viewWidth)
	}
	
	oldEnabled := t.markdownRenderer.IsEnabled()
	
	// Switch to new renderer
	t.markdownRenderer = newRenderer
	
	// Report the change
	status := "enabled"
	if !newRenderer.IsEnabled() {
		status = "disabled (fallback active)"
	}
	
	themePart := ""
	if newType == GlamourRendererType && theme != "auto" {
		themePart = fmt.Sprintf(" with %s theme", theme)
	}
	
	t.addMessage(SystemMessage, fmt.Sprintf("Switched to %s renderer%s (%s)", rendererType, themePart, status))
	t.addDebugMessage(fmt.Sprintf("Renderer switched: %s -> %s (was enabled: %t, now enabled: %t)", 
		rendererType, status, oldEnabled, newRenderer.IsEnabled()))
	
	// Re-render messages to show the change
	if v, err := t.g.View(viewMessages); err == nil {
		t.renderMessages(v)
	}
}

// showRendererInfo displays current renderer status
func (t *TUI) showRendererInfo() {
	status := "enabled"
	if !t.markdownRenderer.IsEnabled() {
		status = "disabled"
	}
	
	// Show current theme if it's a Glamour renderer
	currentInfo := fmt.Sprintf("Current markdown renderer: %s", status)
	if glamourRenderer, ok := t.markdownRenderer.(*GlamourRenderer); ok && glamourRenderer.IsEnabled() {
		currentInfo = fmt.Sprintf("Current markdown renderer: glamour with %s theme (%s)", glamourRenderer.GetTheme(), status)
	}
	
	t.addMessage(SystemMessage, currentInfo)
	t.addMessage(SystemMessage, "Available renderers:")
	t.addMessage(SystemMessage, "  /renderer glamour [theme] - Rich markdown with syntax highlighting")
	t.addMessage(SystemMessage, "  /renderer plaintext - Plain text (no formatting)")
	t.addMessage(SystemMessage, "  /renderer custom - Custom goldmark renderer (placeholder)")
	t.addMessage(SystemMessage, "")
	t.addMessage(SystemMessage, "Glamour themes:")
	t.addMessage(SystemMessage, "  auto, dark, light, dracula, tokyo-night, notty")
}

// yankSelection copies the current selection or entire input to clipboard
func (t *TUI) yankSelection(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		return nil
	}
	
	// Get the current buffer content
	content := strings.TrimSuffix(v.Buffer(), "\n")
	
	// Copy to both internal and system clipboard
	t.clipboard = content
	if err := clipboard.WriteAll(content); err != nil {
		t.addDebugMessage(fmt.Sprintf("Failed to copy to system clipboard: %v", err))
		t.showNotification("Failed to copy to clipboard!")
	} else {
		if content != "" {
			t.addDebugMessage(fmt.Sprintf("Yanked to clipboard: %s", content))
			t.showNotification("Copied to clipboard!")
		} else {
			t.addDebugMessage("Nothing to yank (input is empty)")
			t.showNotification("Nothing to copy!")
		}
	}
	
	return nil
}

// pasteFromClipboard pastes clipboard content at cursor position
func (t *TUI) pasteFromClipboard(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		return nil
	}
	
	// Try system clipboard first, fallback to internal clipboard
	clipboardContent, err := clipboard.ReadAll()
	if err != nil || clipboardContent == "" {
		clipboardContent = t.clipboard
		if clipboardContent == "" {
			t.addDebugMessage("Both system and internal clipboard are empty")
			return nil
		}
	}
	
	// Get current cursor position
	cx, _ := v.Cursor()
	currentContent := strings.TrimSuffix(v.Buffer(), "\n")
	
	// Insert clipboard content at cursor position
	before := ""
	after := ""
	if cx <= len(currentContent) {
		before = currentContent[:cx]
		after = currentContent[cx:]
	} else {
		before = currentContent
	}
	
	newContent := before + clipboardContent + after
	
	// Clear and set new content
	v.Clear()
	fmt.Fprint(v, newContent)
	
	// Position cursor after pasted content
	newCursorPos := len(before) + len(clipboardContent)
	if newCursorPos <= len(newContent) {
		v.SetCursor(newCursorPos, 0)
	}
	
	t.addDebugMessage(fmt.Sprintf("Pasted from clipboard: %s", clipboardContent))
	t.showNotification("Pasted from clipboard!")
	return nil
}

// selectAll selects all text in the input field
func (t *TUI) selectAll(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		return nil
	}
	
	content := strings.TrimSuffix(v.Buffer(), "\n")
	
	// Copy entire content to both internal and system clipboard
	t.clipboard = content
	if err := clipboard.WriteAll(content); err != nil {
		t.addDebugMessage(fmt.Sprintf("Failed to copy to system clipboard: %v", err))
		t.showNotification("Failed to copy to clipboard!")
	} else {
		if content != "" {
			t.addDebugMessage(fmt.Sprintf("Selected all and copied to clipboard: %s", content))
			t.showNotification("Copied to clipboard!")
		} else {
			t.addDebugMessage("Input is empty, nothing to select")
			t.showNotification("Nothing to select!")
		}
	}
	
	// Move cursor to end
	v.SetCursor(len(content), 0)
	
	return nil
}

// closeHelpPanel closes the help overlay
func (t *TUI) closeHelpPanel(g *gocui.Gui, v *gocui.View) error {
	t.showHelp = false
	t.addDebugMessage("Help panel closed")
	
	// Restore focus to input
	t.focusManager.SetFocus(FocusInput)
	g.SetCurrentView(viewInput)
	
	return nil
}

// toggleHelpPanel toggles the help panel visibility
func (t *TUI) toggleHelpPanel(g *gocui.Gui, v *gocui.View) error {
	t.showHelp = !t.showHelp
	t.addDebugMessage(fmt.Sprintf("Help panel toggled: %t", t.showHelp))
	return nil
}

// copyToSystemClipboard copies the content of the focused view to system clipboard
func (t *TUI) copyToSystemClipboard(g *gocui.Gui, v *gocui.View) error {
	focused := t.focusManager.GetCurrentFocus()
	
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
		targetView, err = g.View(viewInput)
	default:
		t.addDebugMessage("No copyable view is focused")
		return nil
	}
	
	if err != nil || targetView == nil {
		t.addDebugMessage("Failed to get focused view for copying")
		return err
	}
	
	// Get the content of the view
	content := strings.TrimSuffix(targetView.Buffer(), "\n")
	if content == "" {
		t.addDebugMessage("View is empty, nothing to copy")
		t.showNotification("Nothing to copy!")
		return nil
	}
	
	// Copy to system clipboard
	if err := clipboard.WriteAll(content); err != nil {
		t.addDebugMessage(fmt.Sprintf("Failed to copy to system clipboard: %v", err))
		t.showNotification("Failed to copy to clipboard!")
		return err
	}
	
	// Also update internal clipboard
	t.clipboard = content
	
	viewName := string(focused)
	t.addDebugMessage(fmt.Sprintf("Copied %s view content to system clipboard (%d chars)", viewName, len(content)))
	t.showNotification("Copied to clipboard!")
	
	return nil
}

// switchTheme changes the UI theme
func (t *TUI) switchTheme(themeName string) {
	if t.themeManager.SetTheme(themeName) {
		currentTheme := t.themeManager.GetCurrentTheme()
		
		// Also update Glamour theme to match
		if glamourRenderer, ok := t.markdownRenderer.(*GlamourRenderer); ok {
			glamourRenderer.SetTheme(currentTheme.GlamourTheme)
		}
		
		t.addMessage(SystemMessage, fmt.Sprintf("Switched to %s theme", currentTheme.Name))
		t.addDebugMessage(fmt.Sprintf("Theme switched to: %s (Glamour: %s)", currentTheme.Name, currentTheme.GlamourTheme))
		
		// Re-render all views to apply new theme
		if v, err := t.g.View(viewMessages); err == nil {
			t.renderMessages(v)
		}
	} else {
		t.addMessage(ErrorMessage, fmt.Sprintf("Unknown theme: %s", themeName))
		t.showThemeInfo()
	}
}

// showThemeInfo displays current theme and available themes
func (t *TUI) showThemeInfo() {
	currentTheme := t.themeManager.GetCurrentTheme()
	t.addMessage(SystemMessage, fmt.Sprintf("Current theme: %s", currentTheme.Name))
	t.addMessage(SystemMessage, currentTheme.Description)
	t.addMessage(SystemMessage, "")
	t.addMessage(SystemMessage, "Available themes:")
	
	// Get themes in a predictable order
	themeNames := t.themeManager.GetAvailableThemes()
	for _, name := range themeNames {
		theme := t.themeManager.GetTheme(name)
		if theme != nil {
			t.addMessage(SystemMessage, fmt.Sprintf("  /theme %s - %s", name, theme.Description))
		}
	}
}