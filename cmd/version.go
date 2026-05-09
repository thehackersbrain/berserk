package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X github.com/thehackersbrain/berserk/cmd.Version=v1.2.3"
// The default below is the baseline for unstamped builds (e.g. `go install`) —
// `make build` overrides it from `git describe` when a tag is reachable.
var Version = "v0.1.2"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print berserk version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("berserk %s %s/%s\n", Version, runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
