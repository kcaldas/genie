package controllers

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

type DebugController struct {
	*BaseController
	debugState      *state.DebugState
	debugComponent  *component.DebugComponent
	layoutManager   *layout.LayoutManager
	clipboard       *helpers.Clipboard
	commandEventBus *events.CommandEventBus
	
	// File-based debugging
	debugFile       *os.File
	debugLogger     logging.Logger
	tailingEnabled  bool
}

func NewDebugController(
	genieService genie.Genie,
	gui types.Gui,
	debugState *state.DebugState,
	debugComponent *component.DebugComponent,
	layoutManager *layout.LayoutManager,
	clipboard *helpers.Clipboard,
	config *helpers.ConfigManager,
	commandEventBus *events.CommandEventBus,
) *DebugController {
	c := &DebugController{
		BaseController:  NewBaseController(debugComponent, gui, config),
		debugState:      debugState,
		debugComponent:  debugComponent,
		layoutManager:   layoutManager,
		clipboard:       clipboard,
		commandEventBus: commandEventBus,
	}

	// Debug mode is now managed by the global logger via environment variables
	// Debug logging is handled by the centralized logger in individual controllers

	// Subscribe to debug view events
	commandEventBus.Subscribe("debug.view", func(data interface{}) {
		c.toggleDebugPanel()
	})

	// Subscribe to debug clear events
	commandEventBus.Subscribe("debug.clear", func(data interface{}) {
		c.ClearDebugMessages()
	})

	// Subscribe to debug copy events
	commandEventBus.Subscribe("debug.copy", func(data interface{}) {
		c.CopyDebugMessages()
	})

	return c
}

// AddDebugMessage adds a debug message and notifies component
func (c *DebugController) AddDebugMessage(message string) {
	c.debugState.AddDebugMessage(message)
	c.gui.PostUIUpdate(func() {
		c.debugComponent.OnMessageAdded()
	})
}

// ClearDebugMessages clears all debug messages and triggers render
func (c *DebugController) ClearDebugMessages() {
	c.debugState.ClearDebugMessages()
	c.renderDebugComponent()
}

// GetDebugMessages returns all debug messages
func (c *DebugController) GetDebugMessages() []string {
	return c.debugState.GetDebugMessages()
}

// CopyDebugMessages copy all debug messages to clipboard
func (c *DebugController) CopyDebugMessages() {
	messages := c.debugState.GetDebugMessages()
	messagesStr := strings.Join(messages, "\n")
	c.clipboard.Copy(messagesStr)
}

// Debug method removed - debug logging is now handled by the centralized logging system

// renderDebugComponent triggers a render of the debug component
func (c *DebugController) renderDebugComponent() {
	c.gui.PostUIUpdate(func() {
		if err := c.debugComponent.Render(); err != nil {
			// Log error but don't propagate - debug rendering shouldn't break the app
			c.debugState.AddDebugMessage(fmt.Sprintf("Error rendering debug: %v", err))
		}
	})
}

// UpdateLogLevel updates the global logger's level based on the provided level string
func (c *DebugController) UpdateLogLevel(levelStr string) {
	// Convert string to slog.Level
	var logLevel slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		// Empty or invalid level means disable debug (error level only)
		logLevel = slog.LevelError
	}

	// Create a new file logger with the updated level to ensure proper file logging
	newLogger := logging.NewFileLoggerFromEnv("genie-debug.log")
	
	// Update the logger's level if it supports it
	if levelUpdater, ok := newLogger.(interface{ SetLevel(slog.Level) }); ok {
		levelUpdater.SetLevel(logLevel)
	}
	
	// Set as the new global logger
	logging.SetGlobalLogger(newLogger)
}

func (c *DebugController) toggleDebugPanel() {
	// Use the new Panel system for cleaner toggle logic
	if debugPanel := c.layoutManager.GetPanel("debug"); debugPanel != nil {
		// Toggle visibility - Panel handles view lifecycle automatically
		isVisible := debugPanel.IsVisible()
		debugPanel.SetVisible(!isVisible)

		// If becoming visible, show debug file content instead of in-memory messages
		if !isVisible {
			c.loadDebugFileContent()
			debugPanel.Render()
		}
	}
}

// loadDebugFileContent reads the debug file and displays it in the debug component
func (c *DebugController) loadDebugFileContent() {
	// Get debug file path using the centralized helper
	debugFile := logging.GetDebugFilePath("genie-debug.log")

	// Read the file content
	if content, err := os.ReadFile(debugFile); err == nil {
		// Clear current debug messages and replace with file content
		c.debugState.ClearDebugMessages()
		
		// Split file content into lines and add as debug messages
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				c.debugState.AddDebugMessage(line)
			}
		}
	} else {
		// If file doesn't exist or can't be read, show a message
		c.debugState.ClearDebugMessages()
		c.debugState.AddDebugMessage(fmt.Sprintf("Debug file not found or unreadable: %s", debugFile))
		c.debugState.AddDebugMessage("Use :debug command to enable debug logging")
	}
}
