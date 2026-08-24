package cli

import (
	"fmt"
	"log/slog"

	"github.com/kcaldas/genie/cmd/bootstrap"
	"github.com/kcaldas/genie/cmd/tui"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
	"github.com/kcaldas/genie/pkg/version"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	workingDir  string
	allowedDirs []string
	verbose     bool
	quiet       bool
	persona     string
	traceLevel  string

	// Genie instance - initialized once and reused
	genieInstance  genie.Genie
	initialSession genie.Session
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
			// Warnings and errors only: Genie starts (and logs
			// startup info like the context budget) before
			// subcommands install their own loggers, so the default
			// must keep info-level chatter off the terminal.
			logger = logging.NewLogger(logging.Config{
				Level:  slog.LevelWarn,
				Format: logging.FormatText,
			})
		}
		logging.SetGlobalLogger(logger)

		// The --trace flag is sugar over GENIE_SESSION_RECORDING; it must
		// land in the env before the Genie instance (and its recorder) is
		// constructed below. The flag wins over an inherited env value.
		if err := configureTraceEnv(traceLevel); err != nil {
			return err
		}

		// Initialize Genie once for all commands
		var err error
		genieInstance, err = bootstrap.Genie()
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

		var startOpts []genie.StartOption
		if len(allowedDirs) > 0 {
			startOpts = append(startOpts, genie.WithAllowedDirs(allowedDirs...))
		}

		initialSession, err = genieInstance.Start(workingDirPtr, personaPtr, startOpts...)
		if err != nil {
			return err // Return the original error without wrapping
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Release background sessions and MCP server subprocesses so
		// quitting Genie never orphans child processes.
		if genieInstance != nil {
			genieInstance.Shutdown()
		}
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
	RootCmd.PersistentFlags().StringArrayVar(&allowedDirs, "allow-dir", nil, "additional directory that file tools may access (repeatable)")
	RootCmd.PersistentFlags().StringVar(&persona, "persona", "", "persona to use (e.g., engineer, product_owner, persona_creator)")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output (debug level)")
	RootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (errors only)")
	RootCmd.PersistentFlags().StringVar(&traceLevel, "trace", "", "record the session under .genie/sessions (standard|full; bare --trace means full)")
	RootCmd.PersistentFlags().Lookup("trace").NoOptDefVal = "full"

	// Add CLI subcommands
	addCommands()
}

// addCommands adds all CLI subcommands to the root command
func addCommands() {
	// Add the ask command with access to the initialized Genie instance
	RootCmd.AddCommand(NewAskCommandWithGenie(func() (genie.Genie, genie.Session) {
		return genieInstance, initialSession
	}))

	// Session trace reader — boots without Genie.
	RootCmd.AddCommand(NewSessionsCommand())

	// Future commands can be added here:
	// RootCmd.AddCommand(NewIdeasCommand(...))
	// RootCmd.AddCommand(NewConfigCommand(...))
}
