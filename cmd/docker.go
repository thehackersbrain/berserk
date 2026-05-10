package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pterm/pterm"
	"github.com/thehackersbrain/berserk/internal/docker"
)

// dockerDataDir returns ~/berserk — the root for container volume mounts.
func dockerDataDir() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return ""
	}
	return filepath.Join(h, "berserk")
}

// expandRunCmd rewrites btweak-era volume paths to ~/berserk/ and expands ~.
func expandRunCmd(cmd string) string {
	h, _ := os.UserHomeDir()
	data := dockerDataDir()

	cmd = strings.ReplaceAll(cmd, "~/btweak/containers/", data+"/containers/")
	cmd = strings.ReplaceAll(cmd, "~/btweak/docker/", data+"/docker/")

	if h != "" {
		cmd = strings.ReplaceAll(cmd, "~/", h+"/")
	}
	return cmd
}

// volumeRe matches -v host:container pairs in docker run strings.
var volumeRe = regexp.MustCompile(`-v\s+([^:\s]+):[^\s]+`)

// extractVolumes returns host-side paths from every -v flag in cmd.
func extractVolumes(cmd string) []string {
	var paths []string
	for _, m := range volumeRe.FindAllStringSubmatch(cmd, -1) {
		if len(m) == 2 {
			paths = append(paths, m[1])
		}
	}
	return paths
}

// ensureVolumeDirs creates every host volume path with MkdirAll.
func ensureVolumeDirs(paths []string) error {
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return fmt.Errorf("creating volume dir %s: %w", p, err)
		}
	}
	return nil
}

// loadDockerGroups reads all *.yaml files from <configDir>/containers/.
// Returns nil groups (no error) when the directory doesn't exist so callers
// that are best-effort (search, list) can degrade gracefully.
func loadDockerGroups() ([]docker.Group, error) {
	dir := filepath.Join(configDir(), "containers")
	groups, err := docker.LoadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("loading docker catalog: %w\n\tadd container yaml files to %s", err, dir)
	}
	return groups, nil
}

// printContainerDetails prints a container's name, description, pull/run
// commands and optional runtime notes (paths shown from catalog as-is).
func printContainerDetails(c docker.Container) {
	pterm.Println("  " + pterm.Bold.Sprint(c.Name))
	pterm.DefaultBasicText.Printfln("    %s", pterm.Gray(c.Description))
	pterm.DefaultBasicText.Printfln("    Pull: %s", pterm.Cyan(c.Command))
	pterm.DefaultBasicText.Printfln("    Run:  %s", pterm.Yellow(c.Run))
	for _, line := range c.RuntimeComments {
		pterm.DefaultBasicText.Printfln("    %s", pterm.Gray(line))
	}
}
