package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/thehackersbrain/berserk/internal/registry"
	"github.com/spf13/cobra"
)

var (
	searchCategory  string
	searchInstaller string
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

		// Require at least one constraint so a bare `berserk search` doesn't
		// dump the entire catalog (use `berserk list` for that).
		if opts.Query == "" && opts.Category == "" && opts.Installer == "" &&
			!searchInstalled && !searchAvailable {
			return fmt.Errorf("provide a query or at least one filter (--category, --installer, --installed, --available)")
		}

		results := reg.SearchWith(opts)

		// Compute install status once and reuse for both filtering and rendering.
		st := loadStateOrWarn()
		installed := installedSet(results, st)
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

// truncRunes shortens s to at most n display runes (not bytes) so multi-byte
// characters in tool descriptions don't get sliced through the middle.
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
	searchCmd.Flags().StringVarP(&searchCategory, "category", "c", "", "filter by category (e.g. web, recon, ad)")
	searchCmd.Flags().StringVarP(&searchInstaller, "installer", "i", "", "filter by installer (pipx, go, cargo, gem, npm, binary, system, oneliner)")
	searchCmd.Flags().BoolVar(&searchInstalled, "installed", false, "only show tools already on $PATH")
	searchCmd.Flags().BoolVar(&searchAvailable, "available", false, "only show tools not yet installed")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 0, "cap the number of results (0 = no cap)")
	searchCmd.MarkFlagsMutuallyExclusive("installed", "available")
	rootCmd.AddCommand(searchCmd)
}
