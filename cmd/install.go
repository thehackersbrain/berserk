package cmd

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/thehackersbrain/berserk/internal/conflict"
	"github.com/thehackersbrain/berserk/internal/distro"
	"github.com/thehackersbrain/berserk/internal/installer"
	"github.com/thehackersbrain/berserk/internal/registry"
	"github.com/thehackersbrain/berserk/internal/state"
	"github.com/spf13/cobra"
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
			return cmd.Help()
		}

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

var printMu sync.Mutex

func runInstalls(tools []registry.Tool, d distro.Distro, opts installer.Options, parallel bool, st *state.State) error {
	var failed atomic.Int32

	doOne := func(t registry.Tool) {
		if err := installOne(t, d, opts, st); err != nil {
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

func installOne(t registry.Tool, d distro.Distro, opts installer.Options, st *state.State) error {
	if c := conflict.Check(t.Name); c != "" {
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

func init() {
	installCmd.Flags().StringVarP(&installProfile, "profile", "p", "", "install all tools in a profile")
	installCmd.Flags().BoolVar(&installAll, "all", false, "install all tools")
	installCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "show what would be installed without doing it")
	rootCmd.AddCommand(installCmd)
}
