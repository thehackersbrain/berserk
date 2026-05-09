package cmd

import (
	"sort"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List available categories",
	Long: `Show every category declared in categories.yaml: its name, how many tools
belong to it, and its description. Filter by category with:
  berserk search --category <name>
  berserk list   --category <name>`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, _, _, err := loadContext()
		if err != nil {
			return err
		}

		if len(reg.Categories) == 0 {
			pterm.Info.Println("No categories defined.")
			return nil
		}

		idx := make([]int, len(reg.Categories))
		for i := range reg.Categories {
			idx[i] = i
		}
		sort.Slice(idx, func(a, b int) bool {
			return reg.Categories[idx[a]].Name < reg.Categories[idx[b]].Name
		})

		data := pterm.TableData{{"CATEGORY", "TOOLS", "DESCRIPTION"}}
		for _, i := range idx {
			c := reg.Categories[i]
			count := pterm.Sprintf("%d", reg.CategoryToolCount(c.Name))
			data = append(data, []string{c.Name, count, c.Description})
		}

		if err := pterm.DefaultTable.WithHasHeader().WithHeaderStyle(pterm.NewStyle(pterm.Bold)).WithData(data).Render(); err != nil {
			return err
		}

		pterm.Println()
		n := len(reg.Categories)
		suffix := "ies"
		if n == 1 {
			suffix = "y"
		}
		pterm.DefaultBasicText.Printfln("%s categor%s — filter with: berserk search --category <name>",
			pterm.Bold.Sprintf("%d", n), suffix)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(categoriesCmd)
}
