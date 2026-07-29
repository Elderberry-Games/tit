// Package version carries build metadata injected at link time.
package version

// Version is the release version, overridden via -ldflags at build time.
var Version = "dev"

// Commit is the short git commit, overridden via -ldflags at build time.
var Commit = "unknown"

// String returns the human-readable version line.
func String() string {
	if Commit == "unknown" {
		return "tit " + Version
	}
	return "tit " + Version + " (" + Commit + ")"
}
