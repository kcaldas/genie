package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// BashTool executes bash commands with optional interactive confirmation
type BashTool struct {
	publisher            events.Publisher
	subscriber           events.Subscriber
	confirmationChannels map[string]chan bool
	confirmationMutex    sync.RWMutex
	requiresConfirmation bool
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
		Name: "bash",
		Description: `Executes a given bash command in a persistent shell session with optional timeout, ensuring proper handling and security measures.

Usage notes:
The command argument is required.
You can specify an optional timeout in milliseconds (up to 300000ms / 5 minutes). If not specified, commands will timeout after 30 seconds.
You can specify an optional working directory (cwd) parameter.
Use "| grep" frequently to filter the output and reduce token usage. For example:
		- If you are build the system - Use grep to filter the results instead of getting the full meaninless response.
		- If you will use cat and know what you are looking for
*IMPORTANT:* Use requires_confirmation: true for destructive or invasive commands that modify state, including:
- git commit, git push, git merge, git rebase, git reset
- rm, rmdir commands that delete files
- sudo commands
- Package installation/removal (npm install, pip install, etc.)
- File modifications that could lose data
Prefer specialized tools when available (use searchInFiles instead of grep, listFiles instead of ls, readFile instead of cat).
When issuing multiple commands, use the ';' or '&&' operator to separate them. DO NOT use newlines (newlines are ok in quoted strings).
*IMPORTANT:* All commands share the same shell session. Shell state (environment variables, virtual environments, current directory, etc.) persist between commands. For example, if you set an environment variable as part of a command, the environment variable will persist for subsequent commands.
Try to maintain your current working directory throughout the session by using absolute paths and avoiding usage of cd. You may use cd if the User explicitly requests it.

Committing changes with git:
When the user asks you to create a new git commit, follow these steps carefully:

Start with a single message that contains exactly three tool_use blocks that do the following (it is VERY IMPORTANT that you send these tool_use blocks in a single message, otherwise it will feel slow to the user!):

Run a git status command to see all untracked files.
Run a git diff command to see both staged and unstaged changes that will be committed.
Run a git log command to see recent commit messages, so that you can follow this repository's commit message style.
Use the git context at the start of this conversation to determine which files are relevant to your commit. Add relevant untracked files to the staging area. Do not commit files that were already modified at the start of this conversation, if they are not relevant to your commit.

Analyze all staged changes (both previously staged and newly added) and draft a commit message:

Use the _display_message parameter to communicate your analysis to the user. Include:
- List the files that have been changed or added
- Summarize the nature of the changes (eg. new feature, enhancement to an existing feature, bug fix, refactoring, test, docs, etc.)
- Brainstorm the purpose or motivation behind these changes
- Do not use tools to explore code, beyond what is available in the git context
- Assess the impact of these changes on the overall project
- Check for any sensitive information that shouldn't be committed
- Draft a concise (1-2 sentences) commit message that focuses on the "why" rather than the "what"
- Ensure your language is clear, concise, and to the point
- Ensure the message accurately reflects the changes and their purpose (i.e. "add" means a wholly new feature, "update" means an enhancement to an existing feature, "fix" means a bug fix, etc.)
- Ensure the message is not generic (avoid words like "Update" or "Fix" without context)
- Review the draft message to ensure it accurately reflects the changes and their purpose
IMPORTANT: Git commits are destructive operations that require user confirmation. ALWAYS use requires_confirmation: true for git commit commands.

In order to ensure good formatting, ALWAYS pass the commit message via a HEREDOC, a la this example:
git commit -m "$(cat <<'EOF'
Commit message here.
Another line.
EOF
)" with requires_confirmation: true

If the commit fails due to pre-commit hook changes, retry the commit ONCE to include these automated changes. If it fails again, it usually means a pre-commit hook is preventing the commit. If the commit succeeds but you notice that files were modified by the pre-commit hook, you MUST amend your commit to include them.

Finally, run git status to make sure the commit succeeded.

Important notes:
When possible, combine the "git add" and "git commit" commands into a single "git commit -am" command, to speed things up
However, be careful not to stage files (e.g. with git add .) for commits that aren't part of the change, they may have untracked files they want to keep around, but not commit.
IMPORTANT: Use requires_confirmation: true for git add . or git add -A commands that stage many files at once.
NEVER update the git config
DO NOT push to the remote repository
IMPORTANT: Never use git commands with the -i flag (like git rebase -i or git add -i) since they require interactive input which is not supported.
If there are no changes to commit (i.e., no untracked files and no modifications), do not create an empty commit
Ensure your commit message is meaningful and concise. It should explain the purpose of the changes, not just describe them.

Creating pull requests:
Use the gh command for GitHub-related tasks including working with issues, pull requests, checks, and releases.

IMPORTANT: When the user asks you to create a pull request, follow these steps carefully:

Understand the current state of the branch. Remember to send a single message that contains multiple tool_use blocks (it is VERY IMPORTANT that you do this in a single message, otherwise it will feel slow to the user!):

Run a git status command to see all untracked files.
Run a git diff command to see both staged and unstaged changes that will be committed.
Check if the current branch tracks a remote branch and is up to date with the remote, so you know if you need to push to the remote
Run a git log command and git diff main...HEAD to understand the full commit history for the current branch (from the time it diverged from the main branch.)
Create new branch if needed

Commit changes if needed

Push to remote with -u flag if needed

Analyze all changes that will be included in the pull request, making sure to look at all relevant commits (not just the latest commit, but all commits that will be included in the pull request!), and draft a pull request summary:

Use the _display_message parameter to communicate your analysis to the user. Include:
- List the commits since diverging from the main branch
- Summarize the nature of the changes (eg. new feature, enhancement to an existing feature, bug fix, refactoring, test, docs, etc.)
- Brainstorm the purpose or motivation behind these changes
- Assess the impact of these changes on the overall project
- Do not use tools to explore code, beyond what is available in the git context
- Check for any sensitive information that shouldn't be committed
- Draft a concise (1-2 bullet points) pull request summary that focuses on the "why" rather than the "what"
- Ensure the summary accurately reflects all changes since diverging from the main branch
- Ensure your language is clear, concise, and to the point
- Ensure the summary accurately reflects the changes and their purpose (ie. "add" means a wholly new feature, "update" means an enhancement to an existing feature, "fix" means a bug fix, etc.)
- Ensure the summary is not generic (avoid words like "Update" or "Fix" without context)
- Review the draft summary to ensure it accurately reflects the changes and their purpose
Create PR using gh pr create with the format below. Use a HEREDOC to pass the body to ensure correct formatting.
gh pr create --title "the pr title" --body "$(cat <<'EOF'
## Summary
<1-3 bullet points>
EOF
)"`,
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
			},
			Required: []string{"success", "results"},
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

		// Check for display message and publish event
		if b.publisher != nil {
			if msg, ok := params["_display_message"].(string); ok && msg != "" {
				b.publisher.Publish("tool.call.message", events.ToolCallMessageEvent{
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
				return map[string]any{
					"success": false,
					"results": "",
					"error":   fmt.Sprintf("confirmation failed: %v", err),
				}, nil
			}

			if !confirmed {
				return map[string]any{
					"success": false,
					"results": "",
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

	// Create and publish confirmation request
	displayCommand := cleanCommandForDisplay(command)
	request := events.ToolConfirmationRequest{
		ExecutionID: executionID,
		ToolName:    "Bash",
		Command:     command,
		Message:     fmt.Sprintf("Execute '%s'? [y/N]", displayCommand),
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

// cleanCommandForDisplay removes HEREDOC syntax for better readability in confirmations
func cleanCommandForDisplay(command string) string {
	// Regex to match HEREDOC pattern and extract the message content
	// Pattern: (before)"$(cat <<'EOF'(message content)EOF\n)"(after)
	heredocRegex := regexp.MustCompile(`(?s)^(.*?)"?\$\(cat <<'EOF'\n(.*?)\nEOF\n\)"?\s*(.*)$`)
	
	matches := heredocRegex.FindStringSubmatch(command)
	if matches != nil && len(matches) == 4 {
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

// executeCommand executes the bash command
func (b *BashTool) executeCommand(ctx context.Context, command string, params map[string]any) (map[string]any, error) {
	// Extract optional working directory
	var cwd string
	if cwdParam, exists := params["cwd"]; exists {
		if cwdStr, ok := cwdParam.(string); ok {
			cwd = cwdStr
		}
	}

	// If no explicit cwd provided, use session working directory from context
	if cwd == "" {
		if sessionCwd := ctx.Value("cwd"); sessionCwd != nil {
			if sessionCwdStr, ok := sessionCwd.(string); ok && sessionCwdStr != "" {
				cwd = sessionCwdStr
			}
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
			"results": string(output),
			"error":   fmt.Sprintf("command timed out after %v", timeout),
		}, nil
	}

	// Check for other errors
	if err != nil {
		return map[string]any{
			"success": false,
			"results": string(output),
			"error":   fmt.Sprintf("command failed: %v", err),
		}, nil
	}

	return map[string]any{
		"success": true,
		"results": string(output),
	}, nil
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
