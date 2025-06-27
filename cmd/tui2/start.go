package tui2

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

// StartTUI starts the gocui-based TUI with file-based logging
func StartTUI(genieInstance genie.Genie, initialSession *genie.Session) error {
	// Configure file-based logging for the entire Genie instance
	if err := setupGenieFileLogging(initialSession.WorkingDirectory); err != nil {
		// Fallback to quiet logging if file logging fails
		logging.SetGlobalLogger(logging.NewQuietLogger())
		fmt.Fprintf(os.Stderr, "Warning: Failed to setup file logging: %v\n", err)
	}
	
	tui, err := NewTUI(genieInstance, initialSession)
	if err != nil {
		return err
	}
	defer tui.Close()
	
	return tui.Run()
}

// setupGenieFileLogging configures Genie's global logger to write to project-specific log file
func setupGenieFileLogging(projectDir string) error {
	// Create .genie directory in the project
	genieDir := filepath.Join(projectDir, ".genie")
	if err := os.MkdirAll(genieDir, 0755); err != nil {
		return fmt.Errorf("failed to create .genie directory: %w", err)
	}
	
	// Create log file in project's .genie directory
	logFile := filepath.Join(genieDir, "tui.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	
	// Configure Genie's logger to write to file with timestamps
	logger := logging.NewLogger(logging.Config{
		Level:   slog.LevelInfo, // Good balance of detail without noise
		Format:  logging.FormatText,
		Output:  file,           // All Genie logs go to file now
		AddTime: true,           // Timestamps essential for file logs
	})
	
	// Set this as Genie's global logger - all components will use it
	logging.SetGlobalLogger(logger)
	
	// Log startup message
	logger.Info("Genie TUI started with file logging", "log_file", logFile)
	
	return nil
}