package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/thehackersbrain/berserk/internal/registry"
)

var (
	searchCategory  string
	searchInstaller string
	searchProfile   string
	searchInstalled bool
	searchAvailable bool
	searchLimit     int
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the catalog for tools you can install",
	Long: `Search tools by name, alias, category, or description.

Filters compose with the query and with each other:
  berserk search nmap                  # name/alias/desc match
  berserk search --category recon      # everything tagged 'recon'
  berserk search ad --installer pipx   # 'ad' tools installed via pipx
  berserk search --available web       # web tools you don't have yet
  berserk search --profile red-team    # search within a profile

Results show a green ✓ for tools already on your $PATH.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, _, _, err := loadContext()
		if err != nil {
			return err
		}

		opts := registry.SearchOpts{
			Category:  searchCategory,
			Installer: searchInstaller,
		}
		if len(args) == 1 {
			opts.Query = args[0]
		}

		if opts.Query == "" && opts.Category == "" && opts.Installer == "" &&
			searchProfile == "" && !searchInstalled && !searchAvailable {
			return fmt.Errorf("provide a query or at least one filter (--category, --installer, --profile, --installed, --available)")
		}

		var results []registry.Tool

		if searchProfile != "" {
			profileTools, err := reg.ToolsForProfile(searchProfile)
			if err != nil {
				return err
			}
			// Search the profile subset by matching opts against it directly.
			results = registry.SearchSlice(profileTools, opts)
		} else {
			results = reg.SearchWith(opts)
		}

		st := loadStateOrWarn()
		// search's --installed/--available is "is this on the box?" —
		// PATH fallback catches tools the user installed before berserk
		// knew about them.
		installed := installedSet(results, st, true)
		results = filterByInstallStatus(results, installed, searchInstalled, searchAvailable)

		if len(results) == 0 {
			pterm.Info.Println("No tools matched.")
			return nil
		}

		total := len(results)
		if searchLimit > 0 && total > searchLimit {
			results = results[:searchLimit]
		}

		if err := renderToolTable(results, true, installed); err != nil {
			return err
		}

		if total > len(results) {
			pterm.DefaultBasicText.Printfln("\nshowing %s of %s result(s)",
				pterm.Bold.Sprintf("%d", len(results)),
				pterm.Bold.Sprintf("%d", total))
		} else {
			pterm.DefaultBasicText.Printfln("\n%s result(s)", pterm.Bold.Sprintf("%d", total))
		}
		return nil
	},
}

func filterByInstallStatus(tools []registry.Tool, installed map[string]bool, onlyInstalled, onlyAvailable bool) []registry.Tool {
	if !onlyInstalled && !onlyAvailable {
		return tools
	}
	out := make([]registry.Tool, 0, len(tools))
	for _, t := range tools {
		present := installed[t.Name]
		if onlyInstalled && present {
			out = append(out, t)
		} else if onlyAvailable && !present {
			out = append(out, t)
		}
	}
	return out
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 3 {
		return string(r[:n])
	}
	return string(r[:n-3]) + "..."
}

func init() {
	searchCmd.Flags().StringVarP(&searchCategory, "category", "c", "", "filter by category")
	searchCmd.Flags().StringVarP(&searchInstaller, "installer", "b", "", "filter by installer")
	searchCmd.Flags().StringVarP(&searchProfile, "profile", "p", "", "search within a profile")
	searchCmd.Flags().BoolVarP(&searchInstalled, "installed", "i", false, "only show tools already on $PATH")
	searchCmd.Flags().BoolVar(&searchAvailable, "available", false, "only show tools not yet installed")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 0, "cap the number of results (0 = no cap)")
	searchCmd.MarkFlagsMutuallyExclusive("installed", "available")
	rootCmd.AddCommand(searchCmd)
}
