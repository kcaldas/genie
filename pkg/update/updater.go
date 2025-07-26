package update

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/kcaldas/genie/pkg/version"
)

const (
	// GitHub repository for releases
	GitHubOwner = "kcaldas"
	GitHubRepo  = "genie"
)

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseNotes   string
	DownloadURL    string
	UpdateNeeded   bool
}

// ProgressCallback is called during download to report progress
type ProgressCallback func(current, total int64)

// Updater handles self-updating logic
type Updater struct {
	source     selfupdate.Source
	updater    *selfupdate.Updater
	repository selfupdate.Repository
}

// NewUpdater creates a new updater instance
func NewUpdater() (*Updater, error) {
	// Create GitHub source
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub source: %w", err)
	}

	// Create updater with checksum validation
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create updater: %w", err)
	}

	// Create repository
	repository := selfupdate.NewRepositorySlug(GitHubOwner, GitHubRepo)

	return &Updater{
		source:     source,
		updater:    updater,
		repository: repository,
	}, nil
}

// CheckForUpdates checks if there's a newer version available
func (u *Updater) CheckForUpdates(ctx context.Context) (*UpdateInfo, error) {
	currentVersion := version.GetVersion()

	// Find latest release
	latest, found, err := u.updater.DetectLatest(ctx, u.repository)
	if err != nil {
		return nil, fmt.Errorf("failed to detect latest version: %w", err)
	}

	if !found {
		return nil, fmt.Errorf("no releases found")
	}

	latestVersion := latest.Version()
	
	// Compare versions using semver
	// Handle development versions
	var updateNeeded bool
	if currentVersion == "dev" || currentVersion == "development" || currentVersion == "" {
		// Development versions always need update
		updateNeeded = true
	} else {
		current, err := semver.NewVersion(currentVersion)
		if err != nil {
			// If current version is not valid semver, assume it needs update
			updateNeeded = true
		} else {
			latestSemver, err := semver.NewVersion(latestVersion)
			if err != nil {
				return nil, fmt.Errorf("invalid latest version %s: %w", latestVersion, err)
			}
			updateNeeded = latestSemver.GreaterThan(current)
		}
	}

	updateInfo := &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		ReleaseNotes:   latest.ReleaseNotes,
		DownloadURL:    latest.AssetURL,
		UpdateNeeded:   updateNeeded,
	}

	return updateInfo, nil
}

// UpdateToLatest performs the actual update to the latest version
func (u *Updater) UpdateToLatest(ctx context.Context, progressCallback ProgressCallback) error {
	// For development versions, we need to detect latest and update to it
	latest, found, err := u.updater.DetectLatest(ctx, u.repository)
	if err != nil {
		return fmt.Errorf("failed to detect latest version: %w", err)
	}

	if !found {
		return fmt.Errorf("no releases found")
	}

	// Get executable path
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}

	// Perform update to the latest release
	err = u.updater.UpdateTo(ctx, latest, exe)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	return nil
}

// UpdateToVersion updates to a specific version
func (u *Updater) UpdateToVersion(ctx context.Context, targetVersion string, progressCallback ProgressCallback) error {
	// First find the release for the target version
	latest, found, err := u.updater.DetectLatest(ctx, u.repository)
	if err != nil {
		return fmt.Errorf("failed to detect releases: %w", err)
	}

	if !found {
		return fmt.Errorf("no releases found")
	}

	// For now, we'll only support updating to the latest version
	// TODO: Add support for specific version updates when the API supports it
	if latest.Version() != targetVersion {
		return fmt.Errorf("specific version updates not yet supported, latest available is %s", latest.Version())
	}

	// Get executable path
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}

	// Perform update to the release
	err = u.updater.UpdateTo(ctx, latest, exe)
	if err != nil {
		return fmt.Errorf("update to version %s failed: %w", targetVersion, err)
	}

	return nil
}

// GetLatestVersion gets the latest version without updating
func (u *Updater) GetLatestVersion(ctx context.Context) (string, error) {
	latest, found, err := u.updater.DetectLatest(ctx, u.repository)
	if err != nil {
		return "", fmt.Errorf("failed to detect latest version: %w", err)
	}

	if !found {
		return "", fmt.Errorf("no releases found")
	}

	return latest.Version(), nil
}

// UpdateOptions contains options for update operations
type UpdateOptions struct {
	Force            bool          // Force update even if no newer version
	TargetVersion    string        // Update to specific version (empty for latest)
	Timeout          time.Duration // Timeout for update operation
	ProgressCallback ProgressCallback // Callback for progress updates (currently unused)
}

// UpdateWithOptions performs update with specified options
func (u *Updater) UpdateWithOptions(ctx context.Context, opts UpdateOptions) (*UpdateInfo, error) {
	// Set timeout if specified
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Check for updates first
	updateInfo, err := u.CheckForUpdates(ctx)
	if err != nil {
		return nil, err
	}

	// If no update needed and not forcing, return current info
	if !updateInfo.UpdateNeeded && !opts.Force {
		return updateInfo, nil
	}

	// Perform update
	if opts.TargetVersion != "" {
		err = u.UpdateToVersion(ctx, opts.TargetVersion, opts.ProgressCallback)
	} else {
		err = u.UpdateToLatest(ctx, opts.ProgressCallback)
	}

	if err != nil {
		return updateInfo, err
	}

	return updateInfo, nil
}