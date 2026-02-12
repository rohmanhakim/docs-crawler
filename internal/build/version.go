package build

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// FullVersion returns the version string with commit hash appended.
// Format: "Version+Commit" (e.g., "1.0.0+abc123")
func FullVersion() string {
	return Version + "+" + Commit
}
