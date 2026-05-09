// Package cmd - check installed tools in the system
package cmd

import (
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Show tools that are not yet installed",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, _, _, err := loadContext()
		if err != nil {
			return err
		}

		st := loadStateOrWarn()
		present := installedSet(reg.Tools, st)

		var missing, installed int
		data := pterm.TableData{{"NAME", "INSTALLER", "STATUS"}}
		for _, t := range reg.Tools {
			if present[t.Name] {
				data = append(data, []string{t.Name, t.Installer, pterm.Green("installed")})
				installed++
			} else {
				data = append(data, []string{t.Name, t.Installer, pterm.Red("missing")})
				missing++
			}
		}

		if err := pterm.DefaultTable.WithHasHeader().WithHeaderStyle(pterm.NewStyle(pterm.Bold)).WithData(data).Render(); err != nil {
			return err
		}

		pterm.Println()
		pterm.DefaultBasicText.Printfln(
			"%s installed, %s missing",
			pterm.FgGreen.Sprintf("%d", installed),
			pterm.FgRed.Sprintf("%d", missing),
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
