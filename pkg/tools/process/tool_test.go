package process

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTool() (*Tool, *Registry) {
	r := NewRegistry()
	bus := &events.NoOpEventBus{}
	t := NewTool(r, bus)
	return t, r
}

func TestProcessTool_Declaration(t *testing.T) {
	tool, _ := newTestTool()
	decl := tool.Declaration()
	assert.Equal(t, "process", decl.Name)
	assert.Contains(t, decl.Parameters.Properties, "action")
	assert.Contains(t, decl.Parameters.Properties, "session_id")
	assert.Contains(t, decl.Parameters.Properties, "data")
	assert.Contains(t, decl.Parameters.Properties, "keys")
}

func TestProcessTool_ListEmpty(t *testing.T) {
	tool, _ := newTestTool()
	handler := tool.Handler()

	result, err := handler(context.Background(), map[string]any{
		"action": "list",
	})
	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
	sessions := result["sessions"].([]map[string]any)
	assert.Empty(t, sessions)
}

func TestProcessTool_ListWithSessions(t *testing.T) {
	tool, r := newTestTool()
	defer r.Shutdown()

	handler := tool.Handler()

	s, err := r.Spawn(context.Background(), "sleep 5", "", false)
	require.NoError(t, err)

	result, err := handler(context.Background(), map[string]any{
		"action": "list",
	})
	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
	sessions := result["sessions"].([]map[string]any)
	assert.Len(t, sessions, 1)
	assert.Equal(t, s.ID, sessions[0]["id"])

	s.Kill()
	s.Wait()
}

func TestProcessTool_Poll(t *testing.T) {
	tool, r := newTestTool()
	defer r.Shutdown()

	handler := tool.Handler()

	s, err := r.Spawn(context.Background(), "echo 'poll test'", "", false)
	require.NoError(t, err)
	s.Wait()
	time.Sleep(50 * time.Millisecond)

	result, err := handler(context.Background(), map[string]any{
		"action":     "poll",
		"session_id": s.ID,
	})
	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
	assert.Contains(t, result["output"].(string), "poll test")
	assert.NotEmpty(t, result["state"])
}

func TestProcessTool_PollNoNewData(t *testing.T) {
	tool, r := newTestTool()
	defer r.Shutdown()

	handler := tool.Handler()

	s, err := r.Spawn(context.Background(), "echo 'once'", "", false)
	require.NoError(t, err)
	s.Wait()
	time.Sleep(50 * time.Millisecond)

	// First poll gets data
	result, _ := handler(context.Background(), map[string]any{
		"action":     "poll",
		"session_id": s.ID,
	})
	assert.Contains(t, result["output"].(string), "once")

	// Second poll — no new data
	result, _ = handler(context.Background(), map[string]any{
		"action":     "poll",
		"session_id": s.ID,
	})
	assert.Equal(t, "", result["output"].(string))
}

func TestProcessTool_Kill(t *testing.T) {
	tool, r := newTestTool()
	defer r.Shutdown()

	handler := tool.Handler()

	s, err := r.Spawn(context.Background(), "sleep 60", "", false)
	require.NoError(t, err)

	result, err := handler(context.Background(), map[string]any{
		"action":     "kill",
		"session_id": s.ID,
	})
	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
	assert.NotEqual(t, "running", result["state"])
}

func TestProcessTool_MissingSessionID(t *testing.T) {
	tool, _ := newTestTool()
	handler := tool.Handler()

	result, err := handler(context.Background(), map[string]any{
		"action": "poll",
	})
	require.NoError(t, err)
	assert.False(t, result["success"].(bool))
	assert.Contains(t, result["error"].(string), "session_id")
}

func TestProcessTool_InvalidAction(t *testing.T) {
	tool, _ := newTestTool()
	handler := tool.Handler()

	result, err := handler(context.Background(), map[string]any{
		"action": "invalid",
	})
	require.NoError(t, err)
	assert.False(t, result["success"].(bool))
	assert.Contains(t, result["error"].(string), "unknown action")
}

func TestProcessTool_SessionNotFound(t *testing.T) {
	tool, _ := newTestTool()
	handler := tool.Handler()

	result, err := handler(context.Background(), map[string]any{
		"action":     "poll",
		"session_id": "nonexistent",
	})
	require.NoError(t, err)
	assert.False(t, result["success"].(bool))
	assert.Contains(t, result["error"].(string), "not found")
}

func TestProcessTool_Write(t *testing.T) {
	tool, r := newTestTool()
	defer r.Shutdown()

	handler := tool.Handler()

	// Both PTY and pipe-based sessions support write via stdin pipe
	s, err := r.Spawn(context.Background(), "cat", "", false)
	require.NoError(t, err)

	result, err := handler(context.Background(), map[string]any{
		"action":     "write",
		"session_id": s.ID,
		"data":       "hello\n",
	})
	require.NoError(t, err)
	assert.True(t, result["success"].(bool))

	s.Kill()
	s.Wait()
}

func TestProcessTool_WriteEmptyData(t *testing.T) {
	tool, r := newTestTool()
	defer r.Shutdown()

	handler := tool.Handler()

	s, err := r.Spawn(context.Background(), "cat", "", true)
	require.NoError(t, err)

	result, err := handler(context.Background(), map[string]any{
		"action":     "write",
		"session_id": s.ID,
		"data":       "",
	})
	require.NoError(t, err)
	assert.False(t, result["success"].(bool))
	assert.Contains(t, result["error"].(string), "data is required")

	s.Kill()
	s.Wait()
}

func TestProcessTool_SendKeys(t *testing.T) {
	tool, r := newTestTool()
	defer r.Shutdown()

	handler := tool.Handler()

	s, err := r.Spawn(context.Background(), "cat", "", true)
	require.NoError(t, err)

	if s.ptyFile == nil {
		t.Skip("PTY not available")
	}

	result, err := handler(context.Background(), map[string]any{
		"action":     "send_keys",
		"session_id": s.ID,
		"keys":       []interface{}{"C-c"},
	})
	require.NoError(t, err)
	assert.True(t, result["success"].(bool))

	s.Kill()
	s.Wait()
}

func TestProcessTool_SendKeysMissing(t *testing.T) {
	tool, r := newTestTool()
	defer r.Shutdown()

	handler := tool.Handler()

	s, err := r.Spawn(context.Background(), "sleep 1", "", false)
	require.NoError(t, err)

	result, err := handler(context.Background(), map[string]any{
		"action":     "send_keys",
		"session_id": s.ID,
	})
	require.NoError(t, err)
	assert.False(t, result["success"].(bool))
	assert.Contains(t, result["error"].(string), "keys is required")

	s.Kill()
	s.Wait()
}

func TestProcessTool_FormatOutput(t *testing.T) {
	tool, _ := newTestTool()

	out := tool.FormatOutput(map[string]interface{}{
		"output": "hello world",
		"state":  "exited",
	})
	assert.Contains(t, out, "hello world")
	assert.Contains(t, out, "exited")

	out = tool.FormatOutput(map[string]interface{}{
		"error": "something went wrong",
	})
	assert.Contains(t, out, "something went wrong")
}
