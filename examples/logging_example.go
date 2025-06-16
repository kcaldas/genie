// Example demonstrating the logging system
// Run with: go run examples/logging_example.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kcaldas/genie/pkg/logging"
)

func main() {
	fmt.Println("=== Logging System Examples ===\n")

	// Example 1: Basic logging levels
	fmt.Println("1. Basic Logging Levels:")
	logging.SetGlobalLogger(logging.NewVerboseLogger())
	
	logging.Debug("This is a debug message", "key", "value")
	logging.Info("Application started successfully")
	logging.Warn("This is a warning message", "component", "example")
	logging.Error("This is an error message", "error", "something went wrong")
	
	fmt.Println()

	// Example 2: Different logger configurations
	fmt.Println("2. Quiet Logger (errors only):")
	logging.SetGlobalLogger(logging.NewQuietLogger())
	logging.Debug("You won't see this debug message")
	logging.Info("You won't see this info message")
	logging.Error("You WILL see this error message")
	
	fmt.Println()

	// Example 3: Component loggers
	fmt.Println("3. Component Loggers:")
	logging.SetGlobalLogger(logging.NewDefaultLogger())
	
	chainLogger := logging.NewChainLogger("user-onboarding")
	chainLogger.Info("chain execution started", "steps", 5)
	
	apiLogger := logging.NewAPILogger("vertex-ai")
	apiLogger.Debug("making API request", "endpoint", "/v1/generate", "model", "gemini-pro")
	
	componentLogger := logging.NewComponentLogger("file-manager")
	componentLogger.Info("file operation completed", "operation", "write", "file", "output.txt")
	
	fmt.Println()

	// Example 4: Structured logging with context
	fmt.Println("4. Structured Logging:")
	logger := logging.NewOperationLogger("database", "migration")
	logger.Info("migration started", 
		"version", "2.1.0",
		"tables_affected", 3,
		"estimated_duration", "5m")
	
	// With additional context
	contextLogger := logger.With("migration_id", "20240101_001", "user", "admin")
	contextLogger.Info("migration step completed", "step", "create_indexes")
	
	fmt.Println()

	// Example 5: Error logging helpers
	fmt.Println("5. Error Logging Helpers:")
	ctx := context.Background()
	testError := errors.New("connection timeout")
	
	// Basic error logging
	logging.LogError(ctx, logger, "database connection failed", testError, "host", "localhost", "port", 5432)
	
	// Error with operation context
	logging.LogErrorWithOperation(ctx, logger, "user_authentication", "login failed", testError, "username", "john_doe")
	
	fmt.Println()

	// Example 6: Using Fatal (commented out to not exit)
	fmt.Println("6. Fatal Logging (commented out):")
	fmt.Println("// logging.Fatal(\"critical system failure\", \"component\", \"core\")")
	
	fmt.Println()

	// Example 7: JSON format logging
	fmt.Println("7. JSON Format Logging:")
	jsonLogger := logging.NewLogger(logging.Config{
		Level:   slog.LevelInfo, // Fixed: should be slog level, not format
		Format:  logging.FormatJSON,
		AddTime: true,
	})
	
	jsonLogger.Info("structured log entry", 
		"event", "user_action",
		"user_id", 12345,
		"action", "file_upload",
		"file_size", 1024576,
		"success", true)
	
	fmt.Println("\n=== End Examples ===")
}

// Example function showing how to integrate logging in your own functions
func processUserData(userID int, action string) error {
	logger := logging.NewComponentLogger("user-processor").With("user_id", userID, "action", action)
	
	logger.Info("processing user data")
	
	// Simulate some work
	if userID < 0 {
		err := errors.New("invalid user ID")
		logging.LogErrorWithOperation(context.Background(), logger, "validation", "user data validation failed", err)
		return err
	}
	
	logger.Info("user data processed successfully", "duration_ms", 42)
	return nil
}