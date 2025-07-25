package shell

import (
	"testing"
)

// TestShellInterface ensures the Shell interface can be implemented.
func TestShellInterface(t *testing.T) {
	// Test that BasicShell implements the Shell interface
	// This is a compile-time check to ensure interface compliance
	var _ Shell = (*BasicShell)(nil)

	// Test Command struct instantiation
	cmd := &Command{
		Text: "/test arg1",
	}
	if cmd.Text != "/test arg1" {
		t.Errorf("Command Text mismatch: got %s, want %s", cmd.Text, "/test arg1")
	}
}
