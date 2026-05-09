package cmd

import (
	"sort"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List available profiles",
	Long: `Show every profile declared in profiles.yaml: its name, description,
the resolved tool count (direct members plus included profiles' members), and
any composition via 'includes:'. Use 'berserk list --profile <name>' to
inspect the actual members of one profile.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, _, _, err := loadContext()
		if err != nil {
			return err
		}

		if len(reg.Profiles) == 0 {
			pterm.Info.Println("No profiles defined.")
			return nil
		}

		// Sort indices so we render in alphabetical order without copying
		// the (potentially large) Profile struct values.
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
			count := pterm.Sprintf("%d", reg.ProfileMemberCount(p.Name))
			data = append(data, []string{
				p.Name,
				count,
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
	},
}

func init() {
	rootCmd.AddCommand(profilesCmd)
}
