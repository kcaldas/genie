//go:build !windows

package process

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserShell_UsesEnvWhenTrusted(t *testing.T) {
	// /bin/sh is in /etc/shells on every Unix system.
	t.Setenv("SHELL", "/bin/sh")
	assert.Equal(t, "/bin/sh", UserShell())
}

func TestUserShell_FallsBackWhenUntrusted(t *testing.T) {
	t.Setenv("SHELL", "/tmp/evil")
	assert.Equal(t, fallbackShell, UserShell())
}

func TestUserShell_FallsBackWhenUnset(t *testing.T) {
	t.Setenv("SHELL", "")
	assert.Equal(t, fallbackShell, UserShell())
}

func TestIsTrustedShell(t *testing.T) {
	// /bin/sh is universally present in /etc/shells.
	assert.True(t, isTrustedShell("/bin/sh"))
	assert.False(t, isTrustedShell("/tmp/evil"))
	assert.False(t, isTrustedShell(""))
}

func TestIsTrustedShell_MissingFile(t *testing.T) {
	// If /etc/shells doesn't exist, nothing is trusted.
	// We can't easily test this without mocking, but verify the function
	// doesn't panic with a valid path.
	if _, err := os.Stat("/etc/shells"); err != nil {
		assert.False(t, isTrustedShell("/bin/sh"))
	}
}
