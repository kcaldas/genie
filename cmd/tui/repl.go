package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kcaldas/genie/cmd/tui/history"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
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

type toolExecutedMsg struct {
	toolName   string
	message    string
	parameters map[string]any
	success    bool
	result     map[string]any
}

type confirmationRequestMsg struct {
	executionID string
	title       string
	message     string
}

type diffConfirmationRequestMsg struct {
	executionID string
	title       string
	filePath    string
	diffContent string
}

type userConfirmationRequestMsg struct {
	request events.UserConfirmationRequest
}

type diffConfirmationResponseMsg struct {
	executionID string
	confirmed   bool
}

type progressUpdateMsg struct {
	message string
}


// ReplModel holds the state for our REPL
type ReplModel struct {
	// UI components
	input    textinput.Model
	viewport viewport.Model
	spinner  spinner.Model

	// Chat state
	messages       []string
	ready          bool
	debug          bool
	loading        bool
	requestTime    time.Time
	
	// Request cancellation
	cancelCurrentRequest context.CancelFunc
	
	// Response tracking
	pendingResponses map[string]chan events.ChatResponseEvent
	responseMutex    sync.Mutex
	
	// Command history
	chatHistory    history.ChatHistory
	
	// TUI configuration
	tuiConfig      *Config

	// AI integration
	genieService     genie.Genie
	markdownRenderer *glamour.TermRenderer

	// Session management
	currentSession   *genie.Session
	sessionID        string
	
	// Event subscription
	subscriber       events.Subscriber
	program          **tea.Program // Reference to the tea program for sending events
	
	// Confirmation state
	confirmationDialog     *ConfirmationModel
	scrollableConfirmationDialog *ScrollableConfirmationModel
	publisher              events.Publisher
	
	// Context view state
	contextView            *ContextViewModel
	showingContextView     bool

	// Project management
	projectDir string

	// Dimensions
	width  int
	height int
	
	// Tool result management
	toolMessages    []toolExecutedMsg // Store tool messages for re-rendering
	toolMessageIds  []int            // Track positions of tool messages in messages slice
	toolsExpanded   bool             // Global toggle for tool result expansion
	
	// Initialization errors
	initError error
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
func InitialModel(genieInstance genie.Genie, initialSession *genie.Session) ReplModel {
	// Load TUI configuration
	tuiConfig, _ := LoadConfig() // Ignore error, use defaults
	
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

	// Get event bus components from the genieInstance (they're part of the core)
	eventBus := genieInstance.GetEventBus()
	subscriber := eventBus  // EventBus embeds Subscriber
	publisher := eventBus   // EventBus embeds Publisher
	
	// Initialize project directory (where genie was started)
	projectDir := initialSession.WorkingDirectory

	// Initialize TUI-specific chat history in the project .genie directory
	historyPath := filepath.Join(projectDir, ".genie", "history")
	chatHistory := history.NewChatHistory(historyPath, true) // Enable saving to disk
	chatHistory.Load() // Load existing history, ignore errors

	// Initialize markdown renderer
	markdownRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),                // Auto-detect dark/light theme
		glamour.WithWordWrap(vp.Width),         // Wrap to viewport width
	)
	if err != nil {
		// If markdown renderer fails, fall back to nil (will use plain text)
		markdownRenderer = nil
	}

	model := ReplModel{
		input:            ti,
		viewport:         vp,
		spinner:          s,
		messages:         []string{},
		genieService:     genieInstance,
		markdownRenderer: markdownRenderer,
		currentSession:   initialSession,
		subscriber:       subscriber,
		publisher:        publisher,
		projectDir:       projectDir,
		ready:            false,
		loading:          false,
		chatHistory:      chatHistory,
		initError:        nil,
		tuiConfig:        tuiConfig,
		pendingResponses: make(map[string]chan events.ChatResponseEvent),
		sessionID:        initialSession.ID,
	}
	
	// We'll set up the event subscription after the program is created
	var program *tea.Program
	model.program = &program


	return model
}

// Init initializes the model (required by tea.Model interface)
func (m ReplModel) Init() tea.Cmd {
	// Base commands to run
	var cmds []tea.Cmd
	
	// Add cursor blink if enabled in config
	if m.tuiConfig != nil && m.tuiConfig.CursorBlink {
		cmds = append(cmds, textinput.Blink)
	}
	
	// Always add spinner tick
	cmds = append(cmds, m.spinner.Tick)
	
	// Show initialization error if there was one
	if m.initError != nil {
		// Create a command that will add the error message after initialization
		showError := func() tea.Msg {
			return tea.WindowSizeMsg{} // Trigger a resize to show the error
		}
		cmds = append(cmds, showError)
	}
	
	return tea.Batch(cmds...)
}

// Update handles messages and updates the model
func (m ReplModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle context view first if active
		if m.showingContextView && m.contextView != nil {
			var cmd tea.Cmd
			var contextModel ContextViewModel
			contextModel, cmd = m.contextView.Update(msg)
			m.contextView = &contextModel
			return m, cmd
		}
		
		// Handle confirmation dialog first if active
		if m.confirmationDialog != nil {
			var cmd tea.Cmd
			var confirmationModel tea.Model
			confirmationModel, cmd = m.confirmationDialog.Update(msg)
			updatedConfirmation := confirmationModel.(ConfirmationModel)
			m.confirmationDialog = &updatedConfirmation
			return m, cmd
		}
		
		// Handle scrollable confirmation dialog if active
		if m.scrollableConfirmationDialog != nil {
			var cmd tea.Cmd
			var scrollableConfirmationModel tea.Model
			scrollableConfirmationModel, cmd = m.scrollableConfirmationDialog.Update(msg)
			updatedScrollableConfirmation := scrollableConfirmationModel.(ScrollableConfirmationModel)
			m.scrollableConfirmationDialog = &updatedScrollableConfirmation
			return m, cmd
		}
		
		// Don't handle input if we're loading (except for ctrl+c, esc, and our toggle/view keys)
		if m.loading && msg.String() != "ctrl+c" && msg.String() != "esc" && 
		   msg.String() != "ctrl+r" && msg.String() != "ctrl+e" && msg.String() != "f12" &&
		   msg.String() != "ctrl+/" && msg.String() != "ctrl+_" {
			return m, nil
		}
		
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+/", "ctrl+_": // Some terminals send ctrl+_ for ctrl+/
			// Open context view modal (shortcut for /context view)
			// TODO: Make keyboard shortcuts configurable via settings
			if m.genieService != nil {
				ctx := context.Background()
				contextData, err := m.genieService.GetContext(ctx, m.sessionID)
				if err == nil {
					contextView := NewContextView(contextData, m.width, m.height)
					m.contextView = &contextView
					m.showingContextView = true
				}
			}
			return m, nil
		case "ctrl+r", "ctrl+e", "f12":
			// Toggle tool result expansion and re-render (try multiple keys)
			m.toolsExpanded = !m.toolsExpanded
			m.rerenderToolMessages()
			
			// Don't pass this message to input field - consume it here
			return m, nil
		case "esc":
			// Cancel current request if one is in progress
			if m.loading && m.cancelCurrentRequest != nil {
				m.cancelCurrentRequest()
				// Don't show message here - let the AI response handler show "Request was cancelled"
				m.loading = false
				m.cancelCurrentRequest = nil
				return m, nil
			}
			return m, nil
		case "enter":
			return m.handleInput()
		case "up":
			return m.navigateHistory(1)
		case "down":
			return m.navigateHistory(-1)
		case "pgup", "pgdown", "home", "end":
			// Handle viewport scrolling for page navigation only
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		default:
			// Handle other keys normally
		}

	case aiResponseMsg:
		// AI response received - stop loading and clear cancel function
		m.loading = false
		m.cancelCurrentRequest = nil
		
		if msg.err != nil {
			// Check if it was a context cancellation (including wrapped errors)
			if errors.Is(msg.err, context.Canceled) || strings.Contains(msg.err.Error(), "canceled") || strings.Contains(msg.err.Error(), "cancelled") {
				m.addMessage(SystemMessage, "Request was cancelled")
			} else {
				m.addMessage(ErrorMessage, fmt.Sprintf("Failed to generate response: %v", msg.err))
			}
		} else {
			// Add assistant response
			m.addMessage(AssistantMessage, msg.response)
			
			// Note: Session interaction tracking is handled internally by Genie
		}
		return m, nil
	
	case toolExecutedMsg:
		// Tool execution event - store message and render with current expansion state
		m.toolMessages = append(m.toolMessages, msg)
		
		// Track the position where this tool message will be added
		messagePosition := len(m.messages)
		m.toolMessageIds = append(m.toolMessageIds, messagePosition)
		
		// Create a tool result component for better formatting
		toolResult := NewToolResult(msg.toolName, msg.parameters, msg.success, msg.result, m.toolsExpanded)
		
		// Add as a regular message (not system message) to remove background
		m.addToolMessage(toolResult.View())
		return m, nil
	
	case confirmationRequestMsg:
		// Tool confirmation request - create confirmation dialog
		confirmation := NewConfirmation(msg.title, msg.message, msg.executionID, m.width)
		m.confirmationDialog = &confirmation
		return m, nil
	
	case diffConfirmationRequestMsg:
		// Tool diff confirmation request - create scrollable confirmation dialog
		diffConfirmation := NewDiffConfirmation(msg.title, msg.filePath, msg.diffContent, msg.executionID, m.width, m.height)
		m.scrollableConfirmationDialog = &diffConfirmation
		return m, nil
	
	case userConfirmationRequestMsg:
		// User confirmation request - create scrollable confirmation dialog
		confirmation := NewScrollableConfirmation(msg.request, m.width, m.height)
		m.scrollableConfirmationDialog = &confirmation
		return m, nil
	
	case confirmationResponseMsg:
		// Handle confirmation response
		if msg.confirmed {
			// User said "Yes" - proceed with the tool execution
			if m.publisher != nil {
				response := events.ToolConfirmationResponse{
					ExecutionID: msg.executionID,
					Confirmed:   true,
				}
				m.publisher.Publish(response.Topic(), response)
			}
		} else {
			// User said "No" - cancel the current request context (like pressing ESC)
			if m.loading && m.cancelCurrentRequest != nil {
				m.cancelCurrentRequest()
				m.loading = false
				m.cancelCurrentRequest = nil
				m.addMessage(SystemMessage, "Request was cancelled")
			}
			
			// Still send the "No" response to the tool system to clean up
			if m.publisher != nil {
				response := events.ToolConfirmationResponse{
					ExecutionID: msg.executionID,
					Confirmed:   false,
				}
				m.publisher.Publish(response.Topic(), response)
			}
		}
		
		// Clear confirmation dialog
		m.confirmationDialog = nil
		return m, nil
	
	case scrollableConfirmationResponseMsg:
		// Handle scrollable confirmation response
		if m.publisher != nil {
			response := events.UserConfirmationResponse{
				ExecutionID: msg.executionID,
				Confirmed:   msg.confirmed,
			}
			m.publisher.Publish(response.Topic(), response)
		}
		// Close scrollable confirmation dialog
		m.scrollableConfirmationDialog = nil
		return m, nil

	case closeContextViewMsg:
		// Close context view modal
		m.showingContextView = false
		m.contextView = nil
		return m, nil
	
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		// Ignore invalid window sizes (happens during initialization)
		if msg.Width < 20 || msg.Height < 10 {
			return m, nil
		}
		
		m.width = msg.Width
		m.height = msg.Height
		
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 4 // space for input
		m.input.Width = msg.Width - 7 // border(2) + padding(2) + margin(3)

		// Update context view if active
		if m.showingContextView && m.contextView != nil {
			var cmd tea.Cmd
			var contextModel ContextViewModel
			contextModel, cmd = m.contextView.Update(msg)
			m.contextView = &contextModel
			return m, cmd
		}

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
			// Show initialization error after the window is ready
			if m.initError != nil {
				m.addMessage(ErrorMessage, fmt.Sprintf("Initialization warning: %v", m.initError))
				m.addMessage(SystemMessage, "Some features may be unavailable. Type /help for available commands.")
			}
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
	
	// Show context view modal if active
	if m.showingContextView && m.contextView != nil {
		return m.contextView.View()
	}
	
	var inputSection string
	if m.confirmationDialog != nil {
		// Show confirmation dialog instead of input
		inputSection = m.confirmationDialog.View()
	} else if m.scrollableConfirmationDialog != nil {
		// Show scrollable confirmation dialog instead of input
		inputSection = m.scrollableConfirmationDialog.View()
	} else if m.loading {
		// Calculate elapsed time
		elapsed := time.Since(m.requestTime)
		elapsedSeconds := elapsed.Seconds()
		
		// Show stopwatch and spinner above input when loading
		spinnerView := fmt.Sprintf(" %.1fs %s Thinking... (Press ESC to cancel)", elapsedSeconds, m.spinner.View())
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



// navigateHistory moves through command history
func (m ReplModel) navigateHistory(direction int) (ReplModel, tea.Cmd) {
	var command string
	
	// Use ChatHistory navigation methods
	if direction > 0 {
		// Moving to older commands (up arrow)
		command = m.chatHistory.NavigatePrev()
	} else {
		// Moving to newer commands (down arrow)
		command = m.chatHistory.NavigateNext()
	}
	
	// Set input text
	m.input.SetValue(command)
	if command != "" {
		// Move cursor to end of input
		m.input.CursorEnd()
	}
	
	return m, nil
}

// handleInput processes user input
func (m ReplModel) handleInput() (ReplModel, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}

	// Note: Confirmation handling is now done in the Update method via confirmationDialog

	// Add to TUI chat history with persistence (automatically resets navigation)
	m.chatHistory.AddCommand(value)

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
		m.addMessage(SystemMessage, "/config - Manage TUI settings")
		m.addMessage(SystemMessage, "/context view - Open context viewer modal")
		m.addMessage(SystemMessage, "/debug - Toggle debug mode")
		m.addMessage(SystemMessage, "/exit - Exit")
		m.addMessage(SystemMessage, "")
		m.addMessage(SystemMessage, fmt.Sprintf("Project: %s", m.projectDir))
		m.addMessage(SystemMessage, "")
		m.addMessage(SystemMessage, "Navigation:")
		m.addMessage(SystemMessage, "↑/↓ - Navigate command history (stored in .genie/history)")
		m.addMessage(SystemMessage, "PgUp/PgDn - Scroll chat")
		m.addMessage(SystemMessage, "")
		m.addMessage(SystemMessage, "Shortcuts:")
		m.addMessage(SystemMessage, "Ctrl+/ (or Ctrl+_) - Open context viewer")

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

	case "/config":
		return m.handleConfigCommand(parts)

	case "/context":
		return m.handleContextCommand(parts)

	case "/exit", "/quit":
		return m, tea.Quit

	default:
		m.addMessage(ErrorMessage, "Unknown command. Type /help")
	}

	return m, nil
}

// handleConfigCommand processes /config commands
func (m ReplModel) handleConfigCommand(parts []string) (ReplModel, tea.Cmd) {
	if len(parts) == 1 {
		// Show current config
		if m.tuiConfig != nil {
			m.addMessage(SystemMessage, "Current TUI Configuration:")
			m.addMessage(SystemMessage, fmt.Sprintf("  cursor_blink: %t", m.tuiConfig.CursorBlink))
			m.addMessage(SystemMessage, fmt.Sprintf("  chat_timeout_seconds: %d", m.tuiConfig.ChatTimeoutSeconds))
			m.addMessage(SystemMessage, "")
			m.addMessage(SystemMessage, "Usage:")
			m.addMessage(SystemMessage, "  /config show              - Show current settings")
			m.addMessage(SystemMessage, "  /config set <key> <value> - Change a setting")
			m.addMessage(SystemMessage, "")
			m.addMessage(SystemMessage, "Available settings:")
			m.addMessage(SystemMessage, "  cursor_blink (true/false) - Enable/disable cursor blinking")
			m.addMessage(SystemMessage, "  chat_timeout_seconds (number) - Chat request timeout in seconds")
		} else {
			m.addMessage(ErrorMessage, "TUI configuration not available")
		}
		return m, nil
	}

	subCommand := parts[1]
	switch subCommand {
	case "show":
		if m.tuiConfig != nil {
			m.addMessage(SystemMessage, "Current TUI Configuration:")
			m.addMessage(SystemMessage, fmt.Sprintf("  cursor_blink: %t", m.tuiConfig.CursorBlink))
			m.addMessage(SystemMessage, fmt.Sprintf("  chat_timeout_seconds: %d", m.tuiConfig.ChatTimeoutSeconds))
		} else {
			m.addMessage(ErrorMessage, "TUI configuration not available")
		}

	case "set":
		if len(parts) < 4 {
			m.addMessage(ErrorMessage, "Usage: /config set <key> <value>")
			return m, nil
		}
		
		key := parts[2]
		value := parts[3]
		
		if m.tuiConfig == nil {
			m.addMessage(ErrorMessage, "TUI configuration not available")
			return m, nil
		}
		
		switch key {
		case "cursor_blink":
			if value == "true" {
				m.tuiConfig.CursorBlink = true
				m.addMessage(SystemMessage, "Cursor blinking enabled. Restart REPL to apply changes.")
			} else if value == "false" {
				m.tuiConfig.CursorBlink = false
				m.addMessage(SystemMessage, "Cursor blinking disabled. Restart REPL to apply changes.")
			} else {
				m.addMessage(ErrorMessage, "cursor_blink must be 'true' or 'false'")
				return m, nil
			}
			
			// Save config
			if err := m.tuiConfig.Save(); err != nil {
				m.addMessage(ErrorMessage, fmt.Sprintf("Failed to save config: %v", err))
			} else {
				m.addMessage(SystemMessage, "Configuration saved successfully")
			}
			
		case "chat_timeout_seconds":
			timeout, err := strconv.Atoi(value)
			if err != nil || timeout <= 0 {
				m.addMessage(ErrorMessage, "chat_timeout_seconds must be a positive number")
				return m, nil
			}
			m.tuiConfig.ChatTimeoutSeconds = timeout
			m.addMessage(SystemMessage, fmt.Sprintf("Chat timeout set to %d seconds", timeout))
			
			// Save config
			if err := m.tuiConfig.Save(); err != nil {
				m.addMessage(ErrorMessage, fmt.Sprintf("Failed to save config: %v", err))
			} else {
				m.addMessage(SystemMessage, "Configuration saved successfully")
			}
			
		default:
			m.addMessage(ErrorMessage, fmt.Sprintf("Unknown configuration key: %s", key))
		}

	default:
		m.addMessage(ErrorMessage, "Unknown config command. Use: show, set")
	}

	return m, nil
}

// handleContextCommand processes /context commands
func (m ReplModel) handleContextCommand(parts []string) (ReplModel, tea.Cmd) {
	if len(parts) == 1 {
		// Show help for context command
		m.addMessage(SystemMessage, "Context Management:")
		m.addMessage(SystemMessage, "  /context view  - Open context viewer modal (ESC to close)")
		m.addMessage(SystemMessage, "  /context clean - Clear context")
		return m, nil
	}

	subCommand := parts[1]
	switch subCommand {
	case "view":
		// Get context from Genie service
		if m.genieService == nil {
			m.addMessage(ErrorMessage, "Genie service not available")
			return m, nil
		}
		
		ctx := context.Background()
		contextData, err := m.genieService.GetContext(ctx, m.sessionID)
		if err != nil {
			m.addMessage(ErrorMessage, fmt.Sprintf("Failed to get context: %v", err))
			return m, nil
		}
		
		// Open context view modal
		contextView := NewContextView(contextData, m.width, m.height)
		m.contextView = &contextView
		m.showingContextView = true

	case "clean":
		// TODO: Add ClearContext method to Genie interface and implement
		m.addMessage(SystemMessage, "Context clearing not yet implemented")

	default:
		m.addMessage(ErrorMessage, "Unknown context command. Use: view, clean")
	}

	return m, nil
}

// handleAskCommand processes regular input as an ask command
func (m ReplModel) handleAskCommand(input string) (ReplModel, tea.Cmd) {
	// Check if Genie service is available
	if m.genieService == nil {
		if m.initError != nil {
			m.addMessage(ErrorMessage, fmt.Sprintf("AI features unavailable: %v", m.initError))
		} else {
			m.addMessage(ErrorMessage, "AI features unavailable. Please check your configuration.")
		}
		return m, nil
	}

	// Set loading state and start spinner
	m.loading = true
	m.requestTime = time.Now()

	// Create cancellable context for this request
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelCurrentRequest = cancel

	// Start AI request asynchronously with cancellable context
	return m, tea.Batch(
		m.spinner.Tick,
		m.makeAIRequestWithContext(ctx, input),
	)
}

// makeAIRequestWithContext creates a tea.Cmd that performs the AI request asynchronously with a cancellable context
func (m ReplModel) makeAIRequestWithContext(ctx context.Context, userInput string) tea.Cmd {
	return func() tea.Msg {
		// Use the Genie service for chat processing (includes output formatting)
		if m.genieService == nil {
			return aiResponseMsg{err: fmt.Errorf("Genie service not available")}
		}
		
		// Use the session ID from the model
		sessionID := m.sessionID
		
		// Use Genie service to process the chat message
		// This handles LLM calls, tool formatting, and all the service layer logic
		err := m.genieService.Chat(ctx, sessionID, userInput)
		if err != nil {
			return aiResponseMsg{err: err}
		}
		
		// The Genie service processes asynchronously and publishes events
		// Create a channel for this specific request
		responseChan := make(chan events.ChatResponseEvent, 1)
		
		// Register this request's channel
		m.responseMutex.Lock()
		m.pendingResponses[sessionID] = responseChan
		m.responseMutex.Unlock()
		
		// Clean up when done
		defer func() {
			m.responseMutex.Lock()
			delete(m.pendingResponses, sessionID)
			close(responseChan)
			m.responseMutex.Unlock()
		}()
		
		// Wait for response, timeout, or cancellation
		timeoutDuration := time.Duration(m.tuiConfig.ChatTimeoutSeconds) * time.Second
		if timeoutDuration <= 0 {
			timeoutDuration = 3 * time.Minute // Fallback to 3 minutes
		}
		timeout := time.After(timeoutDuration)
		ticker := time.NewTicker(20 * time.Second) // Progress updates every 20 seconds
		defer ticker.Stop()
		
		for {
			select {
			case response := <-responseChan:
				return aiResponseMsg{
					response:  response.Response,
					err:       response.Error,
					userInput: userInput,
				}
			case <-ctx.Done():
				return aiResponseMsg{
					err:       fmt.Errorf("request cancelled"),
					userInput: userInput,
				}
			case <-timeout:
				return aiResponseMsg{
					err:       fmt.Errorf("request timed out after %s", timeoutDuration),
					userInput: userInput,
				}
			case <-ticker.C:
				// Continue waiting, View will show elapsed time
			}
		}
	}
}

// GetProjectDir returns the current project directory
func (m ReplModel) GetProjectDir() string {
	return m.projectDir
}



// rerenderToolMessages re-renders all tool messages with current expansion state
func (m *ReplModel) rerenderToolMessages() {
	// Re-render tool messages using their tracked positions
	for i, toolMsg := range m.toolMessages {
		if i < len(m.toolMessageIds) {
			messagePos := m.toolMessageIds[i]
			if messagePos < len(m.messages) {
				// Re-render this specific tool message
				toolResult := NewToolResult(toolMsg.toolName, toolMsg.parameters, toolMsg.success, toolMsg.result, m.toolsExpanded)
				m.messages[messagePos] = toolResult.View()
			}
		}
	}
	
	// Update viewport content
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
	m.viewport.GotoBottom()
}

// addToolMessage adds a tool execution message without background styling
func (m *ReplModel) addToolMessage(content string) {
	// Wrap content to viewport width
	wrapWidth := m.viewport.Width
	if wrapWidth <= 0 {
		wrapWidth = 80 // fallback width
	}
	
	// Just wrap the text without any special styling for background
	wrapped := wordwrap.String(content, wrapWidth)
	
	m.messages = append(m.messages, wrapped)
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
	m.viewport.GotoBottom()
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
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
	m.viewport.GotoBottom()
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


// StartREPL initializes and runs the REPL
func StartREPL(genieInstance genie.Genie, initialSession *genie.Session) {
	// Set up logging for REPL mode (quiet by default)
	logger := logging.NewQuietLogger()
	logging.SetGlobalLogger(logger)

	// Create initial model
	model := InitialModel(genieInstance, initialSession)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use the full terminal
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Set the program reference in the model for event handling
	if model.program != nil {
		*model.program = p
	}

	// Now set up the event subscription with the program reference
	model.subscriber.Subscribe("tool.executed", func(event interface{}) {
		if toolEvent, ok := event.(events.ToolExecutedEvent); ok {
			// Format the function call display
			formattedCall := formatFunctionCall(toolEvent.ToolName, toolEvent.Parameters)
			
			// Determine success based on the message (no "Failed:" prefix means success)
			success := !strings.HasPrefix(toolEvent.Message, "Failed:")
			
			// Send a Bubble Tea message to update the UI
			p.Send(toolExecutedMsg{
				toolName:   toolEvent.ToolName,
				message:    formattedCall,
				parameters: toolEvent.Parameters,
				success:    success,
				result:     toolEvent.Result,
			})
		}
	})

	// Subscribe to tool confirmation requests
	model.subscriber.Subscribe("tool.confirmation.request", func(event interface{}) {
		if confirmationEvent, ok := event.(events.ToolConfirmationRequest); ok {
			// Send a Bubble Tea message to show confirmation dialog
			p.Send(confirmationRequestMsg{
				executionID: confirmationEvent.ExecutionID,
				title:       confirmationEvent.ToolName,
				message:     confirmationEvent.Command,
			})
		}
	})

	// Subscribe to user confirmation requests
	model.subscriber.Subscribe("user.confirmation.request", func(event interface{}) {
		if confirmationEvent, ok := event.(events.UserConfirmationRequest); ok {
			// Send a Bubble Tea message to show confirmation dialog
			p.Send(userConfirmationRequestMsg{
				request: confirmationEvent,
			})
		}
	})
	
	// Subscribe to chat responses permanently
	model.subscriber.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(events.ChatResponseEvent); ok {
			// Route response to waiting channel if exists
			model.responseMutex.Lock()
			if ch, exists := model.pendingResponses[resp.SessionID]; exists {
				select {
				case ch <- resp:
					// Successfully sent response
				default:
					// Channel full or closed, ignore
				}
			}
			model.responseMutex.Unlock()
		}
	})

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running REPL: %v\n", err)
		os.Exit(1)
	}
}