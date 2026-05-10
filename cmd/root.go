// Package cmd -- Main Handler
package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/thehackersbrain/berserk/internal/distro"
	"github.com/thehackersbrain/berserk/internal/installer"
	"github.com/thehackersbrain/berserk/internal/registry"
)

var rootCmd = &cobra.Command{
	Use:               "berserk",
	Short:             "Curated offensive security tool manager",
	Long:              "Package Manager for Security Tools, always from original sources, always latest.",
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// defaultConfigDir is the canonical install location for berserk's config dir.
// Override with --config <dir>. The dir holds config.yaml plus one or more
// tool catalog yaml files (any *.yaml or *.yml that isn't named config.yaml).
const defaultConfigDir = "/usr/share/berserk"

func init() {
	rootCmd.PersistentFlags().String("config", "", "config directory holding config.yaml and tool catalog yaml files (default "+defaultConfigDir+")")
	// --yes/-y maps to pacman's --noconfirm and apt's -y; off by default so
	// the underlying pkg mgr can still prompt on conflicts/replacements.
	rootCmd.PersistentFlags().BoolP("yes", "y", false, "assume yes to system pkg mgr prompts (pacman --noconfirm / apt -y)")
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
}

// configDir resolves which directory to load config and tool yaml files from.
// Explicit --config wins; otherwise we use the default install path.
func configDir() string {
	if f := rootCmd.PersistentFlags().Lookup("config"); f != nil && f.Changed {
		return f.Value.String()
	}
	return defaultConfigDir
}

func loadContext() (*registry.Registry, distro.Distro, installer.Options, error) {
	dir := configDir()
	reg, err := registry.LoadDir(dir)
	if err != nil {
		return nil, 0, installer.Options{}, fmt.Errorf("loading registry from %s: %w\n\trun %s to get the tools catalog", dir, err, pterm.Green("berserk sync"))
	}

	d := distro.Detect()

	// Config keys come from <dir>/config.yaml (flat schema). GITHUB_TOKEN
	// in the env still wins because it's the way most CI users set it.
	token := reg.Config.GithubToken
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		token = t
	}

	installDir := reg.Config.InstallDir
	if installDir == "" {
		installDir = "/usr/local/bin"
	}

	installer.SetVerbose(reg.Config.Verbose)

	// `--yes` is persistent on rootCmd, so any subcommand inherits it.
	// Lookup-then-bool keeps the rest of the codebase off cobra types.
	var assumeYes bool
	if f := rootCmd.PersistentFlags().Lookup("yes"); f != nil {
		assumeYes, _ = strconv.ParseBool(f.Value.String())
	}

	opts := installer.Options{
		InstallDir:  installDir,
		GithubToken: token,
		AssumeYes:   assumeYes,
	}

	return reg, d, opts, nil
}
