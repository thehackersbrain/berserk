package cmd

import (
	"fmt"
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
	installProfile string
	installAll     bool
	installDryRun  bool
)

var installCmd = &cobra.Command{
	Use:   "install [tool...]",
	Short: "Install one or more tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, d, opts, err := loadContext()
		if err != nil {
			return err
		}
		opts.DryRun = installDryRun

		var tools []registry.Tool

		switch {
		case installAll:
			tools = reg.Tools
		case installProfile != "":
			tools, err = reg.ToolsForProfile(installProfile)
			if err != nil {
				return err
			}
		case len(args) > 0:
			for _, name := range args {
				t, ok := reg.FindTool(name)
				if !ok {
					return fmt.Errorf("unknown tool: %s", name)
				}
				tools = append(tools, *t)
			}
		default:
			return fmt.Errorf("no tools specified — pass tool names, --profile, or --all")
		}

		// Drop tools that don't fit the current distro (e.g. system-installer
		// tools whose only configured package field is for the other distro).
		// Surface a single skipped-list summary rather than N failed installs.
		tools = filterByDistro(tools, d)

		// Real installs need a state handle so we can record successes;
		// dry-run skips it because nothing is actually getting installed.
		var st *state.State
		if !opts.DryRun {
			st, err = state.Load()
			if err != nil {
				return fmt.Errorf("loading install state: %w", err)
			}
		}

		return runInstalls(tools, d, opts, reg.Config.Parallel, st)
	},
}

func runInstalls(tools []registry.Tool, d distro.Distro, opts installer.Options, parallel bool, st *state.State) error {
	// Cache "is this tool already managed by some other package manager?"
	// once instead of spawning N×M subprocesses inside installOne.
	snap := conflict.LoadInstalled()

	var failed atomic.Int32

	doOne := func(t registry.Tool) {
		if err := installOne(t, d, opts, st, snap); err != nil {
			printMu.Lock()
			printErr("%s: %v", t.Name, err)
			printMu.Unlock()
			failed.Add(1)
		}
	}

	if !parallel || len(tools) == 1 {
		for _, t := range tools {
			doOne(t)
		}
	} else {
		var wg sync.WaitGroup
		for _, t := range tools {
			wg.Add(1)
			go func(tool registry.Tool) {
				defer wg.Done()
				doOne(tool)
			}(t)
		}
		wg.Wait()
	}

	if n := failed.Load(); n > 0 {
		return fmt.Errorf("%d tool(s) failed to install", n)
	}
	return nil
}

func installOne(t registry.Tool, d distro.Distro, opts installer.Options, st *state.State, snap conflict.Snapshot) error {
	if c := snap.Check(t.Name); c != "" {
		printMu.Lock()
		printWarn("%s already installed via %s", t.Name, c)
		printWarn("Use 'berserk remove %s' first if you want berserk to manage it", t.Name)
		printMu.Unlock()
	}

	// On dry-run, installer.Install prints its own "[dry-run] would install"
	// line — emitting "Installing..." and "X installed" around it would
	// falsely imply the tool actually got installed.
	if !opts.DryRun {
		printMu.Lock()
		printInfo("Installing %s (%s)...", t.Name, t.Installer)
		printMu.Unlock()
	}

	if err := installer.Install(t, d, opts); err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	// Record only on confirmed success. State persistence failures don't
	// roll back the install — the tool really is installed; we just couldn't
	// note it. Surface the warning instead of failing the command.
	if st != nil {
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
	installCmd.Flags().BoolVar(&installAll, "all", false, "install all tools")
	installCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "show what would be installed without doing it")
	rootCmd.AddCommand(installCmd)
}
