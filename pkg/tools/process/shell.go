package process

import (
	"bufio"
	"os"
	"strings"
)

const fallbackShell = "bash"

// UserShell returns the user's shell from $SHELL, validated against
// /etc/shells. Falls back to "bash" if $SHELL is unset or untrusted.
func UserShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return fallbackShell
	}
	if !isTrustedShell(shell) {
		return fallbackShell
	}
	return shell
}

// isTrustedShell checks whether the given path appears in /etc/shells.
func isTrustedShell(shell string) bool {
	f, err := os.Open("/etc/shells")
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == shell {
			return true
		}
	}
	return false
}
