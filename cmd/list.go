package cmd

import (
	"sort"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/thehackersbrain/berserk/internal/registry"
)

var (
	listCategories bool
	listProfiles   bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tools, profiles, or categories",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, _, _, err := loadContext()
		if err != nil {
			return err
		}

		if listCategories {
			if len(reg.Categories) == 0 {
				pterm.Info.Println("No categories defined.")
				return nil
			}

			idx := make([]int, len(reg.Categories))
			for i := range idx {
				idx[i] = i
			}
			sort.Slice(idx, func(a, b int) bool {
				return reg.Categories[idx[a]].Name < reg.Categories[idx[b]].Name
			})

			data := pterm.TableData{{"CATEGORY", "TOOLS", "DESCRIPTION"}}
			for _, i := range idx {
				c := reg.Categories[i]
				data = append(data, []string{
					c.Name,
					pterm.Sprintf("%d", reg.CategoryToolCount(c.Name)),
					c.Description,
				})
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
		}

		if listProfiles {
			if len(reg.Profiles) == 0 {
				pterm.Info.Println("No profiles defined.")
				return nil
			}

			idx := make([]int, len(reg.Profiles))
			for i := range reg.Profiles {
				idx[i] = i
			}
			sort.Slice(idx, func(a, b int) bool {
				return reg.Profiles[idx[a]].Name < reg.Profiles[idx[b]].Name
			})

			data := pterm.TableData{{"PROFILE", "TOOLS", "INCLUDES", "DESCRIPTION"}}
			for _, i := range idx {
				p := reg.Profiles[i]
				data = append(data, []string{
					p.Name,
					pterm.Sprintf("%d", reg.ProfileMemberCount(p.Name)),
					strings.Join(p.Includes, ", "),
					truncRunes(p.Description, 60),
				})
			}

			if err := pterm.DefaultTable.WithHasHeader().WithHeaderStyle(pterm.NewStyle(pterm.Bold)).WithData(data).Render(); err != nil {
				return err
			}

			pterm.Println()
			pterm.DefaultBasicText.Printfln("%s profile(s) — install one with: berserk install --profile <name>",
				pterm.Bold.Sprintf("%d", len(reg.Profiles)))
			return nil
		}

		st := loadStateOrWarn()
		// list's ✓ marker is "did berserk install this?" — state-only.
		// PATH fallback would falsely claim credit for system-installed
		// tools we never touched.
		present := installedSet(reg.Tools, st, false)

		if len(reg.Tools) == 0 {
			pterm.Info.Println("No tools found.")
			return nil
		}

		return renderToolTable(reg.Tools, true, present)
	},
}

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
	listCmd.Flags().BoolVarP(&listProfiles, "profiles", "p", false, "list all profiles")
	listCmd.Flags().BoolVarP(&listCategories, "categories", "c", false, "list all categories")
	listCmd.MarkFlagsMutuallyExclusive("profiles", "categories")
	rootCmd.AddCommand(listCmd)
}
