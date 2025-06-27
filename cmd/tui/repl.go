package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kcaldas/genie/cmd/tui/confirmation"
	"github.com/kcaldas/genie/cmd/tui/contextview"
	"github.com/kcaldas/genie/cmd/tui/history"
	"github.com/kcaldas/genie/cmd/tui/scrollconfirm"
	"github.com/kcaldas/genie/cmd/tui/theme"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

// Message types are now defined in messages_view.go

// Custom messages for tea updates
type aiResponseMsg struct {
	response  string
	err       error
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

type progressUpdateMsg struct {
	message string
}

type toolCallMessageMsg struct {
	toolName string
	message  string
}

// ReplModel holds the state for our REPL
type ReplModel struct {
	// UI components
	input        textinput.Model
	messagesView Model // From messages_view.go
	spinner      spinner.Model

	// Chat state
	ready       bool
	debug       bool
	loading     bool
	requestTime time.Time

	// Request cancellation
	cancelCurrentRequest context.CancelFunc

	// Response tracking removed - using direct callbacks now

	// Command history
	chatHistory history.ChatHistory

	// TUI configuration
	tuiConfig *Config

	// AI integration
	genieService genie.Genie

	// Session management - single session for command app
	currentSession *genie.Session

	// Event subscription
	subscriber events.Subscriber
	program    **tea.Program // Reference to the tea program for sending events

	// Confirmation state
	confirmationDialog           *confirmation.Model
	scrollableConfirmationDialog *scrollconfirm.Model
	publisher                    events.Publisher

	// Context view state
	contextView        *contextview.Model
	showingContextView bool

	// Project management
	projectDir string

	// Dimensions
	width  int
	height int

	// Tool result management now handled by MessagesView

	// Initialization errors
	initError error
}

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

	// Set cursor blink based on config
	if tuiConfig != nil && !tuiConfig.CursorBlink {
		// Disable cursor blinking by setting a non-blinking cursor mode
		ti.Cursor.SetMode(cursor.CursorStatic)
	} else {
		// Enable cursor blinking (default)
		ti.Cursor.SetMode(cursor.CursorBlink)
	}

	// Create messages view
	messagesView := New(80, 20)

	// Create spinner for loading state
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Get event bus components from the genieInstance (they're part of the core)
	eventBus := genieInstance.GetEventBus()
	subscriber := eventBus // EventBus embeds Subscriber
	publisher := eventBus  // EventBus embeds Publisher

	// Initialize project directory (where genie was started)
	projectDir := initialSession.WorkingDirectory

	// Initialize TUI-specific chat history in the project .genie directory
	historyPath := filepath.Join(projectDir, ".genie", "history")
	chatHistory := history.NewChatHistory(historyPath, true) // Enable saving to disk
	chatHistory.Load()                                       // Load existing history, ignore errors

	// Markdown renderer is now handled by MessagesView

	model := ReplModel{
		input:          ti,
		messagesView:   messagesView,
		spinner:        s,
		genieService:   genieInstance,
		currentSession: initialSession,
		subscriber:     subscriber,
		publisher:      publisher,
		projectDir:     projectDir,
		ready:          false,
		loading:        false,
		chatHistory:    chatHistory,
		initError:      nil,
		tuiConfig:      tuiConfig,
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
			contextModel, cmd := m.contextView.Update(msg)
			context := contextModel.(contextview.Model)
			m.contextView = &context
			return m, cmd
		}

		// Handle confirmation dialog first if active
		if m.confirmationDialog != nil {
			confirmationModel, cmd := m.confirmationDialog.Update(msg)
			confirm := confirmationModel.(confirmation.Model)
			m.confirmationDialog = &confirm
			return m, cmd
		}

		// Handle scrollable confirmation dialog if active
		if m.scrollableConfirmationDialog != nil {
			scrollableConfirmationModel, cmd := m.scrollableConfirmationDialog.Update(msg)
			scroll := scrollableConfirmationModel.(scrollconfirm.Model)
			m.scrollableConfirmationDialog = &scroll
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
				contextParts, err := m.genieService.GetContext(ctx)
				if err == nil {
					contextViewInstance := contextview.New(contextParts, m.width, m.height)
					m.contextView = &contextViewInstance
					m.showingContextView = true
				}
			}
			return m, nil
		case "ctrl+r", "ctrl+e", "f12":
			// Toggle tool result expansion and re-render (try multiple keys)
			m.messagesView = m.messagesView.ToggleToolsExpanded()

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
			m.messagesView, cmd = m.messagesView.Update(msg)
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
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Request was cancelled")
			} else {
				m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Failed to generate response: %v", msg.err))
			}
		} else {
			// Add assistant response
			m.messagesView = m.messagesView.AddMessage(AssistantMessage, msg.response)

			// Note: Session interaction tracking is handled internally by Genie
		}
		return m, nil

	case toolExecutedMsg:
		// Tool execution event handled by MessagesView
		m.messagesView = m.messagesView.AddToolMessage(msg)
		return m, nil

	case toolCallMessageMsg:
		// Tool call message - display as tool call message with white indicator
		m.messagesView = m.messagesView.AddMessage(ToolCallMessage, msg.message)
		return m, nil

	case confirmationRequestMsg:
		// Tool confirmation request - create confirmation dialog
		confirmationInstance := confirmation.New(msg.title, msg.message, msg.executionID, m.width)
		m.confirmationDialog = &confirmationInstance
		return m, nil

	case diffConfirmationRequestMsg:
		// Tool diff confirmation request - create scrollable confirmation dialog
		request := events.UserConfirmationRequest{
			ExecutionID: msg.executionID,
			Title:       msg.title,
			Content:     msg.diffContent,
			ContentType: "diff",
			FilePath:    msg.filePath,
		}
		diffConfirmationInstance := scrollconfirm.New(request, m.width, m.height)
		m.scrollableConfirmationDialog = &diffConfirmationInstance
		return m, nil

	case userConfirmationRequestMsg:
		// User confirmation request - create scrollable confirmation dialog
		confirmationInstance := scrollconfirm.New(msg.request, m.width, m.height)
		m.scrollableConfirmationDialog = &confirmationInstance
		return m, nil

	case confirmation.ResponseMsg:
		// Handle confirmation response
		if msg.Confirmed {
			// User said "Yes" - proceed with the tool execution
			if m.publisher != nil {
				response := events.ToolConfirmationResponse{
					ExecutionID: msg.ExecutionID,
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
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Request was cancelled")
			}

			// Still send the "No" response to the tool system to clean up
			if m.publisher != nil {
				response := events.ToolConfirmationResponse{
					ExecutionID: msg.ExecutionID,
					Confirmed:   false,
				}
				m.publisher.Publish(response.Topic(), response)
			}
		}

		// Clear confirmation dialog
		m.confirmationDialog = nil
		return m, nil

	case scrollconfirm.ResponseMsg:
		// Handle scrollable confirmation response
		if m.publisher != nil {
			response := events.UserConfirmationResponse{
				ExecutionID: msg.ExecutionID,
				Confirmed:   msg.Confirmed,
			}
			m.publisher.Publish(response.Topic(), response)
		}
		// Close scrollable confirmation dialog
		m.scrollableConfirmationDialog = nil
		return m, nil

	case contextview.CloseMsg:
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

		m.messagesView = m.messagesView.SetSize(msg.Width-4, msg.Height-4) // space for input
		m.input.Width = msg.Width - 7                                      // border(2) + padding(2) + margin(3)

		// Update context view if active
		if m.showingContextView && m.contextView != nil {
			contextModel, cmd := m.contextView.Update(msg)
			context := contextModel.(contextview.Model)
			m.contextView = &context
			return m, cmd
		}

		// Markdown renderer width is now handled by MessagesView.Resize()

		if !m.ready {
			m.ready = true
			// Show initialization error after the window is ready
			if m.initError != nil {
				m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Initialization warning: %v", m.initError))
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Some features may be unavailable. Type /help for available commands.")
			}
		}

		return m, nil
	}

	// Update input and messages view
	var inputCmd tea.Cmd
	var messagesCmd tea.Cmd

	m.input, inputCmd = m.input.Update(msg)
	m.messagesView, messagesCmd = m.messagesView.Update(msg)

	return m, tea.Batch(inputCmd, messagesCmd)
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

	return lipgloss.JoinVertical(lipgloss.Left, m.messagesView.View(), inputSection)
}

// inputView renders the input area using the current theme
func (m ReplModel) inputView() string {
	// Use focused input style if the input is focused
	styles := theme.GetStyles()
	inputStyle := styles.Input
	if m.input.Focused() {
		inputStyle = styles.InputFocus
	}
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
	m.messagesView = m.messagesView.AddMessage(UserMessage, value)

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
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "/clear - Clear chat")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "/config - Manage TUI settings")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "/theme - List available themes")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "/context view - Open context viewer modal")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "/debug - Toggle debug mode")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "/exit - Exit")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("Project: %s", m.projectDir))
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "Navigation:")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "↑/↓ - Navigate command history (stored in .genie/history)")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "PgUp/PgDn - Scroll chat")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "Shortcuts:")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "Ctrl+/ (or Ctrl+_) - Open context viewer")

	case "/clear":
		m.messagesView = m.messagesView.Clear()

	case "/debug":
		m.debug = !m.debug
		if m.debug {
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "Debug mode enabled")
		} else {
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "Debug mode disabled")
		}

	case "/config":
		return m.handleConfigCommand(parts)

	case "/theme":
		return m.handleThemeCommand(parts)

	case "/context":
		return m.handleContextCommand(parts)

	case "/exit", "/quit":
		return m, tea.Quit

	default:
		m.messagesView = m.messagesView.AddMessage(ErrorMessage, "Unknown command. Type /help")
	}

	return m, nil
}

// handleConfigCommand processes /config commands
func (m ReplModel) handleConfigCommand(parts []string) (ReplModel, tea.Cmd) {
	if len(parts) == 1 {
		// Show current config
		if m.tuiConfig != nil {
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "Current TUI Configuration:")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  cursor_blink: %t", m.tuiConfig.CursorBlink))
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  chat_timeout_seconds: %d", m.tuiConfig.ChatTimeoutSeconds))
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  theme: %s", m.tuiConfig.Theme))
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "Usage:")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "  /config show              - Show current settings")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "  /config set <key> <value> - Change a setting")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "Available settings:")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "  cursor_blink (true/false) - Enable/disable cursor blinking")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "  chat_timeout_seconds (number) - Chat request timeout in seconds")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "  theme (string) - Theme name (default, dark, light, minimal, neon, or custom)")
		} else {
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, "TUI configuration not available")
		}
		return m, nil
	}

	subCommand := parts[1]
	switch subCommand {
	case "show":
		if m.tuiConfig != nil {
			m.messagesView = m.messagesView.AddMessage(SystemMessage, "Current TUI Configuration:")
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  cursor_blink: %t", m.tuiConfig.CursorBlink))
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  chat_timeout_seconds: %d", m.tuiConfig.ChatTimeoutSeconds))
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  theme: %s", m.tuiConfig.Theme))
		} else {
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, "TUI configuration not available")
		}

	case "set":
		if len(parts) < 4 {
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, "Usage: /config set <key> <value>")
			return m, nil
		}

		key := parts[2]
		value := parts[3]

		if m.tuiConfig == nil {
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, "TUI configuration not available")
			return m, nil
		}

		switch key {
		case "cursor_blink":
			if value == "true" {
				m.tuiConfig.CursorBlink = true
				m.input.Cursor.SetMode(cursor.CursorBlink)
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Cursor blinking enabled.")
			} else if value == "false" {
				m.tuiConfig.CursorBlink = false
				m.input.Cursor.SetMode(cursor.CursorStatic)
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Cursor blinking disabled.")
			} else {
				m.messagesView = m.messagesView.AddMessage(ErrorMessage, "cursor_blink must be 'true' or 'false'")
				return m, nil
			}

			// Save config
			if err := m.tuiConfig.Save(); err != nil {
				m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Failed to save config: %v", err))
			} else {
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Configuration saved successfully")
			}

		case "chat_timeout_seconds":
			timeout, err := strconv.Atoi(value)
			if err != nil || timeout <= 0 {
				m.messagesView = m.messagesView.AddMessage(ErrorMessage, "chat_timeout_seconds must be a positive number")
				return m, nil
			}
			m.tuiConfig.ChatTimeoutSeconds = timeout
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("Chat timeout set to %d seconds", timeout))

			// Save config
			if err := m.tuiConfig.Save(); err != nil {
				m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Failed to save config: %v", err))
			} else {
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Configuration saved successfully")
			}

		case "theme":
			// First, check if the theme exists by trying to load it
			if err := theme.LoadGlobalTheme(value); err != nil {
				m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Failed to load theme '%s': %v", value, err))
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Use /theme to see available themes")
				return m, nil
			}

			// If successful, save it to config
			m.tuiConfig.Theme = value
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("Theme changed to '%s'", value))

			// Save config
			if err := m.tuiConfig.Save(); err != nil {
				m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Failed to save config: %v", err))
			} else {
				m.messagesView = m.messagesView.AddMessage(SystemMessage, "Configuration saved successfully")
			}

		default:
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Unknown configuration key: %s", key))
		}

	default:
		m.messagesView = m.messagesView.AddMessage(ErrorMessage, "Unknown config command. Use: show, set")
	}

	return m, nil
}

// handleThemeCommand processes /theme commands
func (m ReplModel) handleThemeCommand(parts []string) (ReplModel, tea.Cmd) {
	// List available themes
	themes, err := theme.GetGlobalProvider().ListThemes()
	if err != nil {
		m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Failed to list themes: %v", err))
		return m, nil
	}

	m.messagesView = m.messagesView.AddMessage(SystemMessage, "Available themes:")
	for _, themeName := range themes {
		if m.tuiConfig != nil && themeName == m.tuiConfig.Theme {
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  %s (current)", themeName))
		} else {
			m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  %s", themeName))
		}
	}

	// Add builtin themes if not in the list
	builtinThemes := []string{"default", "dark", "light", "minimal", "neon"}
	for _, builtin := range builtinThemes {
		found := false
		for _, t := range themes {
			if t == builtin {
				found = true
				break
			}
		}
		if !found {
			if m.tuiConfig != nil && builtin == m.tuiConfig.Theme {
				m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  %s (current, built-in)", builtin))
			} else {
				m.messagesView = m.messagesView.AddMessage(SystemMessage, fmt.Sprintf("  %s (built-in)", builtin))
			}
		}
	}

	m.messagesView = m.messagesView.AddMessage(SystemMessage, "")
	m.messagesView = m.messagesView.AddMessage(SystemMessage, "To change theme: /config set theme <name>")

	return m, nil
}

// handleContextCommand processes /context commands
func (m ReplModel) handleContextCommand(parts []string) (ReplModel, tea.Cmd) {
	if len(parts) == 1 {
		// Show help for context command
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "Context Management:")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "  /context view  - Open context viewer modal (ESC to close)")
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "  /context clean - Clear context")
		return m, nil
	}

	subCommand := parts[1]
	switch subCommand {
	case "view":
		// Get context from Genie service
		if m.genieService == nil {
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, "Genie service not available")
			return m, nil
		}

		ctx := context.Background()
		contextParts, err := m.genieService.GetContext(ctx)
		if err != nil {
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("Failed to get context: %v", err))
			return m, nil
		}

		// Open context view modal
		contextViewInstance := contextview.New(contextParts, m.width, m.height)
		m.contextView = &contextViewInstance
		m.showingContextView = true

	case "clean":
		// TODO: Add ClearContext method to Genie interface and implement
		m.messagesView = m.messagesView.AddMessage(SystemMessage, "Context clearing not yet implemented")

	default:
		m.messagesView = m.messagesView.AddMessage(ErrorMessage, "Unknown context command. Use: view, clean")
	}

	return m, nil
}

// handleAskCommand processes regular input as an ask command
func (m ReplModel) handleAskCommand(input string) (ReplModel, tea.Cmd) {
	// Check if Genie service is available
	if m.genieService == nil {
		if m.initError != nil {
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, fmt.Sprintf("AI features unavailable: %v", m.initError))
		} else {
			m.messagesView = m.messagesView.AddMessage(ErrorMessage, "AI features unavailable. Please check your configuration.")
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

		// Use Genie service to process the chat message
		// This handles LLM calls, tool formatting, and all the service layer logic
		// The response will be published via the event bus and handled by the global subscription
		err := m.genieService.Chat(ctx, userInput)
		if err != nil {
			return aiResponseMsg{err: err, userInput: userInput}
		}

		// Return a success message - the actual response will come via event subscription
		return aiResponseMsg{response: "", err: nil, userInput: userInput}
	}
}

// GetProjectDir returns the current project directory
func (m ReplModel) GetProjectDir() string {
	return m.projectDir
}

// Old message handling functions removed - now handled by MessagesView

// StartREPL initializes and runs the REPL
func StartREPL(genieInstance genie.Genie, initialSession *genie.Session) {
	// Set up logging for REPL mode (quiet by default)
	logger := logging.NewQuietLogger()
	logging.SetGlobalLogger(logger)

	// Initialize theme system
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".genie")
	if err := theme.InitGlobalProvider(configDir); err != nil {
		// Log error but continue with default theme
		logger.Error("Failed to initialize theme system", "error", err)
	}

	// Load theme from TUI config
	tuiConfig, _ := LoadConfig()
	if tuiConfig != nil && tuiConfig.Theme != "" {
		if err := theme.LoadGlobalTheme(tuiConfig.Theme); err != nil {
			// Log error but continue with current theme
			logger.Warn("Failed to load theme from config", "theme", tuiConfig.Theme, "error", err)
		}
	}

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

	// Subscribe to tool call messages
	model.subscriber.Subscribe("tool.call.message", func(event interface{}) {
		if messageEvent, ok := event.(events.ToolCallMessageEvent); ok {

			// Send a Bubble Tea message to display the tool call message
			p.Send(toolCallMessageMsg{
				toolName: messageEvent.ToolName,
				message:  messageEvent.Message,
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

	// Subscribe to chat responses
	model.subscriber.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(events.ChatResponseEvent); ok {
			// Send a Bubble Tea message to update the UI with the AI response
			p.Send(aiResponseMsg{
				response:  resp.Response,
				err:       resp.Error,
				userInput: "", // We don't have the original input here, but it's not needed for display
			})
		}
	})

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running REPL: %v\n", err)
		os.Exit(1)
	}
}
