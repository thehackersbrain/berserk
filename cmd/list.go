package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/thehackersbrain/berserk/internal/registry"
)

var (
	listCategories bool
	listProfiles   bool
	listContainers bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tools, profiles, categories, or Docker container groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		if listContainers {
			groups, err := loadDockerGroups()
			if err != nil {
				return err
			}

			var items []pterm.BulletListItem
			for i, g := range groups {
				items = append(items, pterm.BulletListItem{
					Level:       0,
					Text:        fmt.Sprintf("%d. %s", i+1, pterm.Bold.Sprint(pterm.Cyan(g.Name))),
					BulletStyle: pterm.NewStyle(pterm.FgCyan),
					Bullet:      "▸",
				})
				items = append(items, pterm.BulletListItem{
					Level:  1,
					Text:   pterm.Gray(g.Description),
					Bullet: " ",
				})

				if len(g.Categories) > 0 {
					total := 0
					for _, cat := range g.Categories {
						total += len(cat.Containers)
					}
					items = append(items, pterm.BulletListItem{
						Level:  1,
						Text:   pterm.Yellow(fmt.Sprintf("%d categories, %d containers", len(g.Categories), total)),
						Bullet: " ",
					})
					for j, cat := range g.Categories {
						items = append(items, pterm.BulletListItem{
							Level:       1,
							Text:        fmt.Sprintf("%d.%d. %s %s", i+1, j+1, pterm.Magenta(cat.Name), pterm.Gray(fmt.Sprintf("(%d containers)", len(cat.Containers)))),
							BulletStyle: pterm.NewStyle(pterm.FgMagenta),
							Bullet:      "▸",
						})
					}
				} else {
					items = append(items, pterm.BulletListItem{
						Level:  1,
						Text:   pterm.Yellow(fmt.Sprintf("%d containers", len(g.Containers))),
						Bullet: " ",
					})
					for _, c := range g.Containers {
						items = append(items, pterm.BulletListItem{
							Level:       2,
							Text:        pterm.Green(c.Name),
							BulletStyle: pterm.NewStyle(pterm.FgGreen),
							Bullet:      "▸",
						})
					}
				}
			}

			pterm.Println()
			return pterm.DefaultBulletList.WithItems(items).Render()
		}

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
	listCmd.Flags().BoolVarP(&listContainers, "containers", "d", false, "list Docker container groups from the catalog")
	listCmd.MarkFlagsMutuallyExclusive("profiles", "categories", "containers")
	rootCmd.AddCommand(listCmd)
}
