package version

import (
	"fmt"
	"runtime"
	"time"
)

// Version information set at build time via ldflags
var (
	// Version from git tag (e.g., "v0.10.0")
	Version = "dev"
	
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	
	// GitTag is the git tag if on a tag
	GitTag = ""
	
	// BuildDate is when the binary was built
	BuildDate = "unknown"
	
	// BuildNumber is auto-incremented build number
	BuildNumber = "0"
	
	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()
	
	// Platform is the target platform
	Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// GetVersion returns the full version string
func GetVersion() string {
	if Version == "dev" {
		// Development version - use git info
		if GitTag != "" {
			Version = GitTag
		} else if GitCommit != "unknown" && len(GitCommit) >= 7 {
			Version = fmt.Sprintf("dev-%s", GitCommit[:7])
		}
	}
	
	// Add build number if not zero
	if BuildNumber != "0" {
		return fmt.Sprintf("%s+%s", Version, BuildNumber)
	}
	
	return Version
}

// GetFullVersion returns detailed version information
func GetFullVersion() string {
	return fmt.Sprintf(`MinZ Compiler %s
Build:    #%s
Commit:   %s
Date:     %s
Go:       %s
Platform: %s`,
		GetVersion(),
		BuildNumber,
		GitCommit,
		BuildDate,
		GoVersion,
		Platform)
}

// GetBuildInfo returns a single-line build info string
func GetBuildInfo() string {
	return fmt.Sprintf("MinZ %s (%s, built %s)", GetVersion(), GitCommit[:7], BuildDate)
}

// SetBuildTime sets the build date to current time if not already set
func init() {
	if BuildDate == "unknown" {
		BuildDate = time.Now().Format("2006-01-02T15:04:05Z")
	}
}