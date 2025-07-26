package cli

import (
	"fmt"

	"github.com/kcaldas/genie/cmd/tui"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/version"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	workingDir string
	verbose    bool
	quiet      bool
	persona    string

	// Genie instance - initialized once and reused
	genieInstance  genie.Genie
	initialSession *genie.Session
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "genie",
	Short:   "Genie AI coding assistant",
	Long:    `Genie is an AI coding assistant that helps with software engineering tasks.`,
	Version: version.GetVersion(),
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
		genieInstance, err = tui.ProvideGenie()
		if err != nil {
			return fmt.Errorf("failed to initialize Genie: %w", err)
		}

		// Start Genie with working directory and persona
		var workingDirPtr *string
		if workingDir != "" {
			workingDirPtr = &workingDir
		}

		var personaPtr *string
		if persona != "" {
			personaPtr = &persona
		}

		initialSession, err = genieInstance.Start(workingDirPtr, personaPtr)
		if err != nil {
			return err // Return the original error without wrapping
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check for stdin input before starting TUI
		var stdinContent string
		if hasStdinInput() {
			content, err := readStdinInput()
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			stdinContent = content
		}

		// No subcommand provided - start TUI mode
		tuiApp, err := tui.InjectTUI(initialSession)
		if err != nil {
			return err
		}
		defer tuiApp.Stop()
		
		// Start the TUI with the initial message if provided
		return tuiApp.StartWithMessage(stdinContent)
	},
}

func init() {
	// Global flags available to all commands
	RootCmd.PersistentFlags().StringVar(&workingDir, "cwd", "", "working directory for Genie operations")
	RootCmd.PersistentFlags().StringVar(&persona, "persona", "", "persona to use (e.g., engineer, product_owner, persona_creator)")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output (debug level)")
	RootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (errors only)")

	// Add CLI subcommands
	addCommands()
}

// addCommands adds all CLI subcommands to the root command
func addCommands() {
	// Add the ask command with access to the initialized Genie instance
	RootCmd.AddCommand(NewAskCommandWithGenie(func() (genie.Genie, *genie.Session) {
		return genieInstance, initialSession
	}))

	// Future commands can be added here:
	// RootCmd.AddCommand(NewIdeasCommand(...))
	// RootCmd.AddCommand(NewConfigCommand(...))
}

