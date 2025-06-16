package main

import (
	"github.com/spf13/cobra"
	"github.com/kcaldas/genie/pkg/logging"
)

var version = "dev"

var (
	verbose bool
	quiet   bool
)

var rootCmd = &cobra.Command{
	Use:     "genie",
	Short:   "Genie CLI tool",
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Configure logger based on flags
		var logger logging.Logger
		if quiet {
			logger = logging.NewQuietLogger()
		} else if verbose {
			logger = logging.NewVerboseLogger()
		} else {
			logger = logging.NewDefaultLogger()
		}
		logging.SetGlobalLogger(logger)
	},
}

func init() {
	// Add persistent flags for logging
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output (debug level)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (errors only)")
}

func main() {
	rootCmd.SetVersionTemplate("genie version {{.Version}}\n")
	rootCmd.Execute()
}