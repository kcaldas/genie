package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/update"
	"github.com/kcaldas/genie/pkg/version"
)

// UpdateCommand handles update-related operations in the TUI
type UpdateCommand struct {
	notification types.Notification
}

// NewUpdateCommand creates a new update command
func NewUpdateCommand(notification types.Notification) *UpdateCommand {
	return &UpdateCommand{
		notification: notification,
	}
}

// GetName returns the command name
func (c *UpdateCommand) GetName() string {
	return "update"
}

// GetDescription returns the command description
func (c *UpdateCommand) GetDescription() string {
	return "Check for or perform genie updates"
}

// GetUsage returns the command usage
func (c *UpdateCommand) GetUsage() string {
	return "/update [check|now|version <version>]"
}

// GetExamples returns command examples
func (c *UpdateCommand) GetExamples() []string {
	return []string{
		"/update check - Check for available updates",
		"/update now - Update to latest version",
		"/update version v1.2.3 - Update to specific version",
		"/update force - Force reinstall current version",
	}
}

// GetAliases returns command aliases
func (c *UpdateCommand) GetAliases() []string {
	return []string{"upgrade"}
}

// GetCategory returns the command category
func (c *UpdateCommand) GetCategory() string {
	return "System"
}

// IsHidden returns whether the command is hidden
func (c *UpdateCommand) IsHidden() bool {
	return false
}

// GetShortcuts returns keyboard shortcuts for the command
func (c *UpdateCommand) GetShortcuts() []string {
	return []string{} // No keyboard shortcuts for update command
}

// Execute executes the update command
func (c *UpdateCommand) Execute(args []string) error {
	if len(args) == 0 {
		return c.showUpdateHelp()
	}

	subcommand := args[0]
	switch subcommand {
	case "check":
		return c.checkForUpdates()
	case "now":
		return c.performUpdate(false, "")
	case "version":
		if len(args) < 2 {
			c.notification.AddSystemMessage("❌ Please specify a version: /update version v1.2.3")
			return nil
		}
		return c.performUpdate(false, args[1])
	case "force":
		return c.performUpdate(true, "")
	default:
		return c.showUpdateHelp()
	}
}

func (c *UpdateCommand) showUpdateHelp() error {
	help := `Update Commands:
/update check          - Check for available updates
/update now           - Update to latest version
/update version <ver> - Update to specific version (e.g., v1.2.3)
/update force         - Force reinstall current version

Current version: ` + version.GetVersion()

	c.notification.AddSystemMessage(help)
	return nil
}

func (c *UpdateCommand) checkForUpdates() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.notification.AddSystemMessage("🔍 Checking for updates...")

	updater, err := update.NewUpdater()
	if err != nil {
		c.notification.AddSystemMessage(fmt.Sprintf("❌ Failed to create updater: %v", err))
		return nil
	}

	updateInfo, err := updater.CheckForUpdates(ctx)
	if err != nil {
		c.notification.AddSystemMessage(fmt.Sprintf("❌ Failed to check for updates: %v", err))
		return nil
	}

	if updateInfo.UpdateNeeded {
		msg := fmt.Sprintf("🎉 Update available!\nCurrent: %s → Latest: %s\n\nUse '/update now' to update.",
			updateInfo.CurrentVersion, updateInfo.LatestVersion)
		
		if updateInfo.ReleaseNotes != "" {
			// Limit release notes to prevent overwhelming the chat
			notes := updateInfo.ReleaseNotes
			if len(notes) > 500 {
				notes = notes[:500] + "..."
			}
			msg += fmt.Sprintf("\n\nRelease Notes:\n%s", notes)
		}
		
		c.notification.AddSystemMessage(msg)
	} else {
		c.notification.AddSystemMessage(fmt.Sprintf("✅ You're using the latest version (%s)", updateInfo.LatestVersion))
	}

	return nil
}

func (c *UpdateCommand) performUpdate(force bool, targetVersion string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Show initial message
	if force {
		c.notification.AddSystemMessage("🔄 Force updating genie...")
	} else if targetVersion != "" {
		c.notification.AddSystemMessage(fmt.Sprintf("🔄 Updating to version %s...", targetVersion))
	} else {
		c.notification.AddSystemMessage("🔄 Updating to latest version...")
	}

	updater, err := update.NewUpdater()
	if err != nil {
		c.notification.AddSystemMessage(fmt.Sprintf("❌ Failed to create updater: %v", err))
		return nil
	}

	// Check for updates first (unless forcing)
	if !force && targetVersion == "" {
		updateInfo, err := updater.CheckForUpdates(ctx)
		if err != nil {
			c.notification.AddSystemMessage(fmt.Sprintf("❌ Failed to check for updates: %v", err))
			return nil
		}

		if !updateInfo.UpdateNeeded {
			c.notification.AddSystemMessage(fmt.Sprintf("✅ Already using latest version (%s). Use '/update force' to reinstall.", updateInfo.LatestVersion))
			return nil
		}

		if updateInfo.ReleaseNotes != "" {
			// Show brief release notes
			notes := updateInfo.ReleaseNotes
			if len(notes) > 300 {
				notes = notes[:300] + "..."
			}
			c.notification.AddSystemMessage(fmt.Sprintf("📝 Release Notes:\n%s", notes))
		}
	}

	// Progress tracking
	lastPercent := -1
	progressCallback := func(current, total int64) {
		if total > 0 {
			percent := int((current * 100) / total)
			// Only show every 25% to avoid spam
			if percent != lastPercent && (percent == 25 || percent == 50 || percent == 75 || percent == 100) {
				c.notification.AddSystemMessage(fmt.Sprintf("📥 Download progress: %d%%", percent))
				lastPercent = percent
			}
		}
	}

	// Perform update
	opts := update.UpdateOptions{
		Force:            force,
		TargetVersion:    targetVersion,
		Timeout:          5 * time.Minute,
		ProgressCallback: progressCallback,
	}

	updateInfo, err := updater.UpdateWithOptions(ctx, opts)
	if err != nil {
		c.notification.AddSystemMessage(fmt.Sprintf("❌ Update failed: %v", err))
		return nil
	}

	// Success message
	if targetVersion != "" {
		c.notification.AddSystemMessage(fmt.Sprintf("✅ Successfully updated to version %s!", targetVersion))
	} else {
		c.notification.AddSystemMessage(fmt.Sprintf("✅ Successfully updated to version %s!", updateInfo.LatestVersion))
	}

	c.notification.AddSystemMessage("🚀 Please restart genie to use the new version.")
	c.notification.AddSystemMessage("💡 You can use Ctrl+C to exit and restart.")

	return nil
}

// GetSubcommands returns available subcommands
func (c *UpdateCommand) GetSubcommands() []string {
	return []string{"check", "now", "version", "force"}
}

// GetSubcommandDescription returns description for a subcommand
func (c *UpdateCommand) GetSubcommandDescription(subcommand string) string {
	switch subcommand {
	case "check":
		return "Check for available updates without updating"
	case "now":
		return "Update to the latest version"
	case "version":
		return "Update to a specific version"
	case "force":
		return "Force reinstall current version"
	default:
		return ""
	}
}

// SupportsAutoComplete returns true if the command supports autocompletion
func (c *UpdateCommand) SupportsAutoComplete() bool {
	return true
}

// AutoComplete provides autocompletion suggestions
func (c *UpdateCommand) AutoComplete(args []string) []string {
	if len(args) == 0 {
		return c.GetSubcommands()
	}

	if len(args) == 1 {
		subcommands := c.GetSubcommands()
		prefix := strings.ToLower(args[0])
		var matches []string
		for _, cmd := range subcommands {
			if strings.HasPrefix(strings.ToLower(cmd), prefix) {
				matches = append(matches, cmd)
			}
		}
		return matches
	}

	return nil
}