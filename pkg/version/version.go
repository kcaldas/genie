package version

import (
	"fmt"
	"runtime"
)

var (
	// Build information - these will be set via ldflags during build
	Version   = "dev"
	Commit    = "unknown"
	Date      = "unknown"
	BuiltBy   = "unknown"
	GoVersion = runtime.Version()
)

// Info holds version information
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	BuiltBy   string `json:"built_by"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetInfo returns version information
func GetInfo() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		BuiltBy:   BuiltBy,
		GoVersion: GoVersion,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// GetVersion returns just the version string
func GetVersion() string {
	return Version
}

// String returns a formatted version string
func (i Info) String() string {
	return fmt.Sprintf("genie version %s\ncommit: %s\nbuilt: %s\nby: %s\ngo: %s\nplatform: %s",
		i.Version, i.Commit, i.Date, i.BuiltBy, i.GoVersion, i.Platform)
}

// ShortString returns a short version string
func (i Info) ShortString() string {
	return fmt.Sprintf("genie version %s", i.Version)
}