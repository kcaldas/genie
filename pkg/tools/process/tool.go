package process

import (
	"context"
	"fmt"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// Tool manages background process sessions. It implements the tools.Tool
// interface from the parent package (duck-typed).
type Tool struct {
	registry *Registry
	eventBus events.EventBus
}

// NewTool creates a new process management tool.
func NewTool(registry *Registry, eventBus events.EventBus) *Tool {
	return &Tool{
		registry: registry,
		eventBus: eventBus,
	}
}

// Declaration returns the function declaration for the process tool.
func (t *Tool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "process",
		Description: `Manage background process sessions spawned by the bash tool.
Use this tool to interact with long-running or interactive processes.

Actions:
- list: Show all active sessions with their status
- poll: Get new output from a session since last poll
- write: Send raw text to a session's stdin
- send_keys: Send special keys (Ctrl+C, Enter, arrows, etc.) using tmux-style names
- kill: Terminate a session

Key names for send_keys: Enter, C-c (Ctrl+C), C-d (Ctrl+D), C-z, Escape, Tab, Backspace, Space, Up, Down, Left, Right, Home, End, Delete, F1-F12.`,
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"action": {
					Type:        ai.TypeString,
					Description: "The action to perform on the session",
					Enum:        []string{"list", "poll", "write", "send_keys", "kill"},
				},
				"session_id": {
					Type:        ai.TypeString,
					Description: "The session ID to operate on (required for all actions except list)",
				},
				"data": {
					Type:        ai.TypeString,
					Description: "Raw text to send to the session's stdin (for 'write' action)",
				},
				"keys": {
					Type:        ai.TypeArray,
					Description: "Array of tmux-style key names to send (for 'send_keys' action). Examples: [\"C-c\"], [\"h\",\"e\",\"l\",\"l\",\"o\",\"Enter\"]",
					Items: &ai.Schema{
						Type: ai.TypeString,
					},
				},
			},
			Required: []string{"action"},
		},
		Response: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"success": {
					Type: ai.TypeBoolean,
				},
				"sessions": {
					Type: ai.TypeArray,
					Items: &ai.Schema{
						Type: ai.TypeObject,
					},
				},
				"output": {
					Type: ai.TypeString,
				},
				"state": {
					Type: ai.TypeString,
				},
				"exit_code": {
					Type: ai.TypeInteger,
				},
				"error": {
					Type: ai.TypeString,
				},
			},
		},
	}
}

// Handler returns the function handler for the process tool.
func (t *Tool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		action, _ := params["action"].(string)

		switch action {
		case "list":
			return t.handleList()
		case "poll":
			return t.handlePoll(params)
		case "write":
			return t.handleWrite(params)
		case "send_keys":
			return t.handleSendKeys(params)
		case "kill":
			return t.handleKill(params)
		default:
			return map[string]any{
				"success": false,
				"error":   fmt.Sprintf("unknown action: %q (valid: list, poll, write, send_keys, kill)", action),
			}, nil
		}
	}
}

// FormatOutput formats process tool results for user display.
func (t *Tool) FormatOutput(result map[string]interface{}) string {
	if output, ok := result["output"].(string); ok && output != "" {
		state, _ := result["state"].(string)
		return fmt.Sprintf("**Process Output** (state: %s)\n```\n%s\n```", state, output)
	}

	if sessions, ok := result["sessions"].([]map[string]any); ok {
		if len(sessions) == 0 {
			return "**No active sessions**"
		}
		out := fmt.Sprintf("**%d active session(s)**\n", len(sessions))
		for _, s := range sessions {
			out += fmt.Sprintf("- `%s` %s (%s)\n", s["id"], s["command"], s["state"])
		}
		return out
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Sprintf("**Error**: %s", errMsg)
	}

	return "**Done**"
}

func (t *Tool) getSession(params map[string]any) (*Session, map[string]any) {
	sessionID, _ := params["session_id"].(string)
	if sessionID == "" {
		return nil, map[string]any{
			"success": false,
			"error":   "session_id is required for this action",
		}
	}

	session, ok := t.registry.Get(sessionID)
	if !ok {
		return nil, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("session %q not found", sessionID),
		}
	}

	return session, nil
}

func (t *Tool) handleList() (map[string]any, error) {
	sessions := t.registry.List()
	result := make([]map[string]any, 0, len(sessions))

	for _, s := range sessions {
		state, exitCode := s.GetState()
		entry := map[string]any{
			"id":           s.ID,
			"command":      s.Command,
			"state":        string(state),
			"exit_code":    exitCode,
			"created_at":   s.CreatedAt.Format(time.RFC3339),
			"output_bytes": s.Buffer.TotalBytes(),
		}
		result = append(result, entry)
	}

	return map[string]any{
		"success":  true,
		"sessions": result,
	}, nil
}

func (t *Tool) handlePoll(params map[string]any) (map[string]any, error) {
	session, errResult := t.getSession(params)
	if errResult != nil {
		return errResult, nil
	}

	output := session.Buffer.Drain()

	session.SetLastPolled(time.Now())
	state, exitCode := session.GetState()

	return map[string]any{
		"success":   true,
		"output":    output,
		"state":     string(state),
		"exit_code": exitCode,
	}, nil
}

func (t *Tool) handleWrite(params map[string]any) (map[string]any, error) {
	session, errResult := t.getSession(params)
	if errResult != nil {
		return errResult, nil
	}

	data, _ := params["data"].(string)
	if data == "" {
		return map[string]any{
			"success": false,
			"error":   "data is required for write action",
		}, nil
	}

	if err := session.Write([]byte(data)); err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("write failed: %v", err),
		}, nil
	}

	return map[string]any{
		"success": true,
	}, nil
}

func (t *Tool) handleSendKeys(params map[string]any) (map[string]any, error) {
	session, errResult := t.getSession(params)
	if errResult != nil {
		return errResult, nil
	}

	keysRaw, ok := params["keys"]
	if !ok {
		return map[string]any{
			"success": false,
			"error":   "keys is required for send_keys action",
		}, nil
	}

	// Parse keys array from interface
	keysArr, ok := keysRaw.([]interface{})
	if !ok {
		return map[string]any{
			"success": false,
			"error":   "keys must be an array of strings",
		}, nil
	}

	keys := make([]string, 0, len(keysArr))
	for _, k := range keysArr {
		if s, ok := k.(string); ok {
			keys = append(keys, s)
		}
	}

	if err := session.SendKeys(keys); err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("send_keys failed: %v", err),
		}, nil
	}

	return map[string]any{
		"success": true,
	}, nil
}

func (t *Tool) handleKill(params map[string]any) (map[string]any, error) {
	session, errResult := t.getSession(params)
	if errResult != nil {
		return errResult, nil
	}

	if err := session.Kill(); err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("kill failed: %v", err),
		}, nil
	}

	session.Wait()

	output := session.Buffer.Snapshot()

	state, exitCode := session.GetState()

	return map[string]any{
		"success":   true,
		"output":    output,
		"state":     string(state),
		"exit_code": exitCode,
	}, nil
}
