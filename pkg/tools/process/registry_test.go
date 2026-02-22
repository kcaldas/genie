package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_Spawn(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "echo hello", "", false)
	require.NoError(t, err)
	assert.NotEmpty(t, s.ID)
	assert.Equal(t, "echo hello", s.Command)
	// Use GetState() to avoid racing with the finish goroutine.
	state, _ := s.GetState()
	// echo is fast — may already be finished by the time we check.
	assert.Contains(t, []State{StateRunning, StateExited}, state)

	s.Wait()
}

func TestRegistry_SpawnWithPTY(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "echo pty-test", "", true)
	require.NoError(t, err)

	s.Wait()
	time.Sleep(50 * time.Millisecond)

	snap := s.Buffer.Snapshot()
	assert.Contains(t, snap, "pty-test")

	// Verify PTY was used if available (may fall back to pipes in sandbox)
	if s.ptyFile != nil {
		t.Log("PTY was allocated successfully")
	} else {
		t.Log("PTY not available, fell back to pipes")
	}
}

func TestRegistry_SpawnWithCWD(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "pwd", "/tmp", false)
	require.NoError(t, err)
	s.Wait()

	time.Sleep(50 * time.Millisecond)
	snap := s.Buffer.Snapshot()
	assert.Contains(t, snap, "/tmp")
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "sleep 1", "", false)
	require.NoError(t, err)

	got, ok := r.Get(s.ID)
	assert.True(t, ok)
	assert.Equal(t, s.ID, got.ID)

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)

	s.Kill()
	s.Wait()
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s1, _ := r.Spawn(context.Background(), "sleep 1", "", false)
	time.Sleep(10 * time.Millisecond) // ensure different timestamps
	s2, _ := r.Spawn(context.Background(), "sleep 1", "", false)

	sessions := r.List()
	assert.Len(t, sessions, 2)
	// Newest first
	assert.Equal(t, s2.ID, sessions[0].ID)
	assert.Equal(t, s1.ID, sessions[1].ID)

	s1.Kill()
	s2.Kill()
	s1.Wait()
	s2.Wait()
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "sleep 60", "", false)
	require.NoError(t, err)

	err = r.Remove(s.ID)
	require.NoError(t, err)

	_, ok := r.Get(s.ID)
	assert.False(t, ok)
}

func TestRegistry_RemoveNotFound(t *testing.T) {
	r := NewRegistry()
	err := r.Remove("nonexistent")
	assert.Error(t, err)
}

func TestRegistry_MaxSessions(t *testing.T) {
	r := &Registry{
		sessions:    make(map[string]*Session),
		maxSessions: 2,
		finishedTTL: DefaultFinishedTTL,
		headBufSize: 1024,
		tailBufSize: 1024,
	}
	defer r.Shutdown()

	s1, err := r.Spawn(context.Background(), "sleep 60", "", false)
	require.NoError(t, err)
	s2, err := r.Spawn(context.Background(), "sleep 60", "", false)
	require.NoError(t, err)

	// Third should fail (both running, can't evict)
	_, err = r.Spawn(context.Background(), "sleep 60", "", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max sessions")

	s1.Kill()
	s2.Kill()
	s1.Wait()
	s2.Wait()
}

func TestRegistry_MaxSessionsLRUEviction(t *testing.T) {
	r := &Registry{
		sessions:    make(map[string]*Session),
		maxSessions: 2,
		finishedTTL: 1 * time.Hour, // long TTL so cleanup doesn't remove
		headBufSize: 1024,
		tailBufSize: 1024,
	}
	defer r.Shutdown()

	// Spawn and let first finish
	s1, err := r.Spawn(context.Background(), "echo first", "", false)
	require.NoError(t, err)
	s1.Wait()

	// Spawn second (running)
	s2, err := r.Spawn(context.Background(), "sleep 60", "", false)
	require.NoError(t, err)

	// Third spawn should evict finished s1
	s3, err := r.Spawn(context.Background(), "sleep 60", "", false)
	require.NoError(t, err)

	_, ok := r.Get(s1.ID)
	assert.False(t, ok, "finished session should have been evicted")

	s2.Kill()
	s3.Kill()
	s2.Wait()
	s3.Wait()
}

func TestRegistry_FinishedTTLCleanup(t *testing.T) {
	r := &Registry{
		sessions:    make(map[string]*Session),
		maxSessions: DefaultMaxSessions,
		finishedTTL: 50 * time.Millisecond,
		headBufSize: 1024,
		tailBufSize: 1024,
	}
	defer r.Shutdown()

	s, err := r.Spawn(context.Background(), "echo done", "", false)
	require.NoError(t, err)
	s.Wait()

	// Session should exist immediately
	_, ok := r.Get(s.ID)
	assert.True(t, ok)

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	r.Cleanup()

	_, ok = r.Get(s.ID)
	assert.False(t, ok, "expired session should have been cleaned up")
}

func TestRegistry_Shutdown(t *testing.T) {
	r := NewRegistry()

	s1, _ := r.Spawn(context.Background(), "sleep 60", "", false)
	s2, _ := r.Spawn(context.Background(), "sleep 60", "", false)

	r.Shutdown()

	s1.Wait()
	s2.Wait()

	assert.False(t, s1.IsRunning())
	assert.False(t, s2.IsRunning())
	assert.Empty(t, r.List())
}

func TestRegistry_PTYFallbackToPipes(t *testing.T) {
	r := NewRegistry()
	defer r.Shutdown()

	// Even if PTY is requested, should work (either PTY succeeds or falls back to pipes)
	s, err := r.Spawn(context.Background(), "echo fallback-test", "", true)
	require.NoError(t, err)
	s.Wait()

	time.Sleep(50 * time.Millisecond)
	snap := s.Buffer.Snapshot()
	assert.Contains(t, snap, "fallback-test")

	if s.ptyFile != nil {
		t.Log("PTY was used")
	} else {
		t.Log("Fell back to pipes (PTY not available)")
	}
}
