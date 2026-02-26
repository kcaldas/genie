package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultMaxSessions    = 64
	DefaultFinishedTTL    = 5 * time.Minute
	DefaultHeadBufferSize = 512 * 1024 // 512KB
	DefaultTailBufferSize = 512 * 1024 // 512KB
)

// Registry manages background process sessions.
type Registry struct {
	mu          sync.Mutex
	sessions    map[string]*Session
	maxSessions int
	finishedTTL time.Duration
	headBufSize int
	tailBufSize int
}

// NewRegistry creates a new process registry with default settings.
func NewRegistry() *Registry {
	return &Registry{
		sessions:    make(map[string]*Session),
		maxSessions: DefaultMaxSessions,
		finishedTTL: DefaultFinishedTTL,
		headBufSize: DefaultHeadBufferSize,
		tailBufSize: DefaultTailBufferSize,
	}
}

// Spawn creates and starts a new session. If usePTY is true, allocates a
// pseudo-terminal; falls back to pipes on failure.
func (r *Registry) Spawn(ctx context.Context, command, cwd string, usePTY bool) (*Session, error) {
	r.mu.Lock()
	r.cleanupLocked()

	if len(r.sessions) >= r.maxSessions {
		r.mu.Unlock()
		return nil, fmt.Errorf("max sessions reached (%d)", r.maxSessions)
	}
	r.mu.Unlock()

	sessionCtx, cancel := context.WithCancel(ctx)

	buf := NewHeadTailBuffer(r.headBufSize, r.tailBufSize)

	session := &Session{
		ID:        uuid.New().String()[:8],
		Command:   command,
		CWD:       cwd,
		State:     StateRunning,
		Buffer:    buf,
		CreatedAt: time.Now(),
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	var started bool

	if usePTY {
		cmd := r.makeCmd(sessionCtx, command, cwd)
		started = startWithPTY(session, cmd, buf)
		if started {
			session.cmd = cmd
		}
	}

	if !started {
		// Create a fresh cmd — PTY attempt may have mutated the previous one
		cmd := r.makeCmd(sessionCtx, command, cwd)
		session.cmd = cmd
		if err := r.startWithPipes(session, cmd, buf); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to start process: %w", err)
		}
	}

	// Goroutine to wait for process exit
	go func() {
		err := session.cmd.Wait()
		exitCode := 0
		state := StateExited
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
			state = StateFailed
		}
		session.finish(exitCode, state)
		close(session.done)
	}()

	r.mu.Lock()
	r.sessions[session.ID] = session
	r.mu.Unlock()

	return session, nil
}

// makeCmd creates a fresh exec.Cmd configured for process group isolation.
// Uses the user's shell (validated against /etc/shells) without login mode;
// env vars are inherited explicitly via os.Environ().
func (r *Registry) makeCmd(ctx context.Context, command, cwd string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, UserShell(), "-c", command)
	setProcAttr(cmd)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = os.Environ()
	return cmd
}

// startWithPipes starts the command with standard pipes.
func (r *Registry) startWithPipes(session *Session, cmd *exec.Cmd, buf *HeadTailBuffer) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	session.stdinPipe = stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout pipe

	if err := cmd.Start(); err != nil {
		return err
	}

	// Read stdout → buffer
	go func() {
		io.Copy(buf, stdout)
	}()

	return nil
}

// Get returns a session by ID.
func (r *Registry) Get(id string) (*Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	return s, ok
}

// List returns all sessions sorted by creation time (newest first).
func (r *Registry) List() []*Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Remove kills and removes a session.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	s, ok := r.sessions[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("session %s not found", id)
	}
	delete(r.sessions, id)
	r.mu.Unlock()

	s.Kill()
	return nil
}

// Cleanup prunes expired finished sessions and evicts LRU if over capacity.
func (r *Registry) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanupLocked()
}

func (r *Registry) cleanupLocked() {
	now := time.Now()

	// Remove expired finished sessions
	for id, s := range r.sessions {
		s.mu.Lock()
		finished := s.State != StateRunning && !s.FinishedAt.IsZero() && now.Sub(s.FinishedAt) > r.finishedTTL
		s.mu.Unlock()
		if finished {
			delete(r.sessions, id)
		}
	}

	// LRU eviction of finished sessions if still over capacity
	for len(r.sessions) >= r.maxSessions {
		var oldestID string
		var oldestPolled time.Time

		for id, s := range r.sessions {
			s.mu.Lock()
			isFinished := s.State != StateRunning
			polled := s.LastPolled
			if polled.IsZero() {
				polled = s.CreatedAt
			}
			s.mu.Unlock()

			if isFinished && (oldestID == "" || polled.Before(oldestPolled)) {
				oldestID = id
				oldestPolled = polled
			}
		}

		if oldestID == "" {
			break // all sessions are running, can't evict
		}
		delete(r.sessions, oldestID)
	}
}

// Shutdown kills all running sessions and clears the registry.
func (r *Registry) Shutdown() {
	r.mu.Lock()
	sessions := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	r.sessions = make(map[string]*Session)
	r.mu.Unlock()

	for _, s := range sessions {
		s.Kill()
	}
}
