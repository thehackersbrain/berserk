package installer

import (
	"fmt"

	"github.com/thehackersbrain/berserk/internal/distro"
	"github.com/thehackersbrain/berserk/internal/registry"
)

func System(tool registry.Tool, d distro.Distro) error {
	switch d {
	case distro.Arch:
		pkg := tool.ArchPackage
		if pkg == "" {
			pkg = tool.Name
		}
		return runCmd("sudo", "pacman", "-S", "--needed", pkg)
	case distro.Kali, distro.Parrot, distro.Debian:
		pkg := tool.DebianPackage
		if pkg == "" {
			pkg = tool.Name
		}
		return runCmd("sudo", "apt", "install", pkg)
	default:
		return fmt.Errorf("system installer: unsupported distro %s", d)
	}
}

// SystemUpdate makes `berserk update <system-tool>` actually pull a newer
// version: on Arch we refresh + upgrade just that package via `pacman -Syu`;
// on Debian-family distros we refresh the apt index first, then re-run the
// install (apt upgrades in place when the index has a newer version).
func SystemUpdate(tool registry.Tool, d distro.Distro) error {
	switch d {
	case distro.Arch:
		pkg := tool.ArchPackage
		if pkg == "" {
			pkg = tool.Name
		}
		return runCmd("sudo", "pacman", "-Syu", pkg)
	case distro.Kali, distro.Parrot, distro.Debian:
		if err := runCmd("sudo", "apt", "update"); err != nil {
			return err
		}
		return System(tool, d)
	default:
		return fmt.Errorf("system installer: unsupported distro %s", d)
	}
}
