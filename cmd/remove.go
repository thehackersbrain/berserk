package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thehackersbrain/berserk/internal/installer"
	"github.com/thehackersbrain/berserk/internal/state"
)

var removeDryRun bool

var removeCmd = &cobra.Command{
	Use:   "remove <tool> [tool...]",
	Short: "Remove one or more installed tools",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, d, _, err := loadContext()
		if err != nil {
			return err
		}

		// Load state once. Dry-run skips it for parity with install — nothing
		// is going to be persisted, so reading the file is just I/O for a
		// value we won't use.
		var st *state.State
		if !removeDryRun {
			s, err := state.Load()
			if err != nil {
				printWarn("loading install state: %v (state will not be updated)", err)
			} else {
				st = s
			}
		}

		for _, name := range args {
			t, ok := reg.FindTool(name)
			if !ok {
				printWarn("unknown tool: %s, skipping", name)
				continue
			}

			if removeDryRun {
				fmt.Printf("[dry-run] would remove %s (installer: %s)\n", t.Name, t.Installer)
				continue
			}

			printProgress("Removing %s...", t.Name)
			if err := installer.Remove(*t, d); err != nil {
				printWarn("failed to remove %s: %v", t.Name, err)
				continue
			}

			if st != nil {
				if err := st.Remove(t.Name); err != nil {
					printWarn("removed %s, but updating state failed: %v", t.Name, err)
				}
			}

			printOK("%s removed", t.Name)
		}

		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVar(&removeDryRun, "dry-run", false, "show what would be removed without doing it")
	rootCmd.AddCommand(removeCmd)
}
