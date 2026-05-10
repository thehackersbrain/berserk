// Package cmd -- Main Handler
package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().String("config", "", "config directory holding config.yaml and tool catalog yaml files (default "+defaultConfigDir+")")
}

// configDir resolves which directory to load config and tool yaml files from.
// Explicit --config wins; otherwise we use the default install path.
func configDir() string {
	if f := rootCmd.PersistentFlags().Lookup("config"); f != nil && f.Changed {
		return f.Value.String()
	}
	return defaultConfigDir
}

func initConfig() {
	dir := configDir()
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}

func loadContext() (*registry.Registry, distro.Distro, installer.Options, error) {
	dir := configDir()
	reg, err := registry.LoadDir(dir)
	if err != nil {
		return nil, 0, installer.Options{}, fmt.Errorf("loading registry from %s: %w\n\trun %s to get the tools catalog", dir, err, pterm.Green("berserk catalog"))
	}

	d := distro.Detect()

	token := reg.Config.GithubToken
	if t := viper.GetString("github_token"); t != "" {
		token = t
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		token = t
	}

	installDir := reg.Config.InstallDir
	if installDir == "" {
		installDir = "/usr/local/bin"
	}

	verbose := viper.GetBool("verbose")
	installer.SetVerbose(verbose)

	// UseSudo: default true; allow disabling via config or env
	installer.UseSudo = true
	if viper.IsSet("use_sudo") {
		installer.UseSudo = viper.GetBool("use_sudo")
	}
	if v := os.Getenv("BERSERK_NO_SUDO"); v == "1" || v == "true" {
		installer.UseSudo = false
	}

	opts := installer.Options{
		InstallDir:  installDir,
		GithubToken: token,
		Verbose:     verbose,
	}

	return reg, d, opts, nil
}
