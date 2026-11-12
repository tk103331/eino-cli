// Package version holds the build version. Release binaries inject it at build
// time via -ldflags "-X github.com/tk103331/eino-cli/version.Version=...";
// `go install pkg@version` builds rely on the module version from build info.
package version

import (
	"runtime/debug"
	"strings"
)

// Version is the CLI version ("dev" for plain local builds).
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	// `go install module@version` records the module version in build info.
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			Version = strings.TrimPrefix(v, "v")
		}
	}
}
