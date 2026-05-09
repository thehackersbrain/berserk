package cmd

import (
	"strings"

	"github.com/pterm/pterm"
	"github.com/thehackersbrain/berserk/internal/registry"
	"github.com/spf13/cobra"
)

var (
	listInstalled bool
	listProfile   string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available or installed tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, _, _, err := loadContext()
		if err != nil {
			return err
		}

		var tools []registry.Tool

		if listProfile != "" {
			tools, err = reg.ToolsForProfile(listProfile)
			if err != nil {
				return err
			}
		} else {
			tools = reg.Tools
		}

		st := loadStateOrWarn()
		present := installedSet(tools, st)

		if listInstalled {
			tools = filterByPresence(tools, present)
		}

		if len(tools) == 0 {
			pterm.Info.Println("No tools found.")
			return nil
		}

		return renderToolTable(tools, true, present)
	},
}

func filterByPresence(tools []registry.Tool, present map[string]bool) []registry.Tool {
	out := make([]registry.Tool, 0, len(tools))
	for _, t := range tools {
		if present[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

// renderToolTable renders a catalogue table. When showStatus is true, a leading
// column flags installed tools using the precomputed `installed` set.
func renderToolTable(tools []registry.Tool, showStatus bool, installed map[string]bool) error {
	header := []string{"NAME", "INSTALLER", "CATEGORIES", "DESCRIPTION"}
	if showStatus {
		header = append([]string{""}, header...)
	}
	data := pterm.TableData{header}

	for _, t := range tools {
		row := []string{
			t.Name,
			t.Installer,
			truncRunes(strings.Join(t.Category, ","), 30),
			truncRunes(t.Description, 60),
		}
		if showStatus {
			marker := ""
			if installed[t.Name] {
				marker = pterm.Green("✓")
			}
			row = append([]string{marker}, row...)
		}
		data = append(data, row)
	}

	return pterm.DefaultTable.WithHasHeader().WithHeaderStyle(pterm.NewStyle(pterm.Bold)).WithData(data).Render()
}

func init() {
	listCmd.Flags().BoolVar(&listInstalled, "installed", false, "only show installed tools")
	listCmd.Flags().StringVarP(&listProfile, "profile", "p", "", "filter by profile")
	rootCmd.AddCommand(listCmd)
}
