package version

// GitHash is the git commit the binary was built from. It is overwritten at
// build time via -ldflags "-X niecke-it.de/uptime/internal/version.GitHash=...".
var GitHash = "unknown"
