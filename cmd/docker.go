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
	"gopkg.in/yaml.v3"
)

// dockerDataDir returns the absolute root for container volume mounts —
// `docker_data_dir` from <cfgDir>/config.yaml if set, else ~/berserk.
// Used by --docker-clean to know what to delete.
//
// cfgDir is passed through (rather than calling configDir() internally) so
// the dockerDataDir → configDir → rootCmd → runDockerClean → dockerDataDir
// initialization cycle stays broken.
func dockerDataDir(cfgDir string) string {
	return expandTilde(displayDockerDataDir(cfgDir))
}

// displayDockerDataDir returns the literal root path — "~/"-form preserved
// when the config used it, default "~/berserk" otherwise. Used by
// rewriteBtweakPaths so display lines stay tilde-pretty rather than
// echoing $HOME at the user.
func displayDockerDataDir(cfgDir string) string {
	if raw := readDockerDataDirOverride(cfgDir); raw != "" {
		return raw
	}
	return "~/berserk"
}

// readDockerDataDirOverride reads `docker_data_dir` from <cfgDir>/config.yaml.
// Returns "" when unset, missing, or unparseable — caller falls back to default.
func readDockerDataDirOverride(cfgDir string) string {
	data, err := os.ReadFile(filepath.Join(cfgDir, "config.yaml"))
	if err != nil {
		return ""
	}
	var cfg struct {
		DockerDataDir string `yaml:"docker_data_dir"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.DockerDataDir)
}

// expandTilde resolves a leading "~/" against $HOME. Absolute paths and
// paths without a leading tilde pass through unchanged.
func expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		h, err := os.UserHomeDir()
		if err != nil || h == "" {
			return p
		}
		if p == "~" {
			return h
		}
		return filepath.Join(h, p[2:])
	}
	return p
}

// rewriteBtweakPaths replaces ~/btweak/{containers,docker}/ with the
// configured berserk data dir's matching subdirs. Used for display so the
// user sees where data actually lives without expanding ~ to an absolute
// path when the config uses "~/" form.
func rewriteBtweakPaths(cfgDir, s string) string {
	root := displayDockerDataDir(cfgDir)
	s = strings.ReplaceAll(s, "~/btweak/containers/", root+"/containers/")
	s = strings.ReplaceAll(s, "~/btweak/docker/", root+"/docker/")
	return s
}

// expandRunCmd rewrites btweak-era volume paths to the configured data dir
// and expands a leading ~/ to the absolute home. Used right before exec'ing
// docker. cfgDir threads through to keep the call chain off configDir().
func expandRunCmd(cfgDir, cmd string) string {
	h, _ := os.UserHomeDir()
	cmd = rewriteBtweakPaths(cfgDir, cmd)
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
// commands and runtime notes. Paths are rewritten via the configured
// docker_data_dir (default ~/berserk) so the user sees real locations.
func printContainerDetails(cfgDir string, c docker.Container) {
	pterm.Println("  " + pterm.Bold.Sprint(c.Name))
	pterm.DefaultBasicText.Printfln("    %s", pterm.Gray(c.Description))
	pterm.DefaultBasicText.Printfln("    Pull: %s", pterm.Cyan(c.Command))
	pterm.DefaultBasicText.Printfln("    Run:  %s", pterm.Yellow(rewriteBtweakPaths(cfgDir, c.Run)))
	for _, line := range c.RuntimeComments {
		pterm.DefaultBasicText.Printfln("    %s", pterm.Gray(rewriteBtweakPaths(cfgDir, line)))
	}
}
