package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/muesli/reflow/wordwrap"
)

// Message types for the chat
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	SystemMessage
	ErrorMessage
)


// ReplModel holds the state for our REPL
type ReplModel struct {
	// UI components
	input    textinput.Model
	viewport viewport.Model

	// Chat state
	messages []string
	ready    bool

	// AI integration
	llmClient ai.Gen

	// Session management
	sessionMgr     session.SessionManager
	currentSession session.Session
	historyMgr     history.HistoryManager

	// Dimensions
	width  int
	height int
}

// Styles
var (
	userStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	aiStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	sysStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	
	inputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 1)
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

	// Initialize managers using Wire DI
	sessionMgr := di.ProvideSessionManager()
	historyMgr := di.ProvideHistoryManager()

	// Initialize LLM client
	llmClient, err := di.InitializeGen()
	if err != nil {
		// If LLM initialization fails, we'll show an error in the REPL
		// but still allow the REPL to start for other functions
		llmClient = nil
	}

	// Create initial session
	currentSession, _ := sessionMgr.CreateSession("repl-session")

	model := ReplModel{
		input:          ti,
		viewport:       vp,
		messages:       []string{},
		llmClient:      llmClient,
		sessionMgr:     sessionMgr,
		currentSession: currentSession,
		historyMgr:     historyMgr,
		ready:          false,
	}


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
		case "up", "down", "pgup", "pgdown", "home", "end":
			// Handle viewport scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 4 // space for input
		m.input.Width = msg.Width - 7 // border(2) + padding(2) + margin(3)

		if !m.ready {
			m.ready = true
		}

		return m, nil
	}

	// Update input and viewport
	var inputCmd tea.Cmd
	var viewportCmd tea.Cmd
	
	m.input, inputCmd = m.input.Update(msg)
	m.viewport, viewportCmd = m.viewport.Update(msg)
	
	return m, tea.Batch(inputCmd, viewportCmd)
}

// View renders the model with viewport and input at bottom
func (m ReplModel) View() string {
	if !m.ready {
		return "Initializing Genie REPL..."
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), m.inputView())
}






// inputView renders the input area
func (m ReplModel) inputView() string {
	return inputStyle.Render(m.input.View())
}


// handleInput processes user input
func (m ReplModel) handleInput() (ReplModel, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}

	// Add user message to viewport
	m.addMessage(UserMessage, value)

	// Clear input
	m.input.SetValue("")

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
		m.addMessage(SystemMessage, "/clear - Clear chat")
		m.addMessage(SystemMessage, "/exit - Exit")

	case "/clear":
		m.messages = []string{}
		m.viewport.SetContent("")

	case "/exit", "/quit":
		return m, tea.Quit

	default:
		m.addMessage(ErrorMessage, "Unknown command. Type /help")
	}

	return m, nil
}

// handleAskCommand processes regular input as an ask command
func (m ReplModel) handleAskCommand(input string) (ReplModel, tea.Cmd) {
	// Check if LLM client is available
	if m.llmClient == nil {
		m.addMessage(ErrorMessage, "LLM client not available. Please check your GOOGLE_CLOUD_PROJECT environment variable.")
		return m, nil
	}

	// Build conversation context from previous messages
	conversationContext := m.buildConversationContext()
	
	// Create prompt with conversation context
	fullPrompt := conversationContext + "\n\nUser: " + input
	if conversationContext == "" {
		fullPrompt = input
	}

	aiPrompt := ai.Prompt{
		Name:        "repl-conversation",
		Text:        fullPrompt,
		Instruction: "You are a helpful AI assistant in an interactive conversation. Respond naturally and concisely. If this is a continuation of a conversation, acknowledge the context.",
		ModelName:   "gemini-1.5-flash",
		MaxTokens:   1000,
		Temperature: 0.7,
		TopP:        0.9,
	}

	// Call LLM (this is synchronous for now - could be made async later)
	response, err := m.llmClient.GenerateContent(aiPrompt, false)
	if err != nil {
		m.addMessage(ErrorMessage, fmt.Sprintf("Failed to generate response: %v", err))
		return m, nil
	}

	// Add assistant response
	m.addMessage(AssistantMessage, response)
	
	// Add to session (this will trigger our pubsub events)
	err = m.currentSession.AddInteraction(input, response)
	if err != nil {
		m.addMessage(ErrorMessage, fmt.Sprintf("Failed to add to session: %v", err))
	}

	return m, nil
}

// buildConversationContext creates a context string from session history
func (m ReplModel) buildConversationContext() string {
	// Get recent interactions from history manager
	history, err := m.historyMgr.GetHistory(m.currentSession.GetID())
	if err != nil {
		return ""
	}
	
	// Include last 5 conversation pairs to maintain context but keep prompt manageable
	const maxPairs = 5
	
	var context strings.Builder
	
	// History comes as alternating user/assistant messages
	// Only include complete pairs
	totalMessages := len(history)
	if totalMessages%2 != 0 {
		totalMessages-- // Remove incomplete pair
	}
	
	pairsToInclude := maxPairs
	if totalMessages/2 < maxPairs {
		pairsToInclude = totalMessages / 2
	}
	
	startIdx := totalMessages - (pairsToInclude * 2)
	
	for i := startIdx; i < totalMessages; i += 2 {
		context.WriteString(fmt.Sprintf("User: %s\n", history[i]))
		context.WriteString(fmt.Sprintf("Assistant: %s\n", history[i+1]))
	}
	
	return strings.TrimSpace(context.String())
}

// addMessage prints a message directly to the terminal
func (m *ReplModel) addMessage(msgType MessageType, content string) {
	// Wrap content to viewport width
	wrapWidth := m.viewport.Width
	if wrapWidth <= 0 {
		wrapWidth = 80 // fallback width
	}
	
	var msg string
	switch msgType {
	case UserMessage:
		wrapped := wordwrap.String("> "+content, wrapWidth)
		msg = userStyle.Render(wrapped)
	case AssistantMessage:
		wrapped := wordwrap.String(content, wrapWidth)
		msg = aiStyle.Render(wrapped)
	case SystemMessage:
		wrapped := wordwrap.String(content, wrapWidth)
		msg = sysStyle.Render(wrapped)
	case ErrorMessage:
		wrapped := wordwrap.String("Error: "+content, wrapWidth)
		msg = errStyle.Render(wrapped)
	}
	
	m.messages = append(m.messages, msg)
	m.viewport.SetContent(strings.Join(m.messages, "\n"))
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