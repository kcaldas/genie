package tui2

import (
	"context"
	"path/filepath"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui/history"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
)

// View constants
const (
	viewMessages     = "messages"
	viewInput        = "input"
	viewStatus       = "status"
	viewDialog       = "dialog"
	viewDebug        = "debug"
	viewHelp         = "help"
	viewNotification = "notification"
)

// TUI represents the main TUI application
type TUI struct {
	g              *gocui.Gui
	genieService   genie.Genie
	currentSession *genie.Session
	subscriber     events.Subscriber
	publisher      events.Publisher
	
	// State
	loading              bool
	requestTime          time.Time
	cancelCurrentRequest context.CancelFunc
	messages             []Message
	
	// History management
	chatHistory history.ChatHistory
	
	// Markdown rendering
	markdownRenderer MarkdownRenderer
	
	// Focus management
	focusManager *FocusManager
	
	// Theme management
	themeManager *ThemeManager
	
	// Mini UI system
	miniLayoutManager *MiniLayoutManager
	
	// Debug panel state
	showDebug    bool
	debugMessages []string
	
	// Dialog state
	showDialog     bool
	dialogTitle    string
	dialogMessage  string
	dialogCallback func(confirmed bool)
	
	// Help panel state
	showHelp bool
	
	// Clipboard state
	clipboard string
	
	// Notification state
	notificationText string
	notificationTime time.Time
}

// Message represents a chat message
type Message struct {
	Type    MessageType
	Content string
	Time    time.Time
	Success *bool // For tool messages: nil=progress, true=success, false=failure
}

// MessageType defines the type of message
type MessageType int

const (
	UserMessage MessageType = iota
	AssistantMessage
	SystemMessage
	ErrorMessage
	ToolMessage
)

// NewTUI creates a new gocui-based TUI
func NewTUI(genieInstance genie.Genie, initialSession *genie.Session) (*TUI, error) {
	g, err := gocui.NewGui(gocui.Output256, true) // 256-color mode for better ANSI support
	if err != nil {
		return nil, err
	}
	
	// Enable cursor for input editing
	g.Cursor = true

	// Get event bus components
	eventBus := genieInstance.GetEventBus()
	
	// Initialize history
	projectDir := initialSession.WorkingDirectory
	historyPath := filepath.Join(projectDir, ".genie", "history")
	chatHistory := history.NewChatHistory(historyPath, true)
	if err := chatHistory.Load(); err != nil {
		// Log error but don't fail - history is not critical
		// TODO: Use proper logging once we integrate it
	}
	
	// Initialize markdown renderer with fallback (start with auto style)
	markdownRenderer := NewGlamourRendererWithTheme(80, "auto") // Default width with auto theme
	
	tui := &TUI{
		g:                g,
		genieService:     genieInstance,
		currentSession:   initialSession,
		subscriber:       eventBus,
		publisher:        eventBus,
		messages:         []Message{},
		chatHistory:      chatHistory,
		markdownRenderer: markdownRenderer,
		focusManager:     NewFocusManager(),
		themeManager:     NewThemeManager(),
		debugMessages:    []string{},
	}
	
	// Create mini layout manager
	tui.miniLayoutManager = NewMiniLayoutManager(tui)
	
	// Set up the layout manager
	g.SetManagerFunc(tui.miniLayoutManager.Layout)
	
	// Set up key bindings
	if err := tui.setupKeyBindings(); err != nil {
		g.Close()
		return nil, err
	}
	
	// Set up event subscriptions
	tui.setupEventSubscriptions()
	
	// Add welcome message
	tui.addMessage(SystemMessage, "Welcome to Genie! Type /help for commands.")
	
	return tui, nil
}

// Run starts the TUI main loop
func (t *TUI) Run() error {
	if err := t.g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}
	return nil
}

// Close cleanup resources
func (t *TUI) Close() {
	t.g.Close()
}