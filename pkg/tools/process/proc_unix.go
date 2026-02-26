//go:build !windows

package process

import (
	"io"
	"log"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// setProcAttr configures process group isolation on Unix.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcess terminates the process group: SIGTERM first, then SIGKILL after 5s.
func killProcess(s *Session) error {
	cmd := s.cmd
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Kill()
	}

	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	select {
	case <-s.done:
		return nil
	case <-time.After(5 * time.Second):
	}

	_ = syscall.Kill(-pgid, syscall.SIGKILL)

	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
	}

	return nil
}

// startWithPTY tries to start the command with a PTY. Returns false if PTY
// allocation fails (caller should fall back to pipes).
func startWithPTY(session *Session, cmd *exec.Cmd, buf *HeadTailBuffer) bool {
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		log.Printf("PTY allocation failed, falling back to pipes: %v", err)
		return false
	}

	session.ptyFile = ptmx

	// Read PTY output → buffer
	go func() {
		io.Copy(buf, ptmx)
	}()

	return true
}
