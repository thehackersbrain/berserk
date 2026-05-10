package cmd

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
	"github.com/thehackersbrain/berserk/internal/distro"
	"github.com/thehackersbrain/berserk/internal/installer"
	"github.com/thehackersbrain/berserk/internal/registry"
)

var (
	updateProfile string
	updateDryRun  bool
	updateBackend string
)

var validUpdateBackends = map[string]bool{
	"pipx": true, "cargo": true, "npm": true, "go": true, "gem": true,
}

var updateCmd = &cobra.Command{
	Use:   "update [tool...]",
	Short: "Update tools to latest versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		if updateBackend != "" && !validUpdateBackends[updateBackend] {
			return fmt.Errorf("unknown backend %q (expected one of: pipx, cargo, npm, go, gem)", updateBackend)
		}
		// --backend only narrows the all-backends sweep; combining it with
		// explicit tool names or --profile is a category error, not a
		// silent no-op.
		if updateBackend != "" && (len(args) > 0 || updateProfile != "") {
			return fmt.Errorf("--backend only applies to the no-arg update sweep; drop it when passing tool names or --profile")
		}
		// --profile and positional args also conflict — silently picking
		// one over the other would mask user intent.
		if updateProfile != "" && len(args) > 0 {
			return fmt.Errorf("--profile and explicit tool names are mutually exclusive")
		}

		reg, d, opts, err := loadContext()
		if err != nil {
			return err
		}
		opts.DryRun = updateDryRun

		var failed atomic.Int32

		switch {
		case len(args) > 0:
			// Resolve every name up front so a typo on one tool doesn't strand
			// the user mid-batch — same UX as install.
			var tools []registry.Tool
			var unknown []string
			for _, name := range args {
				t, ok := reg.FindTool(name)
				if !ok {
					unknown = append(unknown, name)
					continue
				}
				tools = append(tools, *t)
			}
			if len(unknown) > 0 {
				return fmt.Errorf("unknown tool(s): %s", strings.Join(unknown, ", "))
			}
			// Parity with install: drop tools not configured for this
			// distro instead of letting them error deep in pacman/apt.
			tools = filterByDistro(tools, d)
			// Parity with install + the --profile branch: honor parallel.
			failed.Store(updateAll(tools, d, opts, reg.Config.Parallel))
		case updateProfile != "":
			tools, err := reg.ToolsForProfile(updateProfile)
			if err != nil {
				return err
			}
			tools = filterByDistro(tools, d)
			failed.Store(updateAll(tools, d, opts, reg.Config.Parallel))
		default:
			if !opts.DryRun {
				printProgress("Updating all backends...")
			}
			failed.Store(updateAllBackends(reg, d, opts, updateBackend))
		}

		if n := failed.Load(); n > 0 {
			return fmt.Errorf("%d update(s) failed", n)
		}
		return nil
	},
}

func updateAllBackends(reg *registry.Registry, d distro.Distro, opts installer.Options, only string) int32 {
	type backendFn struct {
		name string
		fn   func() error
	}

	want := func(name string) bool {
		return only == "" || only == name
	}

	// Dry-run path is a pure-print operation — listing what would happen
	// per backend. We do this first so we don't have to plumb opts.DryRun
	// through every goroutine below.
	if opts.DryRun {
		return dryRunUpdateAllBackends(reg, want)
	}

	// Load state once, before any goroutine launches. Putting it here
	// avoids a future race if a pipx/cargo/npm goroutine later grows
	// state-reading logic, and lets us hold printMu around the warning.
	st := loadStateOrWarn()

	var (
		wg     sync.WaitGroup
		failed atomic.Int32
	)

	for _, b := range []backendFn{
		{"pipx", installer.UpdatePipx},
		{"cargo", installer.UpdateCargo},
		{"npm", installer.UpdateNpm},
	} {
		if !want(b.name) {
			continue
		}
		wg.Add(1)
		go func(b backendFn) {
			defer wg.Done()
			printMu.Lock()
			printProgress("Updating %s packages...", b.name)
			printMu.Unlock()
			if err := b.fn(); err != nil {
				failed.Add(1)
				printMu.Lock()
				printWarn("%s update: %v", b.name, err)
				printMu.Unlock()
			} else {
				printMu.Lock()
				printOK("%s updated", b.name)
				printMu.Unlock()
			}
		}(b)
	}

	if want("go") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, t := range reg.Tools {
				if t.Installer != "go" {
					continue
				}
				if st != nil && !st.Has(t.Name) {
					continue
				}
				printMu.Lock()
				printProgress("Updating go tool: %s", t.Name)
				printMu.Unlock()
				if err := installer.Go(t); err != nil {
					failed.Add(1)
					printMu.Lock()
					printWarn("go update %s: %v", t.Name, err)
					printMu.Unlock()
				}
			}
		}()
	}

	if want("gem") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, t := range reg.Tools {
				if t.Installer != "gem" {
					continue
				}
				if st != nil && !st.Has(t.Name) {
					continue
				}
				printMu.Lock()
				printProgress("Updating gem: %s", t.Name)
				printMu.Unlock()
				if err := installer.GemUpgrade(t); err != nil {
					failed.Add(1)
					printMu.Lock()
					printWarn("gem update %s: %v", t.Name, err)
					printMu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	n := failed.Load()
	if n == 0 {
		printOK("All backends updated")
	} else {
		printWarn("Backend updates finished with %d failure(s)", n)
	}
	return n
}

// dryRunUpdateAllBackends prints the work the no-arg sweep WOULD do
// without touching any pkg manager or running any goroutine. The bug it
// closes: previously the sweep ignored opts.DryRun entirely and called
// pipx upgrade-all, cargo install-update -a, sudo npm update -g, etc.
// regardless — an extremely surprising "dry-run".
func dryRunUpdateAllBackends(reg *registry.Registry, want func(string) bool) int32 {
	st := loadStateOrWarn()
	if want("pipx") {
		fmt.Println("[dry-run] would run: pipx upgrade-all")
	}
	if want("cargo") {
		fmt.Println("[dry-run] would run: cargo install-update -a")
	}
	if want("npm") {
		fmt.Println("[dry-run] would run: sudo npm update -g")
	}
	if want("go") {
		for _, t := range reg.Tools {
			if t.Installer != "go" {
				continue
			}
			if st != nil && !st.Has(t.Name) {
				continue
			}
			fmt.Printf("[dry-run] would update go tool: %s\n", t.Name)
		}
	}
	if want("gem") {
		for _, t := range reg.Tools {
			if t.Installer != "gem" {
				continue
			}
			if st != nil && !st.Has(t.Name) {
				continue
			}
			fmt.Printf("[dry-run] would update gem: %s\n", t.Name)
		}
	}
	return 0
}

func updateAll(tools []registry.Tool, d distro.Distro, opts installer.Options, parallel bool) int32 {
	var failed atomic.Int32

	doOne := func(t registry.Tool) {
		// On dry-run, installer.Update prints its own "[dry-run] would
		// update X via Y" line. Wrapping it in "Updating..." +
		// "X updated" would falsely imply the tool actually changed.
		if !opts.DryRun {
			printMu.Lock()
			printProgress("Updating %s...", t.Name)
			printMu.Unlock()
		}
		if err := installer.Update(t, d, opts); err != nil {
			failed.Add(1)
			printMu.Lock()
			printErr("%s: %v", t.Name, err)
			printMu.Unlock()
			return
		}
		if opts.DryRun {
			return
		}
		printMu.Lock()
		printOK("%s updated", t.Name)
		printMu.Unlock()
	}

	if !parallel {
		for _, t := range tools {
			doOne(t)
		}
		return failed.Load()
	}

	var wg sync.WaitGroup
	for _, t := range tools {
		wg.Add(1)
		go func(tool registry.Tool) {
			defer wg.Done()
			doOne(tool)
		}(t)
	}
	wg.Wait()
	return failed.Load()
}

func init() {
	updateCmd.Flags().StringVarP(&updateProfile, "profile", "p", "", "update all tools in a profile")
	updateCmd.Flags().StringVarP(&updateBackend, "backend", "b", "", "only update a specific backend (pipx, cargo, npm, go, gem)")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "show what would be updated without doing it")
	rootCmd.AddCommand(updateCmd)
}
