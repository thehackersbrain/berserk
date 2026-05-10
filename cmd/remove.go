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

			if st, err := state.Load(); err == nil {
				if err := st.Remove(t.Name); err != nil {
					printWarn("removed %s, but updating state failed: %v", t.Name, err)
				}
			} else {
				printWarn("removed %s, but loading state failed: %v", t.Name, err)
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
