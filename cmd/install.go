package cmd

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
	"github.com/thehackersbrain/berserk/internal/conflict"
	"github.com/thehackersbrain/berserk/internal/distro"
	"github.com/thehackersbrain/berserk/internal/installer"
	"github.com/thehackersbrain/berserk/internal/registry"
	"github.com/thehackersbrain/berserk/internal/state"
)

var (
	installProfile    string
	installCategories []string
	installAll        bool
	installDryRun     bool
)

// installItem wraps a tool with provenance: registry hits flow through
// the normal installer + state-tracking path, while fallback hits go
// through the system pkg mgr and stay outside berserk's state file.
type installItem struct {
	tool     registry.Tool
	fallback bool
}

var installCmd = &cobra.Command{
	Use:   "install [tool...]",
	Short: "Install one or more tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		// --all, --profile and --category are already mutually exclusive via cobra.
		// Catch the orthogonal case: positional args alongside either
		// flag silently dropped the args before, which masked user
		// intent.
		if installAll && len(args) > 0 {
			return fmt.Errorf("--all takes no positional args; drop the tool names")
		}
		if installProfile != "" && len(args) > 0 {
			return fmt.Errorf("--profile takes no positional args; drop the tool names")
		}
		if len(installCategories) > 0 && len(args) > 0 {
			return fmt.Errorf("--category takes no positional args; drop the tool names")
		}

		reg, d, opts, err := loadContext()
		if err != nil {
			return err
		}
		opts.DryRun = installDryRun

		var items []installItem

		switch {
		case installAll:
			items = toItems(reg.Tools)
		case installProfile != "":
			tools, err := reg.ToolsForProfile(installProfile)
			if err != nil {
				return err
			}
			items = toItems(tools)
		case len(installCategories) > 0:
			tools := reg.ToolsForCategories(installCategories)
			if len(tools) == 0 {
				return fmt.Errorf("no tools found in category/categories: %s", strings.Join(installCategories, ", "))
			}
			items = toItems(tools)
		case len(args) > 0:
			items, err = resolveInstallArgs(args, reg, d)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("no tools specified — pass tool names, --profile, --category, or --all")
		}

		// Drop registry tools that don't fit the current distro (e.g. system-
		// installer tools whose only configured package field is for the other
		// distro). Fallback items are always allowed through — they were
		// synthesized for the running distro by definition.
		items = filterItemsByDistro(items, d)

		// Real installs need a state handle so we can record successes;
		// dry-run skips it because nothing is actually getting installed.
		var st *state.State
		if !opts.DryRun {
			st, err = state.Load()
			if err != nil {
				return fmt.Errorf("loading install state: %w", err)
			}
		}

		return runInstalls(items, d, opts, reg.Config.Parallel, st)
	},
}

// resolveInstallArgs maps user-provided names to install items, falling back
// to the system package manager for names absent from the registry. Names
// that aren't in the registry AND can't fall back (e.g. unknown distro)
// are collected and surfaced as a single error.
func resolveInstallArgs(args []string, reg *registry.Registry, d distro.Distro) ([]installItem, error) {
	var items []installItem
	var unknownNoFallback []string
	backend := installer.SystemBackend(d)
	for _, name := range args {
		if t, ok := reg.FindTool(name); ok {
			items = append(items, installItem{tool: *t})
			continue
		}
		if backend == "" {
			unknownNoFallback = append(unknownNoFallback, name)
			continue
		}
		printInfo("%s not found in Berserk Registry", name)
		printInfo("Falling back to %s backend", backend)
		items = append(items, installItem{
			tool:     registry.Tool{Name: name, Installer: "system"},
			fallback: true,
		})
	}
	if len(unknownNoFallback) > 0 {
		return nil, fmt.Errorf("unknown tool(s) and no system fallback available on this distro: %s",
			strings.Join(unknownNoFallback, ", "))
	}
	return items, nil
}

func toItems(tools []registry.Tool) []installItem {
	items := make([]installItem, len(tools))
	for i, t := range tools {
		items[i] = installItem{tool: t}
	}
	return items
}

func runInstalls(items []installItem, d distro.Distro, opts installer.Options, parallel bool, st *state.State) error {
	// Cache "is this tool already managed by some other package manager?"
	// once instead of spawning N×M subprocesses inside installOne.
	snap := conflict.LoadInstalled()

	var failed atomic.Int32

	doOne := func(it installItem) {
		if err := installOne(it, d, opts, st, snap); err != nil {
			printMu.Lock()
			printErr("%s: %v", it.tool.Name, err)
			printMu.Unlock()
			failed.Add(1)
		}
	}

	if !parallel || len(items) == 1 {
		for _, it := range items {
			doOne(it)
		}
	} else {
		var wg sync.WaitGroup
		for _, it := range items {
			wg.Add(1)
			go func(item installItem) {
				defer wg.Done()
				doOne(item)
			}(it)
		}
		wg.Wait()
	}

	if n := failed.Load(); n > 0 {
		return fmt.Errorf("%d tool(s) failed to install", n)
	}
	return nil
}

func installOne(it installItem, d distro.Distro, opts installer.Options, st *state.State, snap conflict.Snapshot) error {
	t := it.tool

	// Conflict snapshot is meaningful only for berserk-managed installs:
	// "X is already installed via cargo, so don't overwrite via pipx" makes
	// no sense for a system-fallback that explicitly asked for the system
	// pkg mgr. Skip the warning for fallbacks.
	if !it.fallback {
		if c := snap.Check(t.Name); c != "" {
			printMu.Lock()
			printWarn("%s already installed via %s", t.Name, c)
			printWarn("Use 'berserk remove %s' first if you want berserk to manage it", t.Name)
			printMu.Unlock()
		}
	}

	// On dry-run, installer.Install prints its own "[dry-run] would install"
	// line — emitting "Installing..." and "X installed" around it would
	// falsely imply the tool actually got installed.
	if !opts.DryRun {
		printMu.Lock()
		if it.fallback {
			printInfo("Installing %s (system fallback)...", t.Name)
		} else {
			printInfo("Installing %s (%s)...", t.Name, t.Installer)
		}
		printMu.Unlock()
	}

	if err := installer.Install(t, d, opts); err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	// State only tracks registry-managed installs. Fallback tools are
	// ad-hoc system pkgs the user asked for by name; recording them would
	// imply berserk owns their lifecycle, which it deliberately doesn't.
	if !it.fallback && st != nil {
		if err := st.Add(t); err != nil {
			printMu.Lock()
			printWarn("%s: installed, but recording state failed: %v", t.Name, err)
			printMu.Unlock()
		}
	}

	printMu.Lock()
	printOK("%s installed", t.Name)
	printMu.Unlock()
	return nil
}

// filterByDistro drops tools that can't run on the detected distro. Only
// `system` installer tools are filtered: when both arch_package and
// debian_package are unset, the package defaults to t.Name on whichever
// distro applies and we leave the tool in. When at least one is set, only
// the matching distro keeps the tool.
func filterByDistro(tools []registry.Tool, d distro.Distro) []registry.Tool {
	out := make([]registry.Tool, 0, len(tools))
	var skipped []string
	for _, t := range tools {
		if toolFitsDistro(t, d) {
			out = append(out, t)
		} else {
			skipped = append(skipped, t.Name)
		}
	}
	if len(skipped) > 0 {
		printInfo("Skipping %d tool(s) not configured for %s: %v", len(skipped), d, skipped)
	}
	return out
}

// filterItemsByDistro is filterByDistro for install items. Fallback items
// are always kept — they were synthesized for the running distro.
func filterItemsByDistro(items []installItem, d distro.Distro) []installItem {
	out := make([]installItem, 0, len(items))
	var skipped []string
	for _, it := range items {
		if it.fallback || toolFitsDistro(it.tool, d) {
			out = append(out, it)
		} else {
			skipped = append(skipped, it.tool.Name)
		}
	}
	if len(skipped) > 0 {
		printInfo("Skipping %d tool(s) not configured for %s: %v", len(skipped), d, skipped)
	}
	return out
}

func toolFitsDistro(t registry.Tool, d distro.Distro) bool {
	if t.Installer != "system" {
		return true
	}
	archSet := t.ArchPackage != ""
	debianSet := t.DebianPackage != ""
	if !archSet && !debianSet {
		return true
	}
	switch d {
	case distro.Arch:
		return archSet
	case distro.Kali, distro.Parrot, distro.Debian:
		return debianSet
	default:
		return false
	}
}

func init() {
	installCmd.Flags().StringVarP(&installProfile, "profile", "p", "", "install all tools in a profile")
	installCmd.Flags().StringSliceVarP(&installCategories, "category", "c", nil, "install all tools in one or more categories")
	installCmd.Flags().BoolVar(&installAll, "all", false, "install all tools")
	installCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "show what would be installed without doing it")
	installCmd.MarkFlagsMutuallyExclusive("all", "profile", "category")
	rootCmd.AddCommand(installCmd)
}
