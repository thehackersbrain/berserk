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
		return runRoot("pacman", "-S", "--noconfirm", "--needed", pkg)
	case distro.Kali, distro.Parrot, distro.Debian:
		pkg := tool.DebianPackage
		if pkg == "" {
			pkg = tool.Name
		}
		return runRoot("apt-get", "install", "-y", pkg)
	default:
		return fmt.Errorf("system installer: unsupported distro %s", d)
	}
}
