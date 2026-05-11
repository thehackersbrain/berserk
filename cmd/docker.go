package cmd

import (
	"errors"
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

// rewriteBtweakPaths replaces ~/btweak/{containers,docker}/ with
// ~/berserk/{containers,docker}/. Used for display so the user sees where
// data actually lives without expanding ~ to an absolute path.
func rewriteBtweakPaths(s string) string {
	s = strings.ReplaceAll(s, "~/btweak/containers/", "~/berserk/containers/")
	s = strings.ReplaceAll(s, "~/btweak/docker/", "~/berserk/docker/")
	return s
}

// expandRunCmd rewrites btweak-era volume paths to ~/berserk/ and expands ~
// to the absolute home dir. Used right before exec'ing docker.
func expandRunCmd(cmd string) string {
	h, _ := os.UserHomeDir()
	cmd = rewriteBtweakPaths(cmd)
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

// ensureVolumeDirs creates every host volume path with MkdirAll. Paths
// containing $ are skipped — those are shell expansions like $(pwd) or
// $HOME that get resolved by the shell when the docker command runs, and
// we shouldn't materialize a directory literally named "$(pwd)".
//
// One bad path (e.g. -v /etc/passwd:/etc/passwd:ro — a file mount, not a
// directory) must not abort creation of the rest. We collect every failure
// and return a joined error so callers can still warn while letting docker
// attempt the mounts it can.
func ensureVolumeDirs(paths []string) error {
	var errs []error
	for _, p := range paths {
		if strings.Contains(p, "$") {
			continue
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			errs = append(errs, fmt.Errorf("creating volume dir %s: %w", p, err))
		}
	}
	return errors.Join(errs...)
}

// loadDockerGroups reads all *.yaml files from <configDir>/containers/.
// Returns (nil, nil) when the directory does not exist — callers in
// best-effort contexts (search) degrade silently. Real parse/IO errors
// are returned as-is so the caller can surface them.
func loadDockerGroups() ([]docker.Group, error) {
	dir := filepath.Join(configDir(), "containers")
	groups, err := docker.LoadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("docker catalog at %s: %w", dir, err)
	}
	return groups, nil
}

// printContainerDetails prints a container's name, description, pull/run
// commands and runtime notes. Paths are rewritten to ~/berserk/ so the
// user sees the locations berserk will actually use at runtime.
func printContainerDetails(c docker.Container) {
	pterm.Println("  " + pterm.Bold.Sprint(c.Name))
	pterm.DefaultBasicText.Printfln("    %s", pterm.Gray(c.Description))
	pterm.DefaultBasicText.Printfln("    Pull: %s", pterm.Cyan(c.Command))
	pterm.DefaultBasicText.Printfln("    Run:  %s", pterm.Yellow(rewriteBtweakPaths(c.Run)))
	for _, line := range c.RuntimeComments {
		pterm.DefaultBasicText.Printfln("    %s", pterm.Gray(rewriteBtweakPaths(line)))
	}
}
