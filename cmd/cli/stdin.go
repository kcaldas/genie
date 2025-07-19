package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// hasStdinInput checks if data is available from stdin (pipe or redirect)
func hasStdinInput() bool {
	return !isatty.IsTerminal(os.Stdin.Fd())
}

// readStdinInput reads all available input from stdin
func readStdinInput() (string, error) {
	var content strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}
	
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}
	
	// Remove trailing newline
	result := content.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	
	return result, nil
}