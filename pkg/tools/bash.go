package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// BashTool executes bash commands with optional interactive confirmation
type BashTool struct {
	publisher              events.Publisher
	subscriber             events.Subscriber
	confirmationChannels   map[string]chan bool
	confirmationMutex      sync.RWMutex
	requiresConfirmation   bool
}

// NewBashTool creates a new bash tool with interactive confirmation support
func NewBashTool(publisher events.Publisher, subscriber events.Subscriber, requiresConfirmation bool) Tool {
	tool := &BashTool{
		publisher:            publisher,
		subscriber:           subscriber,
		confirmationChannels: make(map[string]chan bool),
		requiresConfirmation: requiresConfirmation,
	}

	// Subscribe to confirmation responses
	if subscriber != nil {
		subscriber.Subscribe("tool.confirmation.response", tool.handleConfirmationResponse)
	}

	return tool
}

// Declaration returns the function declaration for the bash tool
func (b *BashTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "runBashCommand",
		Description: "Execute shell commands for tasks not covered by other specific tools. Dangerous commands will require user confirmation.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for executing a bash command",
			Properties: map[string]*ai.Schema{
				"command": {
					Type:        ai.TypeString,
					Description: "The shell command to execute. Examples: 'ls -la' to list files, 'git status' to check git status, 'find . -name \"*.go\"' to find Go files, 'ps aux' to check processes",
					MinLength:   1,
					MaxLength:   1000,
				},
				"cwd": {
					Type:        ai.TypeString,
					Description: "Optional working directory to run the command in. Use absolute or relative paths. Example: '/path/to/project' or '.'",
					MaxLength:   500,
				},
				"timeout_ms": {
					Type:        ai.TypeInteger,
					Description: "Optional timeout in milliseconds. Default is 30000ms (30 seconds). Use higher values for long-running commands",
					Minimum:     100,
					Maximum:     300000, // 5 minutes max
				},
			},
			Required: []string{"command"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the bash command execution",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the command executed successfully",
				},
				"output": {
					Type:        ai.TypeString,
					Description: "The command output (stdout and stderr combined)",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if the command failed",
				},
			},
			Required: []string{"success", "output"},
		},
	}
}

// Handler returns the function handler for the bash tool
func (b *BashTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Generate execution ID for this tool execution
		executionID := uuid.New().String()
		
		// Add execution ID to context
		ctx = context.WithValue(ctx, "executionID", executionID)

		// Extract command parameter
		command, ok := params["command"].(string)
		if !ok {
			return nil, fmt.Errorf("command parameter is required and must be a string")
		}

		// Check if command requires confirmation
		if b.requiresConfirmation {
			confirmed, err := b.requestConfirmation(ctx, executionID, command)
			if err != nil {
				return map[string]any{
					"success": false,
					"output":  "",
					"error":   fmt.Sprintf("confirmation failed: %v", err),
				}, nil
			}
			
			if !confirmed {
				return map[string]any{
					"success": false,
					"output":  "",
					"error":   "command cancelled by user",
				}, nil
			}
		}

		// Execute the command
		return b.executeCommand(ctx, command, params)
	}
}

// requestConfirmation requests user confirmation and waits for response
func (b *BashTool) requestConfirmation(ctx context.Context, executionID, command string) (bool, error) {
	// Create confirmation channel for this execution
	confirmationChan := make(chan bool, 1)
	
	b.confirmationMutex.Lock()
	b.confirmationChannels[executionID] = confirmationChan
	b.confirmationMutex.Unlock()
	
	// Clean up channel when done
	defer func() {
		b.confirmationMutex.Lock()
		delete(b.confirmationChannels, executionID)
		b.confirmationMutex.Unlock()
	}()

	// Get session ID from context
	sessionID := "unknown"
	if ctx != nil {
		if id, ok := ctx.Value("sessionID").(string); ok && id != "" {
			sessionID = id
		}
	}

	// Create and publish confirmation request
	request := events.ToolConfirmationRequest{
		ExecutionID: executionID,
		SessionID:   sessionID,
		ToolName:    "Run Bash Command",
		Command:     command,
		Message:     fmt.Sprintf("Execute '%s'? [y/N]", command),
	}

	if b.publisher != nil {
		b.publisher.Publish(request.Topic(), request)
	}

	// Wait for confirmation response indefinitely
	select {
	case confirmed := <-confirmationChan:
		return confirmed, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// handleConfirmationResponse handles incoming confirmation responses
func (b *BashTool) handleConfirmationResponse(event interface{}) {
	if response, ok := event.(events.ToolConfirmationResponse); ok {
		b.confirmationMutex.RLock()
		if ch, exists := b.confirmationChannels[response.ExecutionID]; exists {
			// Send response to waiting channel (non-blocking)
			select {
			case ch <- response.Confirmed:
			default:
				// Channel is full or closed, ignore
			}
		}
		b.confirmationMutex.RUnlock()
	}
}

// executeCommand executes the bash command
func (b *BashTool) executeCommand(ctx context.Context, command string, params map[string]any) (map[string]any, error) {
	// Extract optional working directory
	var cwd string
	if cwdParam, exists := params["cwd"]; exists {
		if cwdStr, ok := cwdParam.(string); ok {
			cwd = cwdStr
		}
	}

	// Extract optional timeout
	var timeout time.Duration = 30 * time.Second // Default 30s timeout
	if timeoutParam, exists := params["timeout_ms"]; exists {
		if timeoutMs, ok := timeoutParam.(float64); ok {
			timeout = time.Duration(timeoutMs) * time.Millisecond
		}
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create the command
	cmd := exec.CommandContext(execCtx, "bash", "-c", command)
	
	// Set working directory if provided
	if cwd != "" {
		cmd.Dir = cwd
	}

	// Set environment
	cmd.Env = os.Environ()

	// Execute command and capture output
	output, err := cmd.CombinedOutput()
	
	// Check for timeout
	if execCtx.Err() == context.DeadlineExceeded {
		return map[string]any{
			"success": false,
			"output":  string(output),
			"error":   fmt.Sprintf("command timed out after %v", timeout),
		}, nil
	}

	// Check for other errors
	if err != nil {
		return map[string]any{
			"success": false,
			"output":  string(output),
			"error":   fmt.Sprintf("command failed: %v", err),
		}, nil
	}

	return map[string]any{
		"success": true,
		"output":  string(output),
	}, nil
}