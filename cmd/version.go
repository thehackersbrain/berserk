package cmd

import (
	"fmt"
	"runtime"
)

// Version is set at build time via -ldflags "-X github.com/thehackersbrain/berserk/cmd.Version=v1.2.3"
// The default below is the baseline for unstamped builds (e.g. `go install`) —
// `make build` overrides it from `git describe` when a tag is reachable.
var Version = "v0.1.3"

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(fmt.Sprintf("berserk %s %s/%s\n", Version, runtime.GOOS, runtime.GOARCH))
}
