package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

const (
	catalogTarget = "/usr/share/berserk/"
	catalogRepo   = "https://github.com/berserkarch/berserk-repo.git"
)

var syncCatalog = &cobra.Command{
	Use:   "sync",
	Short: "Sync tools catalog (clone if absent, pull if present)",
	RunE: func(cmd *cobra.Command, args []string) error {
		var gcmd *exec.Cmd
		if _, err := os.Stat(catalogTarget); os.IsNotExist(err) {
			gcmd = exec.Command("git", "clone", catalogRepo, catalogTarget)
		} else {
			fmt.Fprintf(os.Stderr, "Pulling latest catalog in %s...\n", catalogTarget)
			gcmd = exec.Command("git", "-C", catalogTarget, "pull")
		}

		gcmd.Stdout = os.Stdout
		gcmd.Stderr = os.Stderr
		gcmd.Stdin = os.Stdin

		if err := gcmd.Run(); err != nil {
			return fmt.Errorf("git operation failed: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCatalog)
}
