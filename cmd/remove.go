package cmd

import (
	"fmt"

	"github.com/thehackersbrain/berserk/internal/installer"
	"github.com/thehackersbrain/berserk/internal/state"
	"github.com/spf13/cobra"
)

var removeDryRun bool

var removeCmd = &cobra.Command{
	Use:   "remove <tool>",
	Short: "Remove an installed tool",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, d, _, err := loadContext()
		if err != nil {
			return err
		}

		t, ok := reg.FindTool(args[0])
		if !ok {
			return fmt.Errorf("unknown tool: %s", args[0])
		}

		if removeDryRun {
			fmt.Printf("[dry-run] would remove %s (installer: %s)\n", t.Name, t.Installer)
			return nil
		}

		printProgress("Removing %s...", t.Name)
		if err := installer.Remove(*t, d); err != nil {
			return err
		}

		// Drop the state entry now that the tool is gone. A read failure here
		// is non-fatal — the uninstall succeeded, the state file is just stale.
		if st, err := state.Load(); err == nil {
			if err := st.Remove(t.Name); err != nil {
				printWarn("removed, but updating state failed: %v", err)
			}
		} else {
			printWarn("removed, but loading state failed: %v", err)
		}

		printOK("%s removed", t.Name)
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVar(&removeDryRun, "dry-run", false, "show what would be removed without doing it")
	rootCmd.AddCommand(removeCmd)
}
