package cmd

import (
	"errors"
	"fmt"
	"io/fs"
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
		_, err := os.Stat(catalogTarget)
		var gcmd *exec.Cmd
		switch {
		case err == nil:
			fmt.Fprintf(os.Stderr, "Pulling latest catalog in %s...\n", catalogTarget)
			gcmd = exec.Command("sudo", "git", "-C", catalogTarget, "pull")
		case errors.Is(err, fs.ErrNotExist):
			gcmd = exec.Command("sudo", "git", "clone", catalogRepo, catalogTarget)
		default:
			return fmt.Errorf("checking %s: %w", catalogTarget, err)
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
