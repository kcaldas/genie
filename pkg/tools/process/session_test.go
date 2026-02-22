package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_WriteToRunningProcess(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	// Start cat which echoes stdin to stdout
	s, err := r.Spawn(context.Background(), "cat", "", false)
	require.NoError(t, err)

	// Both PTY and pipe-based sessions should support write
	err = s.Write([]byte("hello\n"))
	assert.NoError(t, err)

	// Give cat time to echo back
	time.Sleep(100 * time.Millisecond)

	snap := s.Buffer.Snapshot()
	assert.Contains(t, snap, "hello")

	s.Kill()
	s.Wait()
}

func TestSession_WriteToFinishedProcess(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "echo done", "", false)
	require.NoError(t, err)

	s.Wait()
	err = s.Write([]byte("hello"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestSession_SendKeys(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	// Start a process with PTY
	s, err := r.Spawn(context.Background(), "cat", "", true)
	require.NoError(t, err)

	if s.ptyFile == nil {
		// PTY not available (sandbox), skip interactive test
		t.Skip("PTY not available in this environment")
	}

	// Send keys and verify no error
	err = s.SendKeys([]string{"h", "e", "l", "l", "o", "Enter"})
	require.NoError(t, err)

	// Give it a moment to process
	time.Sleep(100 * time.Millisecond)

	// Kill with Ctrl+D via send keys
	err = s.SendKeys([]string{"C-d"})
	require.NoError(t, err)

	s.Wait()
}

func TestSession_Kill(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "sleep 60", "", false)
	require.NoError(t, err)

	assert.True(t, s.IsRunning())

	err = s.Kill()
	require.NoError(t, err)

	s.Wait()
	assert.False(t, s.IsRunning())
}

func TestSession_KillAlreadyFinished(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "echo hi", "", false)
	require.NoError(t, err)

	s.Wait()
	assert.False(t, s.IsRunning())

	// Kill on finished session should be a no-op
	err = s.Kill()
	require.NoError(t, err)
}

func TestSession_IsRunningTransitions(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "sleep 0.1", "", false)
	require.NoError(t, err)

	assert.True(t, s.IsRunning())

	s.Wait()
	assert.False(t, s.IsRunning())
	assert.NotEqual(t, StateRunning, s.State)
}

func TestSession_ExitCode(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	// Success
	s1, err := r.Spawn(context.Background(), "exit 0", "", false)
	require.NoError(t, err)
	s1.Wait()
	assert.Equal(t, 0, s1.ExitCode)
	assert.Equal(t, StateExited, s1.State)

	// Failure
	s2, err := r.Spawn(context.Background(), "exit 42", "", false)
	require.NoError(t, err)
	s2.Wait()
	assert.Equal(t, 42, s2.ExitCode)
	assert.Equal(t, StateFailed, s2.State)
}

func TestSession_OutputCapture(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "echo 'hello world'", "", false)
	require.NoError(t, err)
	s.Wait()

	// Give io.Copy goroutine a moment to flush
	time.Sleep(50 * time.Millisecond)

	snap := s.Buffer.Snapshot()
	assert.Contains(t, snap, "hello world")
}

func TestSession_PTYOutputCapture(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "echo 'pty hello'", "", true)
	require.NoError(t, err)
	s.Wait()

	time.Sleep(50 * time.Millisecond)

	snap := s.Buffer.Snapshot()
	assert.Contains(t, snap, "pty hello")

	if s.ptyFile != nil {
		t.Log("PTY was used")
	} else {
		t.Log("Fell back to pipes")
	}
}
