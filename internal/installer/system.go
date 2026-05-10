package installer

import (
	"fmt"

	"github.com/thehackersbrain/berserk/internal/distro"
	"github.com/thehackersbrain/berserk/internal/registry"
)

func System(tool registry.Tool, d distro.Distro, assumeYes bool) error {
	switch d {
	case distro.Arch:
		pkg := tool.ArchPackage
		if pkg == "" {
			pkg = tool.Name
		}
		args := []string{"pacman", "-S", "--needed"}
		if assumeYes {
			args = append(args, "--noconfirm")
		}
		args = append(args, pkg)
		return runPkgMgrCmd(assumeYes, args...)
	case distro.Kali, distro.Parrot, distro.Debian:
		pkg := tool.DebianPackage
		if pkg == "" {
			pkg = tool.Name
		}
		args := []string{"apt", "install"}
		if assumeYes {
			args = append(args, "-y")
		}
		args = append(args, pkg)
		return runPkgMgrCmd(assumeYes, args...)
	default:
		return fmt.Errorf("system installer: unsupported distro %s", d)
	}
}

// SystemUpdate makes `berserk update <system-tool>` actually pull a newer
// version: on Arch we refresh + upgrade just that package via `pacman -Syu`;
// on Debian-family distros we refresh the apt index once per process, then
// re-run the install (apt upgrades in place when the index has a newer
// version).
func SystemUpdate(tool registry.Tool, d distro.Distro, assumeYes bool) error {
	switch d {
	case distro.Arch:
		pkg := tool.ArchPackage
		if pkg == "" {
			pkg = tool.Name
		}
		args := []string{"pacman", "-Syu"}
		if assumeYes {
			args = append(args, "--noconfirm")
		}
		args = append(args, pkg)
		return runPkgMgrCmd(assumeYes, args...)
	case distro.Kali, distro.Parrot, distro.Debian:
		if err := ensureAptIndex(assumeYes); err != nil {
			return fmt.Errorf("refreshing apt index: %w", err)
		}
		return System(tool, d, assumeYes)
	default:
		return fmt.Errorf("system installer: unsupported distro %s", d)
	}
}
