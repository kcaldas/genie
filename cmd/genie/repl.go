package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/session"
)

// Message types for the chat
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	SystemMessage
	ErrorMessage
)

// Message represents a chat message
type Message struct {
	Type      MessageType
	Content   string
	Timestamp time.Time
}

// ReplModel holds the state for our REPL
type ReplModel struct {
	// UI components
	input    textinput.Model
	viewport viewport.Model

	// Chat state
	messages []Message
	ready    bool

	// Session management
	sessionMgr     session.SessionManager
	currentSession session.Session

	// Dimensions
	width  int
	height int
}

// Styles
var (
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#10B981")
	errorColor     = lipgloss.Color("#EF4444")
	mutedColor     = lipgloss.Color("#6B7280")

	headerStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	messageUserStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				MarginTop(1).
				MarginBottom(0)

	messageAssistantStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				MarginTop(0).
				MarginBottom(1)

	messageSystemStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Italic(true).
				MarginTop(0).
				MarginBottom(1)

	messageErrorStyle = lipgloss.NewStyle().
				Foreground(errorColor).
				MarginTop(0).
				MarginBottom(1)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)
)

// InitialModel creates the initial model for the REPL
func InitialModel() ReplModel {
	// Create text input
	ti := textinput.New()
	ti.Placeholder = "Type your message or /help for commands..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 50

	// Create viewport for messages
	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Initialize session manager using Wire DI
	sessionMgr := di.ProvideSessionManager()

	// Create initial session
	currentSession, _ := sessionMgr.CreateSession("repl-session")

	model := ReplModel{
		input:          ti,
		viewport:       vp,
		messages:       []Message{},
		sessionMgr:     sessionMgr,
		currentSession: currentSession,
		ready:          false,
	}

	// Add welcome message
	model.addMessage(SystemMessage, "Welcome to Genie REPL! Type /help for available commands.")

	return model
}

// Init initializes the model (required by tea.Model interface)
func (m ReplModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the model
func (m ReplModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m.handleInput()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport size
		headerHeight := 3
		footerHeight := 4
		inputHeight := 3
		verticalMargins := headerHeight + footerHeight + inputHeight

		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - verticalMargins

		// Update input width
		m.input.Width = msg.Width - 6

		if !m.ready {
			m.ready = true
		}

		return m, nil
	}

	// Update input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

// View renders the model
func (m ReplModel) View() string {
	if !m.ready {
		return "Initializing Genie REPL..."
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.headerView(),
		m.messagesView(),
		m.inputView(),
		m.footerView(),
	)
}

// headerView renders the header
func (m ReplModel) headerView() string {
	sessionInfo := fmt.Sprintf("Session: %s", m.currentSession.GetID())
	title := headerStyle.Render(fmt.Sprintf("Genie REPL v%s | %s", version, sessionInfo))
	return title
}

// messagesView renders the chat messages
func (m ReplModel) messagesView() string {
	var content strings.Builder

	for _, msg := range m.messages {
		timestamp := msg.Timestamp.Format("15:04:05")
		
		switch msg.Type {
		case UserMessage:
			content.WriteString(messageUserStyle.Render(fmt.Sprintf("[%s] > %s", timestamp, msg.Content)))
		case AssistantMessage:
			content.WriteString(messageAssistantStyle.Render(msg.Content))
		case SystemMessage:
			content.WriteString(messageSystemStyle.Render(fmt.Sprintf("[%s] %s", timestamp, msg.Content)))
		case ErrorMessage:
			content.WriteString(messageErrorStyle.Render(fmt.Sprintf("[%s] Error: %s", timestamp, msg.Content)))
		}
		content.WriteString("\n")
	}

	m.viewport.SetContent(content.String())
	return m.viewport.View()
}

// inputView renders the input area
func (m ReplModel) inputView() string {
	return inputStyle.Render(m.input.View())
}

// footerView renders the footer with help text
func (m ReplModel) footerView() string {
	help := "/help | /status | /clear | exit | Ctrl+C"
	return footerStyle.Render(help)
}

// handleInput processes user input
func (m ReplModel) handleInput() (ReplModel, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}

	// Clear input
	m.input.SetValue("")

	// Add user message
	m.addMessage(UserMessage, value)

	// Handle commands
	if strings.HasPrefix(value, "/") {
		return m.handleSlashCommand(value)
	}

	// Handle as ask command
	return m.handleAskCommand(value)
}

// handleSlashCommand processes slash commands
func (m ReplModel) handleSlashCommand(cmd string) (ReplModel, tea.Cmd) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return m, nil
	}

	command := parts[0]

	switch command {
	case "/help":
		m.addMessage(SystemMessage, "Available commands:")
		m.addMessage(SystemMessage, "  /help - Show this help")
		m.addMessage(SystemMessage, "  /status - Show system status")
		m.addMessage(SystemMessage, "  /clear - Clear chat history")
		m.addMessage(SystemMessage, "  /session - Session management")
		m.addMessage(SystemMessage, "  exit - Exit REPL")
		m.addMessage(SystemMessage, "  Or just type a message to ask AI")

	case "/status":
		m.addMessage(SystemMessage, "Status: Ready")
		m.addMessage(SystemMessage, "Model: Not configured yet")
		m.addMessage(SystemMessage, fmt.Sprintf("Session: %s", m.currentSession.GetID()))

	case "/clear":
		m.messages = []Message{}
		m.addMessage(SystemMessage, "Chat history cleared")

	case "/session":
		if len(parts) > 1 && parts[1] == "new" {
			// Create new session
			newSession, err := m.sessionMgr.CreateSession(fmt.Sprintf("session-%d", time.Now().Unix()))
			if err != nil {
				m.addMessage(ErrorMessage, fmt.Sprintf("Failed to create session: %v", err))
			} else {
				m.currentSession = newSession
				m.addMessage(SystemMessage, fmt.Sprintf("Created and switched to session: %s", newSession.GetID()))
			}
		} else {
			m.addMessage(SystemMessage, fmt.Sprintf("Current session: %s", m.currentSession.GetID()))
			m.addMessage(SystemMessage, "Usage: /session new")
		}

	case "/exit", "/quit":
		return m, tea.Quit

	default:
		m.addMessage(ErrorMessage, fmt.Sprintf("Unknown command: %s. Type /help for available commands.", command))
	}

	return m, nil
}

// handleAskCommand processes regular input as an ask command
func (m ReplModel) handleAskCommand(input string) (ReplModel, tea.Cmd) {
	// For now, just echo back - we'll integrate with the real LLM later
	m.addMessage(AssistantMessage, fmt.Sprintf("Echo: %s", input))
	
	// Add to session (this will trigger our pubsub events)
	err := m.currentSession.AddInteraction(input, fmt.Sprintf("Echo: %s", input))
	if err != nil {
		m.addMessage(ErrorMessage, fmt.Sprintf("Failed to add to session: %v", err))
	}

	return m, nil
}

// addMessage adds a message to the chat
func (m *ReplModel) addMessage(msgType MessageType, content string) {
	msg := Message{
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)

	// Auto-scroll to bottom
	m.viewport.GotoBottom()
}

// startRepl initializes and runs the REPL
func startRepl() {
	// Set up logging for REPL mode (quiet by default)
	logger := logging.NewQuietLogger()
	logging.SetGlobalLogger(logger)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(
		InitialModel(),
		tea.WithAltScreen(),       // Use the full terminal
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running REPL: %v\n", err)
		os.Exit(1)
	}
}