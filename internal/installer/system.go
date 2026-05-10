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
		return runCmd("sudo", "pacman", "-S", "--noconfirm", "--needed", pkg)
	case distro.Kali, distro.Parrot, distro.Debian:
		pkg := tool.DebianPackage
		if pkg == "" {
			pkg = tool.Name
		}
		return runCmd("sudo", "apt-get", "install", "-y", pkg)
	default:
		return fmt.Errorf("system installer: unsupported distro %s", d)
	}
}

// SystemUpdate refreshes and upgrades the package on Arch (so a plain
// `berserk update <pkg>` actually pulls a newer version), or runs the same
// `apt-get install -y` path on Debian-family distros (apt upgrades in place
// when a newer version is available).
func SystemUpdate(tool registry.Tool, d distro.Distro) error {
	switch d {
	case distro.Arch:
		pkg := tool.ArchPackage
		if pkg == "" {
			pkg = tool.Name
		}
		return runCmd("sudo", "pacman", "-Syu", "--noconfirm", pkg)
	case distro.Kali, distro.Parrot, distro.Debian:
		return System(tool, d)
	default:
		return fmt.Errorf("system installer: unsupported distro %s", d)
	}
}
