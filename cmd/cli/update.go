package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/kcaldas/genie/pkg/update"
	"github.com/kcaldas/genie/pkg/version"
	"github.com/spf13/cobra"
)

var (
	checkOnly     bool
	forceUpdate   bool
	targetVersion string
	timeout       time.Duration
)

// newUpdateCommand creates the update command
func newUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update genie to the latest version",
		Long: `Update genie to the latest version from GitHub releases.

Examples:
  genie update                    # Update to latest version
  genie update --check            # Check for updates without updating
  genie update --version v1.2.3   # Update to specific version
  genie update --force            # Force update even if same version`,
		RunE: runUpdateCommand,
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check for updates without updating")
	cmd.Flags().BoolVar(&forceUpdate, "force", false, "Force update even if current version is latest")
	cmd.Flags().StringVar(&targetVersion, "version", "", "Update to specific version")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Timeout for update operation")

	return cmd
}

func runUpdateCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Create updater
	updater, err := update.NewUpdater()
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	// If just checking for updates
	if checkOnly {
		return checkForUpdates(ctx, updater)
	}

	// Perform update
	return performUpdate(ctx, updater)
}

func checkForUpdates(ctx context.Context, updater *update.Updater) error {
	fmt.Printf("Current version: %s\n", version.GetVersion())
	fmt.Println("Checking for updates...")

	updateInfo, err := updater.CheckForUpdates(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	fmt.Printf("Latest version: %s\n", updateInfo.LatestVersion)

	if updateInfo.UpdateNeeded {
		fmt.Printf("🎉 A new version is available!\n")
		fmt.Printf("Current: %s → Latest: %s\n", updateInfo.CurrentVersion, updateInfo.LatestVersion)
		if updateInfo.ReleaseNotes != "" {
			fmt.Printf("\nRelease Notes:\n%s\n", updateInfo.ReleaseNotes)
		}
		fmt.Printf("\nRun 'genie update' to update to the latest version.\n")
	} else {
		fmt.Printf("✅ You are already using the latest version.\n")
	}

	return nil
}

func performUpdate(ctx context.Context, updater *update.Updater) error {
	fmt.Printf("Current version: %s\n", version.GetVersion())

	// Check for updates first (unless forcing)
	if !forceUpdate {
		updateInfo, err := updater.CheckForUpdates(ctx)
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		if !updateInfo.UpdateNeeded {
			fmt.Printf("✅ You are already using the latest version (%s).\n", updateInfo.LatestVersion)
			fmt.Println("Use --force to reinstall the current version.")
			return nil
		}

		fmt.Printf("🔄 Updating from %s to %s...\n", updateInfo.CurrentVersion, updateInfo.LatestVersion)
		if updateInfo.ReleaseNotes != "" {
			fmt.Printf("\nRelease Notes:\n%s\n\n", updateInfo.ReleaseNotes)
		}
	} else {
		fmt.Println("🔄 Force updating...")
	}

	// Create progress callback
	var lastPercent int
	progressCallback := func(current, total int64) {
		if total > 0 {
			percent := int((current * 100) / total)
			if percent != lastPercent && percent%10 == 0 {
				fmt.Printf("📥 Downloaded %d%%\n", percent)
				lastPercent = percent
			}
		}
	}

	// Perform update with options
	opts := update.UpdateOptions{
		Force:            forceUpdate,
		TargetVersion:    targetVersion,
		Timeout:          timeout,
		ProgressCallback: progressCallback,
	}

	updateInfo, err := updater.UpdateWithOptions(ctx, opts)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	if targetVersion != "" {
		fmt.Printf("✅ Successfully updated to version %s!\n", targetVersion)
	} else {
		fmt.Printf("✅ Successfully updated to version %s!\n", updateInfo.LatestVersion)
	}

	fmt.Println("\n🚀 Restart genie to use the new version.")
	return nil
}

func init() {
	// Add update command to root
	RootCmd.AddCommand(newUpdateCommand())
}