package version

import "fmt"

// Set at build time via -ldflags (see deploy/Dockerfile and .github/workflows/release.yml).
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// String returns a human-readable build identity.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildTime)
}

// Map returns version fields for JSON APIs.
func Map() map[string]string {
	return map[string]string{
		"version":   Version,
		"commit":    Commit,
		"buildTime": BuildTime,
	}
}
