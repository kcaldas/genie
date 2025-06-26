// Package tui provides terminal user interface components for Genie.
// This file contains the messages component for displaying chat conversations.
package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/kcaldas/genie/cmd/tui/theme"
	"github.com/kcaldas/genie/cmd/tui/toolresult"
	"github.com/muesli/reflow/wordwrap"
)

// MessageType represents different types of messages
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	SystemMessage
	ErrorMessage
	ToolCallMessage
)

// Model represents a messages component that displays chat conversations.
// It follows the Bubble Tea component patterns with immutable updates and
// proper separation of concerns for rendering, state management, and user interaction.
type Model struct {
	viewport         viewport.Model
	messages         []string
	markdownRenderer *glamour.TermRenderer
	
	// Tool message tracking for re-rendering
	toolMessages   []toolExecutedMsg
	toolMessageIds []int
	toolsExpanded  bool
}

// Message styles are now provided by the theme system
// Accessed via theme.Styles().UserMessage, etc.

// New creates a new messages view following Bubbles patterns
func New(width, height int) Model {
	vp := viewport.New(width, height)
	vp.SetContent("")

	// Initialize markdown renderer
	markdownRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(vp.Width),
	)
	if err != nil {
		markdownRenderer = nil // Fallback to plain text
	}

	return Model{
		viewport:         vp,
		messages:         []string{},
		markdownRenderer: markdownRenderer,
		toolMessages:     []toolExecutedMsg{},
		toolMessageIds:   []int{},
		toolsExpanded:    false,
	}
}

// SetSize updates the dimensions of the messages view
func (m Model) SetSize(width, height int) Model {
	m.viewport.Width = width
	m.viewport.Height = height

	// Update markdown renderer width if available
	if m.markdownRenderer != nil {
		newRenderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(m.viewport.Width),
		)
		if err == nil {
			m.markdownRenderer = newRenderer
		}
	}
	return m
}

// AddMessage adds a new message to the view
func (m Model) AddMessage(msgType MessageType, content string) Model {
	wrapWidth := m.viewport.Width
	if wrapWidth <= 0 {
		wrapWidth = 80 // fallback width
	}

	// Get current theme styles
	styles := theme.GetStyles()
	
	var msg string
	switch msgType {
	case UserMessage:
		wrapped := wordwrap.String("> "+content, wrapWidth)
		msg = styles.UserMessage.Render(wrapped)
	case AssistantMessage:
		// Try to render as markdown first, fallback to plain text
		if m.markdownRenderer != nil {
			rendered, err := m.markdownRenderer.Render(content)
			if err == nil {
				// Successfully rendered markdown - apply AI style to the result
				msg = styles.AIMessage.Render(strings.TrimSpace(rendered))
			} else {
				// Fallback to plain text wrapping
				wrapped := wordwrap.String(content, wrapWidth)
				msg = styles.AIMessage.Render(wrapped)
			}
		} else {
			// No markdown renderer available - use plain text
			wrapped := wordwrap.String(content, wrapWidth)
			msg = styles.AIMessage.Render(wrapped)
		}
	case SystemMessage:
		wrapped := wordwrap.String(content, wrapWidth)
		msg = styles.SystemMessage.Render(wrapped)
	case ErrorMessage:
		wrapped := wordwrap.String("Error: "+content, wrapWidth)
		msg = styles.ErrorMessage.Render(wrapped)
	case ToolCallMessage:
		// Add circle indicator for tool call messages using consistent styling
		wrapped := wordwrap.String("● "+content, wrapWidth)
		msg = styles.ToolCallMessage.Render(wrapped)
	}

	m.messages = append(m.messages, msg)
	m = m.updateViewport()
	return m
}

// AddToolMessage adds a tool execution message
func (m Model) AddToolMessage(toolMsg toolExecutedMsg) Model {
	// Store tool message for re-rendering
	m.toolMessages = append(m.toolMessages, toolMsg)

	// Track the position where this tool message will be added
	messagePosition := len(m.messages)
	m.toolMessageIds = append(m.toolMessageIds, messagePosition)

	// Create a tool result component for better formatting
	toolResult := toolresult.New(toolMsg.toolName, toolMsg.parameters, toolMsg.success, toolMsg.result, m.toolsExpanded)

	// Wrap content to viewport width
	wrapWidth := m.viewport.Width
	if wrapWidth <= 0 {
		wrapWidth = 80 // fallback width
	}

	// Just wrap the text without any special styling for background
	wrapped := wordwrap.String(toolResult.View(), wrapWidth)

	m.messages = append(m.messages, wrapped)
	m = m.updateViewport()
	return m
}

// ToggleToolsExpanded toggles the expansion state of tool results and re-renders
func (m Model) ToggleToolsExpanded() Model {
	m.toolsExpanded = !m.toolsExpanded
	m = m.rerenderToolMessages()
	return m
}

// rerenderToolMessages re-renders all tool messages with current expansion state
func (m Model) rerenderToolMessages() Model {
	// Re-render tool messages using their tracked positions
	for i, toolMsg := range m.toolMessages {
		if i < len(m.toolMessageIds) {
			messagePos := m.toolMessageIds[i]
			if messagePos < len(m.messages) {
				// Re-render this specific tool message
				toolResult := toolresult.New(toolMsg.toolName, toolMsg.parameters, toolMsg.success, toolMsg.result, m.toolsExpanded)
				m.messages[messagePos] = toolResult.View()
			}
		}
	}

	m = m.updateViewport()
	return m
}

// Clear removes all messages
func (m Model) Clear() Model {
	m.messages = []string{}
	m.toolMessages = []toolExecutedMsg{}
	m.toolMessageIds = []int{}
	m = m.updateViewport()
	return m
}

// updateViewport updates the viewport content with bottom padding
func (m Model) updateViewport() Model {
	viewportContent := strings.Join(m.messages, "\n\n") + "\n" // Add bottom padding
	m.viewport.SetContent(viewportContent)
	m.viewport.GotoBottom()
	return m
}

// View returns the rendered view
func (m Model) View() string {
	return m.viewport.View()
}

// Update handles viewport updates (for scrolling, etc.) following Bubbles patterns
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// GetMessages returns all current messages (for testing)
func (m Model) GetMessages() []string {
	return m.messages
}

// GetMessageCount returns the number of messages (for testing)
func (m Model) GetMessageCount() int {
	return len(m.messages)
}

// formatFunctionCall formats a function call for display like: readFile({file_path: "README.md"})
func formatFunctionCall(toolName string, params map[string]any) string {
	if len(params) == 0 {
		return fmt.Sprintf("%s()", toolName)
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

	return fmt.Sprintf("%s({%s})", toolName, strings.Join(paramPairs, ", "))
}