package cmd

import (
	"fmt"
	"runtime"
)

var Version = "v0.2.3"

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(fmt.Sprintf("berserk %s %s/%s\n", Version, runtime.GOOS, runtime.GOARCH))
}
