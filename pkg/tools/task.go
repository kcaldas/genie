package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// TaskTool spawns a subprocess genie instance for complex research tasks
type TaskTool struct {
	publisher events.Publisher
}

// NewTaskTool creates a new task tool
func NewTaskTool(publisher events.Publisher) Tool {
	return &TaskTool{
		publisher: publisher,
	}
}

// Declaration returns the function declaration for the task tool
func (t *TaskTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "Task",
		Description: "Spawn an isolated Genie session for complex research, analysis, or multi-step tasks. Use when you need to perform extensive code exploration, research patterns, or analyze large codebases without polluting the current conversation context.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for spawning a task session",
			Properties: map[string]*ai.Schema{
				"summary": {
					Type:        ai.TypeString,
					Description: "Brief summary of what this task will accomplish and why it's needed. This will be shown to the user before starting the task.",
					MinLength:   10,
					MaxLength:   200,
				},
				"prompt": {
					Type:        ai.TypeString,
					Description: "Detailed task description and context for the subprocess. Be specific about what you want to research, analyze, or accomplish. Include relevant file paths, patterns to look for, or specific questions to answer. Make is explicit the format of the response you want and that this not involves any code changes.",
					MinLength:   10,
					MaxLength:   4000,
				},
				"timeout_ms": {
					Type:        ai.TypeInteger,
					Description: "Optional timeout in milliseconds. Default is 120000ms (2 minutes). Use higher values for complex analysis tasks",
					Minimum:     5000,   // 5 seconds minimum
					Maximum:     600000, // 10 minutes max
				},
			},
			Required: []string{"summary", "prompt"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the task execution",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the task completed successfully",
				},
				"results": {
					Type:        ai.TypeString,
					Description: "The task output and findings",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if the task failed",
				},
			},
			Required: []string{"success", "results"},
		},
	}
}

// Handler returns the function handler for the task tool
func (t *TaskTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Extract required parameters
		summary, ok := params["summary"].(string)
		if !ok || strings.TrimSpace(summary) == "" {
			return nil, fmt.Errorf("summary parameter is required and must be a non-empty string")
		}

		prompt, ok := params["prompt"].(string)
		if !ok || strings.TrimSpace(prompt) == "" {
			return nil, fmt.Errorf("prompt parameter is required and must be a non-empty string")
		}

		// Send notification about the task
		if t.publisher != nil {
			notification := events.NotificationEvent{
				Message:     fmt.Sprintf("**Task Starting:** %s", summary),
				Role:        "assistant",
				ContentType: "markdown",
			}
			t.publisher.Publish(notification.Topic(), notification)
		}

		// Execute the task
		return t.executeTask(ctx, prompt, params)
	}
}

// executeTask executes the genie subprocess
func (t *TaskTool) executeTask(ctx context.Context, prompt string, params map[string]any) (map[string]any, error) {
	// Extract optional working directory
	var cwd string
	if cwdParam, exists := params["workspace"]; exists {
		if cwdStr, ok := cwdParam.(string); ok {
			cwd = strings.TrimSpace(cwdStr)
		}
	}

	// If no explicit workspace provided, use session working directory from context
	if cwd == "" {
		if sessionCwd := ctx.Value("cwd"); sessionCwd != nil {
			if sessionCwdStr, ok := sessionCwd.(string); ok && sessionCwdStr != "" {
				cwd = sessionCwdStr
			}
		}
	}

	// Extract optional persona, default to current persona from context
	persona := "genie" // fallback default
	if contextPersona := ctx.Value("persona"); contextPersona != nil {
		if personaStr, ok := contextPersona.(string); ok && strings.TrimSpace(personaStr) != "" {
			persona = strings.TrimSpace(personaStr)
		}
	}

	// Allow override via parameter
	if personaParam, exists := params["persona"]; exists {
		if personaStr, ok := personaParam.(string); ok && strings.TrimSpace(personaStr) != "" {
			persona = strings.TrimSpace(personaStr)
		}
	}

	// Extract optional timeout
	var timeout time.Duration = 2 * time.Minute // Default 2 minutes
	if timeoutParam, exists := params["timeout_ms"]; exists {
		if timeoutMs, ok := timeoutParam.(float64); ok {
			timeout = time.Duration(timeoutMs) * time.Millisecond
		}
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build enhanced prompt for deep research
	enhancedPrompt := fmt.Sprintf(`DEEP RESEARCH TASK:

You are conducting thorough research. Follow this approach:

1. **ANALYZE THOROUGHLY**: Examine the codebase systematically, don't stop at surface-level findings
2. **VALIDATE FINDINGS**: Cross-reference patterns across multiple files and locations  
3. **GO DEEPER**: When you find something interesting, investigate related code, dependencies, and usage patterns
4. **MULTIPLE PERSPECTIVES**: Look at the problem from different angles - architecture, implementation, testing, documentation
5. **TRACE CONNECTIONS**: Follow imports, function calls, and data flow to understand the complete picture
6. **VERIFY ASSUMPTIONS**: Don't accept first findings - look for counter-examples and edge cases

RESEARCH TASK:
%s

Provide comprehensive findings with specific file references and line numbers where relevant.`, prompt)

	// Build the genie command - put global flags before subcommand
	args := []string{}
	if persona != "" {
		args = append(args, "--persona", persona)
	}
	args = append(args, "ask", "--accept-all", enhancedPrompt)
	// Use local build if available, fallback to installed genie
	genieCmd := "genie"
	if _, err := os.Stat("./build/genie"); err == nil {
		genieCmd = "./build/genie"
	}
	cmd := exec.CommandContext(execCtx, genieCmd, args...)

	// Set working directory if provided
	if cwd != "" {
		cmd.Dir = cwd
	}

	// Inherit environment variables from current process
	cmd.Env = os.Environ()

	// Execute command and capture output
	output, err := cmd.CombinedOutput()

	// Check for timeout
	if execCtx.Err() == context.DeadlineExceeded {
		return map[string]any{
			"success": false,
			"results": string(output),
			"error":   fmt.Sprintf("task timed out after %v", timeout),
		}, nil
	}

	// Check for other errors
	if err != nil {
		return map[string]any{
			"success": false,
			"results": string(output),
			"error":   fmt.Sprintf("task failed: %v", err),
		}, nil
	}

	// Clean up the output (remove any command echoing)
	results := strings.TrimSpace(string(output))

	return map[string]any{
		"success": true,
		"results": results,
	}, nil
}

// FormatOutput formats task results for user display
func (t *TaskTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	output, _ := result["results"].(string)
	errorMsg, _ := result["error"].(string)

	if !success {
		if errorMsg != "" {
			return fmt.Sprintf("**Task Failed**\n```\n%s\n```", errorMsg)
		}
		return "**Task Failed**"
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return "**Task completed successfully (no output)**"
	}

	// Format output nicely
	return fmt.Sprintf("**Task Results**\n\n%s", output)
}
