package installer

import (
	"fmt"
	"os"

	"github.com/thehackersbrain/berserk/internal/distro"
	"github.com/thehackersbrain/berserk/internal/registry"
)

// SystemBackend returns the user-facing name of the system package manager
// for d (e.g. "pacman" on Arch, "apt" on Debian-family). Empty string when
// no system fallback is available — callers use that as the signal to
// surface an "unknown tool" error instead of attempting a fallback.
func SystemBackend(d distro.Distro) string {
	switch d {
	case distro.Arch:
		return "pacman"
	case distro.Kali, distro.Parrot, distro.Debian:
		return "apt"
	default:
		return ""
	}
}

// depsFor returns the distro-specific dependency list for tool, or nil if
// none are declared for d. "debian" entries cover Kali and Parrot too —
// they share the apt package universe with Debian proper.
func depsFor(tool registry.Tool, d distro.Distro) []string {
	if len(tool.Depends) == 0 {
		return nil
	}
	switch d {
	case distro.Arch:
		return tool.Depends["arch"]
	case distro.Kali, distro.Parrot, distro.Debian:
		return tool.Depends["debian"]
	default:
		return nil
	}
}

// installDeps installs every system package listed in tool.Depends for the
// current distro before the main installer runs. Batched into a single
// pacman/apt invocation — N subprocess calls would re-resolve the package
// universe N times. Returns nil with no work when no deps are declared.
// assumeYes adds `--noconfirm` (pacman) or `-y` (apt); off by default
// because non-interactive mode hides legitimate conflict prompts.
func installDeps(tool registry.Tool, d distro.Distro, dryRun, assumeYes bool) error {
	deps := depsFor(tool, d)
	if len(deps) == 0 {
		return nil
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would install deps for %s: %v\n", tool.Name, deps)
		return nil
	}

	fmt.Fprintf(os.Stderr, "  Installing deps for %s: %v\n", tool.Name, deps)
	switch d {
	case distro.Arch:
		args := []string{"pacman", "-S", "--needed"}
		if assumeYes {
			args = append(args, "--noconfirm")
		}
		args = append(args, deps...)
		return runPkgMgrCmd(assumeYes, args...)
	case distro.Kali, distro.Parrot, distro.Debian:
		args := []string{"apt", "install"}
		if assumeYes {
			args = append(args, "-y")
		}
		args = append(args, deps...)
		return runPkgMgrCmd(assumeYes, args...)
	default:
		// depsFor returned non-nil for an unknown distro — shouldn't happen,
		// but surface it loudly rather than silently skipping the user's deps.
		return fmt.Errorf("cannot install deps on distro %s", d)
	}
}
