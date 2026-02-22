package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// State represents the lifecycle state of a session.
type State string

const (
	StateRunning State = "running"
	StateExited  State = "exited"
	StateFailed  State = "failed"
)

// Session represents a running or finished process with its output buffer.
type Session struct {
	ID         string
	Command    string
	CWD        string
	State      State
	ExitCode   int
	Buffer     *HeadTailBuffer
	CreatedAt  time.Time
	FinishedAt time.Time
	LastPolled time.Time

	ptyFile   *os.File
	stdinPipe io.WriteCloser
	cmd       *exec.Cmd
	mu        sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
}

// Write sends raw data to the process stdin (or PTY).
func (s *Session) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != StateRunning {
		return fmt.Errorf("session %s is not running (state: %s)", s.ID, s.State)
	}

	if s.ptyFile != nil {
		_, err := s.ptyFile.Write(data)
		return err
	}

	if s.stdinPipe != nil {
		_, err := s.stdinPipe.Write(data)
		return err
	}

	return fmt.Errorf("no writable fd (session has neither PTY nor stdin pipe)")
}

// SendKeys encodes tmux-style key names and writes them to the process.
func (s *Session) SendKeys(keys []string) error {
	data, err := EncodeKeys(keys)
	if err != nil {
		return fmt.Errorf("key encoding failed: %w", err)
	}
	return s.Write(data)
}

// Kill terminates the process group: SIGTERM first, then SIGKILL after 5s.
func (s *Session) Kill() error {
	s.mu.Lock()
	if s.State != StateRunning {
		s.mu.Unlock()
		return nil // already done
	}
	cmd := s.cmd
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		// Fallback: kill just the process
		return cmd.Process.Kill()
	}

	// SIGTERM to process group
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	// Wait up to 5s for graceful exit
	select {
	case <-s.done:
		return nil
	case <-time.After(5 * time.Second):
	}

	// SIGKILL if still running
	_ = syscall.Kill(-pgid, syscall.SIGKILL)

	// Wait for process to actually finish
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
	}

	return nil
}

// IsRunning returns true if the process is still running.
func (s *Session) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State == StateRunning
}

// Wait blocks until the process exits.
func (s *Session) Wait() {
	<-s.done
}

// Done returns a channel that is closed when the process exits.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// GetState returns the current state and exit code under lock.
func (s *Session) GetState() (State, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State, s.ExitCode
}

// SetLastPolled updates the last polled time under lock.
func (s *Session) SetLastPolled(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastPolled = t
}

// finish is called when the process exits. Must be called exactly once.
func (s *Session) finish(exitCode int, state State) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ExitCode = exitCode
	s.State = state
	s.FinishedAt = time.Now()

	if s.ptyFile != nil {
		s.ptyFile.Close()
	}
	if s.stdinPipe != nil {
		s.stdinPipe.Close()
	}
}
