package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/kcaldas/genie/pkg/tools/process"
)

// BashTool executes bash commands with optional interactive confirmation
type BashTool struct {
	eventBus             events.EventBus
	confirmer            Confirmer
	requiresConfirmation bool
	processRegistry      *process.Registry
}

// NewBashTool creates a new bash tool with interactive confirmation support.
// An optional process.Registry enables PTY and background execution.
func NewBashTool(eventBus events.EventBus, requiresConfirmation bool, registry ...*process.Registry) Tool {
	tool := &BashTool{
		eventBus:             eventBus,
		requiresConfirmation: requiresConfirmation,
	}

	if len(registry) > 0 && registry[0] != nil {
		tool.processRegistry = registry[0]
	}

	if eventBus != nil {
		tool.confirmer = NewBusConfirmer(eventBus)
	}

	return tool
}

// Declaration returns the function declaration for the bash tool
func (b *BashTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "bash",
		Description: `Executes a bash command in a persistent shell session.

Usage notes:
- Shell state (environment variables, virtualenvs, current directory) persists across calls. Prefer absolute paths; avoid cd unless the user asks.
- Optional timeout_ms (default 30000, max 300000) and cwd parameters.
- Chain commands with ';' or '&&'. No newlines between commands (newlines are fine inside quoted strings). Pass multi-line content (e.g. commit messages) via a HEREDOC.
- Filter large outputs (| grep, | tail) to keep token usage down, and prefer specialized tools when available: searchInFiles over grep, listFiles over ls, readFile over cat.
- Set requires_confirmation: true for destructive or state-changing commands: rm, sudo, package installs, git commit/push/merge/rebase/reset, and bulk staging like git add -A.
- Never use interactive -i flags (git rebase -i, git add -i) — interactive input is not supported. For interactive or long-running programs (vim, claude, dev servers), use pty: true with background: true to get a session_id, then drive it with the 'process' tool.`,
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
				"requires_confirmation": {
					Type:        ai.TypeBoolean,
					Description: "Whether to explicitly require user confirmation for this specific command execution, overriding the default behavior.",
				},
				"_display_message": {
					Type:        ai.TypeString,
					Description: "A clear, concise description of what the command does (5-10 words).",
				},
				"pty": {
					Type:        ai.TypeBoolean,
					Description: "Allocate a pseudo-terminal for interactive programs (vim, claude, codex). Defaults to false.",
				},
				"background": {
					Type:        ai.TypeBoolean,
					Description: "Run in background, return session ID immediately. Use 'process' tool to poll/write/kill. Defaults to false.",
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
				"results": {
					Type:        ai.TypeString,
					Description: "The command output (stdout and stderr combined)",
				},
				"error": {
					Type:        ai.TypeString,
					Description: "Error message if the command failed",
				},
				"session_id": {
					Type:        ai.TypeString,
					Description: "Session ID for background processes (use with 'process' tool)",
				},
				"state": {
					Type:        ai.TypeString,
					Description: "Process state: running, exited, or failed",
				},
			},
			Required: []string{"success", "results"},
		},
	}
}

// Handler returns the function handler for the bash tool
func (b *BashTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (ai.ToolOutput, error) {
		// Generate execution ID for this tool execution
		executionID := uuid.New().String()

		// Add execution ID to context
		ctx = toolctx.WithExecutionID(ctx, executionID)

		// Extract command parameter
		command, ok := params["command"].(string)
		if !ok {
			return ai.ToolOutput{}, fmt.Errorf("command parameter is required and must be a string")
		}

		// Check for display message and publish event
		if b.eventBus != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				b.eventBus.Publish("tool.call.message", events.ToolCallMessageEvent{
					ToolName: "bash",
					Message:  msg,
				})
			}
		}

		// Determine if confirmation is required for this specific command
		explicitConfirmation, _ := params["requires_confirmation"].(bool)

		// Check if command requires confirmation based on global setting or explicit parameter
		if b.requiresConfirmation || explicitConfirmation {
			confirmed, err := b.requestConfirmation(ctx, executionID, command)
			if err != nil {
				return failedOutput(map[string]any{
					"success": false,
					"results": "",
					"error":   fmt.Sprintf("confirmation failed: %v", err),
				}), nil
			}

			if !confirmed {
				return failedOutput(map[string]any{
					"success": false,
					"results": "",
					"error":   "command cancelled by user",
				}), nil
			}
		}

		// Check for PTY/background execution
		usePTY, _ := params["pty"].(bool)
		background, _ := params["background"].(bool)

		if background && b.processRegistry != nil {
			return b.executeBackground(ctx, command, params, usePTY)
		}
		if usePTY && b.processRegistry != nil {
			return b.executePTYSync(ctx, command, params)
		}

		// Default: existing synchronous execution path
		return b.executeCommand(ctx, command, params)
	}
}

// requestConfirmation requests user confirmation and waits for response
func (b *BashTool) requestConfirmation(ctx context.Context, executionID, command string) (bool, error) {
	if b.confirmer == nil {
		// No confirmer means no way to ask; refuse rather than run unconfirmed.
		return false, fmt.Errorf("confirmation required but no confirmer is configured")
	}

	displayCommand := cleanCommandForDisplay(command)
	request := events.ToolConfirmationRequest{
		ExecutionID: executionID,
		ToolName:    "Bash",
		Command:     command,
		Message:     fmt.Sprintf("Execute '%s'? [y/N]", displayCommand),
	}

	return b.confirmer.ConfirmExecution(ctx, request)
}

// cleanCommandForDisplay removes HEREDOC syntax for better readability in confirmations
func cleanCommandForDisplay(command string) string {
	// Regex to match HEREDOC pattern and extract the message content
	// Pattern: (before)"$(cat <<'EOF'(message content)EOF\n)"(after)
	heredocRegex := regexp.MustCompile(`(?s)^(.*?)"?\$\(cat <<'EOF'\n(.*?)\nEOF\n\)"?\s*(.*)$`)

	matches := heredocRegex.FindStringSubmatch(command)
	if len(matches) == 4 {
		before := strings.TrimSpace(matches[1])
		messageContent := strings.TrimSpace(matches[2])
		after := strings.TrimSpace(matches[3])

		// Remove trailing quote if present
		before = strings.TrimSuffix(before, `"`)
		before = strings.TrimSpace(before)

		// Keep original formatting but trim excessive whitespace
		messageContent = strings.TrimSpace(messageContent)

		result := before + ` "` + messageContent + `"`
		if after != "" {
			result += " " + after
		}
		return result
	}

	return command
}

// resolveCWD extracts the working directory from params or context.
func (b *BashTool) resolveCWD(ctx context.Context, params map[string]any) string {
	if cwdParam, exists := params["cwd"]; exists {
		if cwdStr, ok := cwdParam.(string); ok && cwdStr != "" {
			return cwdStr
		}
	}
	if sessionCwd, ok := toolctx.WorkingDir(ctx); ok && sessionCwd != "" {
		return sessionCwd
	}
	return ""
}

// executeBackground spawns a background session and returns the session ID
// with initial output after a brief warmup period.
func (b *BashTool) executeBackground(ctx context.Context, command string, params map[string]any, usePTY bool) (ai.ToolOutput, error) {
	cwd := b.resolveCWD(ctx, params)

	session, err := b.processRegistry.Spawn(context.Background(), command, cwd, usePTY)
	if err != nil {
		return failedOutput(map[string]any{
			"success": false,
			"results": "",
			"error":   fmt.Sprintf("failed to spawn background process: %v", err),
		}), nil
	}

	// Brief warmup to capture initial output
	time.Sleep(200 * time.Millisecond)

	output := session.Buffer.Drain()

	state, _ := session.GetState()

	return resultOutput(map[string]any{
		"success":    true,
		"results":    output,
		"session_id": session.ID,
		"state":      string(state),
	}), nil
}

// executePTYSync spawns a PTY session and waits for it to complete (or timeout).
func (b *BashTool) executePTYSync(ctx context.Context, command string, params map[string]any) (ai.ToolOutput, error) {
	cwd := b.resolveCWD(ctx, params)

	timeout := 30 * time.Second
	if timeoutParam, exists := params["timeout_ms"]; exists {
		if timeoutMs, ok := timeoutParam.(float64); ok {
			timeout = time.Duration(timeoutMs) * time.Millisecond
		}
	}

	session, err := b.processRegistry.Spawn(context.Background(), command, cwd, true)
	if err != nil {
		// Fall back to standard execution if PTY spawn fails
		return b.executeCommand(ctx, command, params)
	}

	// Wait for exit or timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-session.Done():
		// Process finished
	case <-timer.C:
		// Timeout — return what we have with session_id for further interaction
		output := session.Buffer.Snapshot()
		return failedOutput(map[string]any{
			"success":    false,
			"results":    output,
			"session_id": session.ID,
			"state":      "running",
			"error":      fmt.Sprintf("command still running after %v, use process tool with session_id to interact", timeout),
		}), nil
	}

	output := session.Buffer.Snapshot()

	state, exitCode := session.GetState()

	success := exitCode == 0

	result := map[string]any{
		"success": success,
		"results": output,
		"state":   string(state),
	}

	if !success {
		result["error"] = fmt.Sprintf("command failed with exit code %d", exitCode)
	}

	if success {
		return resultOutput(result), nil
	}
	return failedOutput(result), nil
}

// executeCommand executes the bash command
func (b *BashTool) executeCommand(ctx context.Context, command string, params map[string]any) (ai.ToolOutput, error) {
	cwd := b.resolveCWD(ctx, params)

	// Extract optional timeout
	timeout := 30 * time.Second // Default 30s timeout
	if timeoutParam, exists := params["timeout_ms"]; exists {
		if timeoutMs, ok := timeoutParam.(float64); ok {
			timeout = time.Duration(timeoutMs) * time.Millisecond
		}
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create the command using the user's shell, validated against /etc/shells.
	cmd := exec.CommandContext(execCtx, process.UserShell(), "-l", "-c", command)

	// Set working directory if provided
	if cwd != "" {
		cmd.Dir = cwd
	}

	// Inherit parent env (includes vars from .zshrc when launched from interactive terminal).
	cmd.Env = os.Environ()

	// Kill the whole process group on timeout/cancel and bound how long
	// exited-but-inherited output pipes may stay open, so a background
	// grandchild (e.g. "some-daemon &") cannot hang the agent turn.
	process.ConfigureGroupKill(cmd)
	cmd.WaitDelay = 3 * time.Second

	// Execute command and capture output
	output, err := cmd.CombinedOutput()

	// Check for timeout
	if execCtx.Err() == context.DeadlineExceeded {
		return failedOutput(map[string]any{
			"success": false,
			"results": string(output),
			"error":   fmt.Sprintf("command timed out after %v", timeout),
		}), nil
	}

	// Check for other errors
	if err != nil {
		return failedOutput(map[string]any{
			"success": false,
			"results": string(output),
			"error":   fmt.Sprintf("command failed: %v", err),
		}), nil
	}

	return resultOutput(map[string]any{
		"success": true,
		"results": string(output),
	}), nil
}

// FormatOutput formats bash command results for user display
func (b *BashTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	output, _ := result["results"].(string)
	errorMsg, _ := result["error"].(string)

	if !success {
		if errorMsg != "" {
			return fmt.Sprintf("**Command Failed**\n```\n%s\n```", errorMsg)
		}
		return "**Command Failed**"
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return "**Command completed successfully**"
	}

	// Format output nicely in a code block
	return fmt.Sprintf("**Command Output**\n```\n%s\n```", output)
}
