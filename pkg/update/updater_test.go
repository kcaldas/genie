package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/kcaldas/genie/pkg/version"
)

// setVersion overrides the build-time version for the duration of a test.
func setVersion(t *testing.T, v string) {
	t.Helper()
	previous := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = previous })
}

// stubAsset implements selfupdate.SourceAsset.
type stubAsset struct {
	id   int64
	name string
}

func (a stubAsset) GetID() int64                  { return a.id }
func (a stubAsset) GetName() string               { return a.name }
func (a stubAsset) GetSize() int                  { return 1024 }
func (a stubAsset) GetBrowserDownloadURL() string { return "https://example.com/download/" + a.name }

// stubRelease implements selfupdate.SourceRelease.
type stubRelease struct {
	tag    string
	notes  string
	assets []selfupdate.SourceAsset
}

func (r stubRelease) GetID() int64                        { return 1 }
func (r stubRelease) GetTagName() string                  { return r.tag }
func (r stubRelease) GetDraft() bool                      { return false }
func (r stubRelease) GetPrerelease() bool                 { return false }
func (r stubRelease) GetPublishedAt() time.Time           { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
func (r stubRelease) GetReleaseNotes() string             { return r.notes }
func (r stubRelease) GetName() string                     { return r.tag }
func (r stubRelease) GetURL() string                      { return "https://example.com/releases/" + r.tag }
func (r stubRelease) GetAssets() []selfupdate.SourceAsset { return r.assets }

// stubSource implements selfupdate.Source without any network access.
type stubSource struct {
	releases        []selfupdate.SourceRelease
	listErr         error
	downloadCalls   atomic.Int32
	downloadPayload string
}

func (s *stubSource) ListReleases(ctx context.Context, repository selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.releases, nil
}

func (s *stubSource) DownloadReleaseAsset(ctx context.Context, rel *selfupdate.Release, assetID int64) (io.ReadCloser, error) {
	s.downloadCalls.Add(1)
	if s.downloadPayload == "" {
		return nil, errors.New("stub source: download not available")
	}
	return io.NopCloser(strings.NewReader(s.downloadPayload)), nil
}

// binaryAssetName builds an asset name matching the running OS/arch so
// go-selfupdate's detection accepts it.
func binaryAssetName(version string) string {
	return fmt.Sprintf("genie_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
}

// releaseWithAssets returns a release for the given version, carrying a
// platform-matching binary asset and the checksums file the validator needs.
func releaseWithAssets(version, notes string) stubRelease {
	return stubRelease{
		tag:   "v" + version,
		notes: notes,
		assets: []selfupdate.SourceAsset{
			stubAsset{id: 100, name: binaryAssetName(version)},
			stubAsset{id: 101, name: "checksums.txt"},
		},
	}
}

// newTestUpdater builds an Updater backed by the given stub source.
func newTestUpdater(t *testing.T, source *stubSource) *Updater {
	t.Helper()
	updater, err := NewUpdaterWithSource(source, selfupdate.NewRepositorySlug(GitHubOwner, GitHubRepo))
	if err != nil {
		t.Fatalf("NewUpdaterWithSource returned error: %v", err)
	}
	return updater
}

func TestNewUpdaterConstructsDefaultGitHubSource(t *testing.T) {
	updater, err := NewUpdater()
	if err != nil {
		t.Fatalf("NewUpdater returned error: %v", err)
	}
	if updater.source == nil || updater.updater == nil || updater.repository == nil {
		t.Error("NewUpdater should populate source, updater, and repository")
	}
}

func TestCheckForUpdatesDecision(t *testing.T) {
	const latest = "1.3.0"

	tests := []struct {
		name           string
		currentVersion string
		wantNeeded     bool
	}{
		{"dev version always updates", "dev", true},
		{"development version always updates", "development", true},
		{"empty version always updates", "", true},
		{"older version updates", "1.0.0", true},
		{"same version is up to date", "1.3.0", false},
		{"newer version is up to date", "2.0.0", false},
		{"non-semver current version updates", "not-a-version", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setVersion(t, tt.currentVersion)
			updater := newTestUpdater(t, &stubSource{
				releases: []selfupdate.SourceRelease{releaseWithAssets(latest, "notes")},
			})

			info, err := updater.CheckForUpdates(context.Background())
			if err != nil {
				t.Fatalf("CheckForUpdates returned error: %v", err)
			}

			if info.UpdateNeeded != tt.wantNeeded {
				t.Errorf("UpdateNeeded = %v, want %v", info.UpdateNeeded, tt.wantNeeded)
			}
			if info.CurrentVersion != tt.currentVersion {
				t.Errorf("CurrentVersion = %q, want %q", info.CurrentVersion, tt.currentVersion)
			}
			if info.LatestVersion != latest {
				t.Errorf("LatestVersion = %q, want %q", info.LatestVersion, latest)
			}
		})
	}
}

func TestCheckForUpdatesPopulatesReleaseDetails(t *testing.T) {
	setVersion(t, "1.0.0")
	release := releaseWithAssets("1.3.0", "Bug fixes and improvements")
	updater := newTestUpdater(t, &stubSource{releases: []selfupdate.SourceRelease{release}})

	info, err := updater.CheckForUpdates(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}

	if info.ReleaseNotes != "Bug fixes and improvements" {
		t.Errorf("ReleaseNotes = %q, want release notes from source", info.ReleaseNotes)
	}
	wantURL := "https://example.com/download/" + binaryAssetName("1.3.0")
	if info.DownloadURL != wantURL {
		t.Errorf("DownloadURL = %q, want %q", info.DownloadURL, wantURL)
	}
}

func TestCheckForUpdatesPicksHighestVersion(t *testing.T) {
	setVersion(t, "1.2.0")
	updater := newTestUpdater(t, &stubSource{
		releases: []selfupdate.SourceRelease{
			releaseWithAssets("1.1.0", "old"),
			releaseWithAssets("1.3.0", "newest"),
			releaseWithAssets("1.2.5", "middle"),
		},
	})

	info, err := updater.CheckForUpdates(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}
	if info.LatestVersion != "1.3.0" {
		t.Errorf("LatestVersion = %q, want %q", info.LatestVersion, "1.3.0")
	}
	if !info.UpdateNeeded {
		t.Error("UpdateNeeded = false, want true for 1.2.0 -> 1.3.0")
	}
}

func TestCheckForUpdatesSourceErrorPropagates(t *testing.T) {
	setVersion(t, "1.0.0")
	sourceErr := errors.New("github is down")
	updater := newTestUpdater(t, &stubSource{listErr: sourceErr})

	_, err := updater.CheckForUpdates(context.Background())
	if err == nil {
		t.Fatal("expected error from failing source, got nil")
	}
	if !errors.Is(err, sourceErr) {
		t.Errorf("error should wrap the source error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "failed to detect latest version") {
		t.Errorf("error should describe the failing step, got: %v", err)
	}
}

func TestCheckForUpdatesNoReleases(t *testing.T) {
	setVersion(t, "1.0.0")
	updater := newTestUpdater(t, &stubSource{})

	_, err := updater.CheckForUpdates(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no releases found") {
		t.Errorf("expected 'no releases found' error, got: %v", err)
	}
}

func TestGetLatestVersion(t *testing.T) {
	updater := newTestUpdater(t, &stubSource{
		releases: []selfupdate.SourceRelease{releaseWithAssets("2.1.0", "")},
	})

	latest, err := updater.GetLatestVersion(context.Background())
	if err != nil {
		t.Fatalf("GetLatestVersion returned error: %v", err)
	}
	if latest != "2.1.0" {
		t.Errorf("latest = %q, want %q", latest, "2.1.0")
	}
}

func TestGetLatestVersionErrors(t *testing.T) {
	t.Run("source failure", func(t *testing.T) {
		updater := newTestUpdater(t, &stubSource{listErr: errors.New("boom")})
		if _, err := updater.GetLatestVersion(context.Background()); err == nil {
			t.Error("expected error from failing source, got nil")
		}
	})

	t.Run("no releases", func(t *testing.T) {
		updater := newTestUpdater(t, &stubSource{})
		_, err := updater.GetLatestVersion(context.Background())
		if err == nil || !strings.Contains(err.Error(), "no releases found") {
			t.Errorf("expected 'no releases found' error, got: %v", err)
		}
	})
}

func TestUpdateWithOptionsShortCircuitsWhenUpToDate(t *testing.T) {
	setVersion(t, "1.3.0")
	source := &stubSource{releases: []selfupdate.SourceRelease{releaseWithAssets("1.3.0", "")}}
	updater := newTestUpdater(t, source)

	// Timeout exercises the context-with-timeout branch
	info, err := updater.UpdateWithOptions(context.Background(), UpdateOptions{Timeout: time.Minute})
	if err != nil {
		t.Fatalf("UpdateWithOptions returned error: %v", err)
	}
	if info.UpdateNeeded {
		t.Error("UpdateNeeded = true, want false when already on latest")
	}
	if calls := source.downloadCalls.Load(); calls != 0 {
		t.Errorf("download called %d times, want 0 (must short-circuit)", calls)
	}
}

func TestUpdateWithOptionsPropagatesCheckError(t *testing.T) {
	setVersion(t, "1.0.0")
	sourceErr := errors.New("network unreachable")
	updater := newTestUpdater(t, &stubSource{listErr: sourceErr})

	_, err := updater.UpdateWithOptions(context.Background(), UpdateOptions{})
	if !errors.Is(err, sourceErr) {
		t.Errorf("error should wrap source error, got: %v", err)
	}
}

func TestUpdateWithOptionsPropagatesDownloadFailure(t *testing.T) {
	setVersion(t, "1.0.0")
	source := &stubSource{releases: []selfupdate.SourceRelease{releaseWithAssets("1.3.0", "")}}
	updater := newTestUpdater(t, source)

	info, err := updater.UpdateWithOptions(context.Background(), UpdateOptions{})
	if err == nil {
		t.Fatal("expected error when the source cannot serve the asset, got nil")
	}
	if !strings.Contains(err.Error(), "update failed") {
		t.Errorf("error should come from the update step, got: %v", err)
	}
	// The failed attempt still reports what it was trying to install
	if info == nil || info.LatestVersion != "1.3.0" {
		t.Errorf("UpdateInfo should describe the attempted update, got: %+v", info)
	}
	if calls := source.downloadCalls.Load(); calls == 0 {
		t.Error("expected the update path to reach the source download")
	}
}

func TestUpdateWithOptionsForcedTargetVersionRoutesToUpdateToVersion(t *testing.T) {
	setVersion(t, "1.3.0")
	source := &stubSource{releases: []selfupdate.SourceRelease{releaseWithAssets("1.3.0", "")}}
	updater := newTestUpdater(t, source)

	// Already up to date, but Force plus a non-latest TargetVersion must hit
	// the UpdateToVersion path and surface its limitation error.
	_, err := updater.UpdateWithOptions(context.Background(), UpdateOptions{
		Force:         true,
		TargetVersion: "0.9.0",
	})
	if err == nil || !strings.Contains(err.Error(), "specific version updates not yet supported") {
		t.Errorf("expected UpdateToVersion limitation error, got: %v", err)
	}
}

func TestUpdateToVersionRejectsNonLatestTarget(t *testing.T) {
	updater := newTestUpdater(t, &stubSource{
		releases: []selfupdate.SourceRelease{releaseWithAssets("1.3.0", "")},
	})

	err := updater.UpdateToVersion(context.Background(), "0.9.0", nil)
	if err == nil {
		t.Fatal("expected error for non-latest target version, got nil")
	}
	if !strings.Contains(err.Error(), "specific version updates not yet supported") {
		t.Errorf("error should explain the limitation, got: %v", err)
	}
	if !strings.Contains(err.Error(), "1.3.0") {
		t.Errorf("error should mention the latest available version, got: %v", err)
	}
}

func TestUpdateToVersionSourceErrorPropagates(t *testing.T) {
	sourceErr := errors.New("listing failed")
	updater := newTestUpdater(t, &stubSource{listErr: sourceErr})

	err := updater.UpdateToVersion(context.Background(), "1.3.0", nil)
	if !errors.Is(err, sourceErr) {
		t.Errorf("error should wrap source error, got: %v", err)
	}
}
