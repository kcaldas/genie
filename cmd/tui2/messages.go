package tui2

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/awesome-gocui/gocui"
)

// addMessage adds a message to the main chat view
func (t *TUI) addMessage(msgType MessageType, content string) {
	msg := Message{
		Type:    msgType,
		Content: content,
		Time:    time.Now(),
		Success: nil, // Default to no success state
	}
	t.messages = append(t.messages, msg)
	
	// Update messages view
	if v, err := t.g.View(viewMessages); err == nil {
		t.renderMessages(v)
	}
}

// addToolMessage adds a tool message with success state
func (t *TUI) addToolMessage(content string, success *bool) {
	msg := Message{
		Type:    ToolMessage,
		Content: content,
		Time:    time.Now(),
		Success: success,
	}
	t.messages = append(t.messages, msg)
	
	// Update messages view
	if v, err := t.g.View(viewMessages); err == nil {
		t.renderMessages(v)
	}
}

// renderMessages renders all chat messages to the messages view
func (t *TUI) renderMessages(v *gocui.View) {
	v.Clear()
	
	for i, msg := range t.messages {
		// Add empty line before every message (except the first one) for readability
		if i > 0 {
			fmt.Fprintln(v, "")
		}
		
		// Render message based on type
		switch msg.Type {
		case UserMessage:
			t.renderUserMessage(v, msg.Content)
		case AssistantMessage:
			t.renderAssistantMessage(v, msg.Content)
		case SystemMessage:
			t.renderSystemMessage(v, msg.Content)
		case ErrorMessage:
			t.renderErrorMessage(v, msg.Content)
		case ToolMessage:
			t.renderToolMessage(v, msg.Content, msg.Success)
		}
	}
}

// renderUserMessage renders a user message with themed color and prefix
func (t *TUI) renderUserMessage(v *gocui.View, content string) {
	color := t.themeManager.GetANSIColor(ElementSecondary)
	fmt.Fprintf(v, "%s> %s\033[0m\n", color, content)
}

// renderAssistantMessage renders an AI response with markdown formatting
func (t *TUI) renderAssistantMessage(v *gocui.View, content string) {
	// Try to render as markdown first
	if t.markdownRenderer.IsEnabled() {
		rendered, err := t.markdownRenderer.Render(content)
		if err == nil {
			// Successfully rendered markdown
			fmt.Fprint(v, rendered)
			if !strings.HasSuffix(rendered, "\n") {
				fmt.Fprintln(v) // Ensure newline
			}
			return
		}
		// Log markdown render failure to debug
		t.addDebugMessage(fmt.Sprintf("Markdown render failed: %v", err))
	}
	
	// Fallback to plain text with themed color
	color := t.themeManager.GetANSIColor(ElementPrimary)
	fmt.Fprintf(v, "%s%s\033[0m\n", color, content)
}

// renderSystemMessage renders a system message with themed color and bullet
func (t *TUI) renderSystemMessage(v *gocui.View, content string) {
	color := t.themeManager.GetANSIColor(ElementInfo)
	fmt.Fprintf(v, "%s• %s\033[0m\n", color, content)
}

// renderErrorMessage renders an error message with themed color
func (t *TUI) renderErrorMessage(v *gocui.View, content string) {
	color := t.themeManager.GetANSIColor(ElementError)
	fmt.Fprintf(v, "%sError: %s\033[0m\n", color, content)
}

// renderToolMessage renders tool messages with appropriate colored circle indicators
func (t *TUI) renderToolMessage(v *gocui.View, content string, success *bool) {
	// Color the circle based on success state using theme colors
	if success == nil {
		// Tool call message (progress updates) - info color circle
		infoColor := t.themeManager.GetANSIColor(ElementInfo)
		colored := strings.Replace(content, "●", infoColor+"●\033[0m", 1)
		secondaryColor := t.themeManager.GetANSIColor(ElementSecondary)
		fmt.Fprintf(v, "%s%s\033[0m\n", secondaryColor, colored)
	} else if *success {
		// Success color circle for successful executions
		successColor := t.themeManager.GetANSIColor(ElementSuccess)
		colored := strings.Replace(content, "●", successColor+"●\033[0m", 1)
		fmt.Fprintf(v, "%s\n", colored)
	} else {
		// Error color circle for failed executions
		errorColor := t.themeManager.GetANSIColor(ElementError)
		colored := strings.Replace(content, "●", errorColor+"●\033[0m", 1)
		fmt.Fprintf(v, "%s\n", colored)
	}
}

// renderStatus renders the status bar
func (t *TUI) renderStatus(v *gocui.View) {
	v.Clear()
	
	if t.loading {
		elapsed := time.Since(t.requestTime)
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		idx := int(elapsed.Milliseconds()/100) % len(spinner)
		fmt.Fprintf(v, " %s %.1fs Thinking... (Press ESC to cancel)", spinner[idx], elapsed.Seconds())
	} else {
		fmt.Fprintf(v, " Ready")
	}
}

// formatFunctionCall formats a tool call with its parameters for display
func formatFunctionCall(toolName string, params map[string]any) string {
	// Make tool name bold using ANSI escape codes
	boldToolName := fmt.Sprintf("\033[1m%s\033[0m", toolName)
	
	if len(params) == 0 {
		return fmt.Sprintf("%s()", boldToolName)
	}

	var paramPairs []string
	for key, value := range params {
		// Format the value appropriately
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = fmt.Sprintf(`"%s"`, v)
		case bool:
			valueStr = fmt.Sprintf("%t", v)
		case nil:
			valueStr = "null"
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		paramPairs = append(paramPairs, fmt.Sprintf("%s: %s", key, valueStr))
	}

	// Sort for consistent display
	sort.Strings(paramPairs)

	return fmt.Sprintf("%s({%s})", boldToolName, strings.Join(paramPairs, ", "))
}

// renderNotification renders the notification panel
func (t *TUI) renderNotification(v *gocui.View) {
	v.Clear()
	
	// Check if we have a recent notification to show
	if t.notificationText != "" && time.Since(t.notificationTime) < 3*time.Second {
		// Show notification in the panel
		v.Title = " " + t.notificationText + " "
		fmt.Fprint(v, "Press any key to continue...")
	} else {
		// Hide the notification
		v.Title = ""
		fmt.Fprint(v, "")
	}
}

// showNotification displays a temporary notification message
func (t *TUI) showNotification(message string) {
	t.notificationText = message
	t.notificationTime = time.Now()
	
	// Debug: Log notification
	t.addDebugMessage(fmt.Sprintf("NOTIFICATION: %s", message))
	
	// Update the notification view
	if v, err := t.g.View(viewNotification); err == nil {
		t.renderNotification(v)
		t.addDebugMessage("Notification panel updated")
	} else {
		t.addDebugMessage(fmt.Sprintf("Notification view not found: %v", err))
	}
	
	// Audio feedback
	fmt.Print("\a") // Terminal bell sound
	
	// Automatically clear notification after 3 seconds
	go func() {
		time.Sleep(3 * time.Second)
		t.g.Update(func(g *gocui.Gui) error {
			if time.Since(t.notificationTime) >= 3*time.Second {
				t.notificationText = ""
				if v, err := g.View(viewNotification); err == nil {
					t.renderNotification(v)
				}
			}
			return nil
		})
	}()
}