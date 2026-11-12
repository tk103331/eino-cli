// Package version holds the build version, injected at release time via
// -ldflags "-X github.com/tk103331/eino-cli/version.Version=vX.Y.Z".
package version

// Version is the CLI version ("dev" for local builds).
var Version = "dev"
