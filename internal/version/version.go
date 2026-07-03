package version

// Both values are overwritten at build time via
// -ldflags "-X niecke-it.de/uptime/internal/version.<Name>=...".
var (
	// GitHash is the git commit the binary was built from.
	GitHash = "unknown"
	// Version is the semantic release version (e.g. "1.2.3"), derived from the
	// git tag. Defaults to "dev" for non-release builds.
	Version = "dev"
)
