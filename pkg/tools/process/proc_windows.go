//go:build windows

package process

import "os/exec"

// setProcAttr is a no-op on Windows (no process group isolation).
func setProcAttr(cmd *exec.Cmd) {}

// killProcess terminates the process directly on Windows.
func killProcess(s *Session) error {
	cmd := s.cmd
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// startWithPTY is not supported on Windows; always returns false.
func startWithPTY(session *Session, cmd *exec.Cmd, buf *HeadTailBuffer) bool {
	return false
}
