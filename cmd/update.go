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
			failed.Store(int32(updateAll(tools, d, opts, reg.Config.Parallel)))
		default:
			printProgress("Updating all backends...")
			failed.Store(int32(updateAllBackends(reg, d, opts)))
		}

		if n := failed.Load(); n > 0 {
			return fmt.Errorf("%d update(s) failed", n)
		}
		return nil
	},
}

func updateAllBackends(reg *registry.Registry, d distro.Distro, opts installer.Options) int {
	type backendFn struct {
		name string
		fn   func() error
	}
	backends := []backendFn{
		{"pipx", installer.UpdatePipx},
		{"cargo", installer.UpdateCargo},
		{"gem", installer.UpdateGem},
		{"npm", installer.UpdateNpm},
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed int
	)

	for _, b := range backends {
		wg.Add(1)
		go func(b backendFn) {
			defer wg.Done()
			mu.Lock()
			printProgress("Updating %s packages...", b.name)
			mu.Unlock()
			if err := b.fn(); err != nil {
				mu.Lock()
				printWarn("%s update: %v", b.name, err)
				failed++
				mu.Unlock()
			} else {
				mu.Lock()
				printOK("%s updated", b.name)
				mu.Unlock()
			}
		}(b)
	}

	// Reinstall all go tools @latest
	st := loadStateOrWarn()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, t := range reg.Tools {
			if t.Installer != "go" {
				continue
			}
			if st != nil && !st.Has(t.Name) {
				continue // skip go tools the user never installed
			}
			mu.Lock()
			printProgress("Updating go tool: %s", t.Name)
			mu.Unlock()
			if err := installer.Go(t); err != nil {
				mu.Lock()
				printWarn("go update %s: %v", t.Name, err)
				failed++
				mu.Unlock()
			}
		}
	}()

	wg.Wait()
	if failed == 0 {
		printOK("All backends updated")
	} else {
		printWarn("Backend updates finished with %d failure(s)", failed)
	}
	return failed
}

func updateAll(tools []registry.Tool, d distro.Distro, opts installer.Options, parallel bool) int {
	var (
		mu     sync.Mutex
		failed int
	)

	doOne := func(t registry.Tool) {
		mu.Lock()
		printProgress("Updating %s...", t.Name)
		mu.Unlock()
		if err := installer.Update(t, d, opts); err != nil {
			mu.Lock()
			printErr("%s: %v", t.Name, err)
			failed++
			mu.Unlock()
		} else {
			mu.Lock()
			printOK("%s updated", t.Name)
			mu.Unlock()
		}
	}

	if !parallel {
		for _, t := range tools {
			doOne(t)
		}
		return failed
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
	return failed
}

func init() {
	updateCmd.Flags().StringVarP(&updateProfile, "profile", "p", "", "update all tools in a profile")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "show what would be updated without doing it")
	rootCmd.AddCommand(updateCmd)
}
