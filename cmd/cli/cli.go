package cli

import (
	"fmt"
	"os"
)

// Execute runs the CLI with all commands
func Execute() {
	rootCmd.SetVersionTemplate("genie version {{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}