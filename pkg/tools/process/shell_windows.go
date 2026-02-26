//go:build windows

package process

// UserShell returns "bash" on Windows (expects Git Bash in PATH).
func UserShell() string {
	return "bash"
}
