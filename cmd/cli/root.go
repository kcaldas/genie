package cli

import (
	"fmt"

	"github.com/kcaldas/genie/cmd/tui"
	"github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	workingDir string
	verbose    bool
	quiet      bool
	
	// Genie instance - initialized once and reused
	genieInstance  genie.Genie
	initialSession *genie.Session
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "genie",
	Short: "Genie AI coding assistant",
	Long:  `Genie is an AI coding assistant that helps with software engineering tasks.`,
	Version: "dev", // This could be passed in or read from build info
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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

		// Initialize Genie once for all commands
		var err error
		genieInstance, err = di.ProvideGenie()
		if err != nil {
			return fmt.Errorf("failed to initialize Genie: %w", err)
		}

		// Start Genie with working directory
		var workingDirPtr *string
		if workingDir != "" {
			workingDirPtr = &workingDir
		}

		initialSession, err = genieInstance.Start(workingDirPtr)
		if err != nil {
			return fmt.Errorf("failed to start Genie: %w", err)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// No subcommand provided - start REPL (TUI mode)
		tui.StartREPL(genieInstance, initialSession)
		return nil
	},
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().StringVar(&workingDir, "cwd", "", "working directory for Genie operations")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output (debug level)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (errors only)")

	// Add CLI subcommands
	addCommands()
}

// addCommands adds all CLI subcommands to the root command
func addCommands() {
	// Add the ask command with access to the initialized Genie instance
	rootCmd.AddCommand(NewAskCommandWithGenie(func() (genie.Genie, *genie.Session) {
		return genieInstance, initialSession
	}))
	
	// Future commands can be added here:
	// rootCmd.AddCommand(NewIdeasCommand(...))
	// rootCmd.AddCommand(NewConfigCommand(...))
}