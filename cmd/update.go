package cmd

import (
	"fmt"
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

var updateCmd = &cobra.Command{
	Use:   "update [tool...]",
	Short: "Update tools to latest versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, d, opts, err := loadContext()
		if err != nil {
			return err
		}
		opts.DryRun = updateDryRun

		var failed atomic.Int32

		switch {
		case len(args) > 0:
			for _, name := range args {
				t, ok := reg.FindTool(name)
				if !ok {
					printWarn("unknown tool: %s", name)
					continue
				}
				printProgress("Updating %s...", name)
				if err := installer.Update(*t, d, opts); err != nil {
					printErr("%s: %v", name, err)
					failed.Add(1)
				} else {
					printOK("%s updated", name)
				}
			}
		case updateProfile != "":
			tools, err := reg.ToolsForProfile(updateProfile)
			if err != nil {
				return err
			}
			tools = filterByDistro(tools, d)
			failed.Store(updateAll(tools, d, opts, reg.Config.Parallel))
		default:
			printProgress("Updating all backends...")
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

	st := loadStateOrWarn()

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

func updateAll(tools []registry.Tool, d distro.Distro, opts installer.Options, parallel bool) int32 {
	var failed atomic.Int32

	doOne := func(t registry.Tool) {
		printMu.Lock()
		printProgress("Updating %s...", t.Name)
		printMu.Unlock()
		if err := installer.Update(t, d, opts); err != nil {
			failed.Add(1)
			printMu.Lock()
			printErr("%s: %v", t.Name, err)
			printMu.Unlock()
		} else {
			printMu.Lock()
			printOK("%s updated", t.Name)
			printMu.Unlock()
		}
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
