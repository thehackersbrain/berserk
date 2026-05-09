package cmd

import (
	"fmt"
	"os/exec"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verify all installer backends are available",
	RunE: func(cmd *cobra.Command, args []string) error {
		backends := []string{"pipx", "cargo", "go", "gem", "npm", "git"}
		var items []pterm.BulletListItem
		ok := 0
		for _, b := range backends {
			path, err := exec.LookPath(b)
			if err != nil {
				items = append(items, pterm.BulletListItem{
					Level:       0,
					Bullet:      "✗",
					BulletStyle: pterm.NewStyle(pterm.FgRed),
					Text:        fmt.Sprintf("%-6s not found", b),
					TextStyle:   pterm.NewStyle(pterm.FgRed),
				})
				continue
			}
			ok++
			items = append(items, pterm.BulletListItem{
				Level:       0,
				Bullet:      "✓",
				BulletStyle: pterm.NewStyle(pterm.FgGreen),
				Text:        fmt.Sprintf("%-6s %s", b, pterm.Gray(path)),
			})
		}

		if err := pterm.DefaultBulletList.WithItems(items).Render(); err != nil {
			return err
		}
		pterm.Println()

		if ok == len(backends) {
			pterm.Success.Println("All backends available.")
		} else {
			pterm.Warning.Printfln("%d/%d backends missing — install them to use all tool types.",
				len(backends)-ok, len(backends))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
