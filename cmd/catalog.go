package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var getCatalog = &cobra.Command{
	Use:   "catalog",
	Short: "Get tools catalog",
	RunE: func(cmd *cobra.Command, args []string) error {
		target := "/usr/share/berserk/"
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("directory already exists: %s", target)
		}

		gcmd := exec.Command("git", "clone", "https://github.com/berserkarch/berserk-repo.git", target)
		gcmd.Stdout = os.Stdout
		gcmd.Stderr = os.Stderr
		gcmd.Stdin = os.Stdin
		if err := gcmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCatalog)
}
