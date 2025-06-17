package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/context"
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

// Custom messages for tea updates
type aiResponseMsg struct {
	response string
	err      error
	userInput string
}


// ReplModel holds the state for our REPL
type ReplModel struct {
	// UI components
	input    textinput.Model
	viewport viewport.Model
	spinner  spinner.Model

	// Chat state
	messages    []string
	ready       bool
	debug       bool
	loading     bool
	requestTime time.Time

	// AI integration
	llmClient        ai.Gen
	promptExecutor   ai.PromptExecutor
	markdownRenderer *glamour.TermRenderer

	// Session management
	sessionMgr     session.SessionManager
	currentSession session.Session
	historyMgr     history.HistoryManager
	contextMgr     context.ContextManager

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

	// Create spinner for loading state
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Initialize managers using Wire DI
	sessionMgr := di.ProvideSessionManager()
	historyMgr := di.ProvideHistoryManager()
	contextMgr := di.ProvideContextManager()

	// Initialize LLM client and prompt executor
	llmClient, err := di.InitializeGen()
	if err != nil {
		// If LLM initialization fails, we'll show an error in the REPL
		// but still allow the REPL to start for other functions
		llmClient = nil
	}
	
	promptExecutor, err := di.InitializePromptExecutor()
	if err != nil {
		// If prompt executor initialization fails, fall back to nil
		promptExecutor = nil
	}
	
	// Initialize markdown renderer
	markdownRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),                // Auto-detect dark/light theme
		glamour.WithWordWrap(vp.Width),         // Wrap to viewport width
	)
	if err != nil {
		// If markdown renderer fails, fall back to nil (will use plain text)
		markdownRenderer = nil
	}

	// Create initial session
	currentSession, _ := sessionMgr.CreateSession("repl-session")

	model := ReplModel{
		input:            ti,
		viewport:         vp,
		spinner:          s,
		messages:         []string{},
		llmClient:        llmClient,
		promptExecutor:   promptExecutor,
		markdownRenderer: markdownRenderer,
		sessionMgr:       sessionMgr,
		currentSession:   currentSession,
		historyMgr:       historyMgr,
		contextMgr:       contextMgr,
		ready:            false,
		loading:          false,
	}


	return model
}

// Init initializes the model (required by tea.Model interface)
func (m ReplModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// Update handles messages and updates the model
func (m ReplModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't handle input if we're loading
		if m.loading && msg.String() != "ctrl+c" {
			return m, nil
		}
		
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

	case aiResponseMsg:
		// AI response received - stop loading
		m.loading = false
		
		if msg.err != nil {
			m.addMessage(ErrorMessage, fmt.Sprintf("Failed to generate response: %v", msg.err))
		} else {
			// Add assistant response
			m.addMessage(AssistantMessage, msg.response)
			
			// Add to session (this will trigger our pubsub events)
			err := m.currentSession.AddInteraction(msg.userInput, msg.response)
			if err != nil {
				m.addMessage(ErrorMessage, fmt.Sprintf("Failed to add to session: %v", err))
			}
		}
		return m, nil
	
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 4 // space for input
		m.input.Width = msg.Width - 7 // border(2) + padding(2) + margin(3)

		// Update markdown renderer width if available
		if m.markdownRenderer != nil {
			// Create a new renderer with the updated width
			newRenderer, err := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width),
			)
			if err == nil {
				m.markdownRenderer = newRenderer
			}
		}

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
	
	var inputSection string
	if m.loading {
		// Calculate elapsed time
		elapsed := time.Since(m.requestTime)
		elapsedSeconds := elapsed.Seconds()
		
		// Show stopwatch and spinner above input when loading
		spinnerView := fmt.Sprintf(" %.1fs %s Thinking...", elapsedSeconds, m.spinner.View())
		inputSection = lipgloss.JoinVertical(lipgloss.Left, spinnerView, m.inputView())
	} else {
		inputSection = m.inputView()
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), inputSection)
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
		m.addMessage(SystemMessage, "/debug - Toggle debug mode")
		m.addMessage(SystemMessage, "/exit - Exit")

	case "/clear":
		m.messages = []string{}
		m.viewport.SetContent("")

	case "/debug":
		m.debug = !m.debug
		if m.debug {
			m.addMessage(SystemMessage, "Debug mode enabled")
		} else {
			m.addMessage(SystemMessage, "Debug mode disabled")
		}

	case "/exit", "/quit":
		return m, tea.Quit

	default:
		m.addMessage(ErrorMessage, "Unknown command. Type /help")
	}

	return m, nil
}

// handleAskCommand processes regular input as an ask command
func (m ReplModel) handleAskCommand(input string) (ReplModel, tea.Cmd) {
	// Check if prompt executor is available
	if m.promptExecutor == nil {
		m.addMessage(ErrorMessage, "Prompt executor not available. Please check your configuration.")
		return m, nil
	}

	// Set loading state and start spinner
	m.loading = true
	m.requestTime = time.Now()

	// Build conversation context from previous messages
	conversationContext := m.buildConversationContext()

	// Debug: Show what we're sending to AI if debug mode is enabled
	if m.debug {
		m.addMessage(SystemMessage, fmt.Sprintf("DEBUG - Context length: %d chars", len(conversationContext)))
		if conversationContext != "" {
			m.addMessage(SystemMessage, fmt.Sprintf("DEBUG - Context:\n%s", conversationContext))
		}
	}

	// Start AI request asynchronously
	return m, tea.Batch(
		m.spinner.Tick,
		m.makeAIRequest(input, conversationContext),
	)
}

// makeAIRequest creates a tea.Cmd that performs the AI request asynchronously
func (m ReplModel) makeAIRequest(userInput, context string) tea.Cmd {
	return func() tea.Msg {
		response, err := m.promptExecutor.Execute("conversation", m.debug, ai.Attr{
			Key:   "context",
			Value: context,
		}, ai.Attr{
			Key:   "message", 
			Value: userInput,
		})
		
		return aiResponseMsg{
			response:  response,
			err:       err,
			userInput: userInput,
		}
	}
}

// buildConversationContext creates a context string using the context manager
func (m ReplModel) buildConversationContext() string {
	// Use context manager to build conversation context
	const maxPairs = 5
	conversationContext, err := m.contextMgr.GetConversationContext(m.currentSession.GetID(), maxPairs)
	if err != nil {
		return ""
	}
	return conversationContext
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
		// Try to render as markdown first, fallback to plain text
		if m.markdownRenderer != nil {
			rendered, err := m.markdownRenderer.Render(content)
			if err == nil {
				// Successfully rendered markdown - apply AI style to the result
				msg = aiStyle.Render(strings.TrimSpace(rendered))
			} else {
				// Fallback to plain text wrapping
				wrapped := wordwrap.String(content, wrapWidth)
				msg = aiStyle.Render(wrapped)
			}
		} else {
			// No markdown renderer available - use plain text
			wrapped := wordwrap.String(content, wrapWidth)
			msg = aiStyle.Render(wrapped)
		}
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