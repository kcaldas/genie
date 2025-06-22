package cli

import (
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	quiet   bool
)

var rootCmd = &cobra.Command{
	Use:     "genie",
	Short:   "Genie CLI tool",
	Version: "dev", // This could be passed in or read from build info
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

	// Add all CLI subcommands
	rootCmd.AddCommand(NewAskCommand())
	// Future commands can be added here:
	// rootCmd.AddCommand(NewIdeasCommand())
	// rootCmd.AddCommand(NewConfigCommand())
}

// Execute runs the CLI with all commands
func Execute() {
	rootCmd.SetVersionTemplate("genie version {{.Version}}\n")
	rootCmd.Execute()
}