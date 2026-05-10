package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/thehackersbrain/berserk/internal/distro"
	"github.com/thehackersbrain/berserk/internal/registry"
)

type Options struct {
	InstallDir  string
	GithubToken string
	DryRun      bool
	Verbose     bool
}

// SetVerbose should be called once before any Install calls begin, not per-tool.
func SetVerbose(v bool) { Verbose = v }

func Install(tool registry.Tool, d distro.Distro, opts Options) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] would install %s via %s\n", tool.Name, tool.Installer)
		return nil
	}

	switch tool.Installer {
	case "pipx":
		return Pipx(tool)
	case "cargo":
		return Cargo(tool)
	case "go":
		return Go(tool)
	case "gem":
		return Gem(tool)
	case "npm":
		return Npm(tool)
	case "binary":
		return Binary(tool, opts.InstallDir, opts.GithubToken)
	case "system":
		return System(tool, d)
	case "custom":
		return CustomInstall(tool)
	default:
		return fmt.Errorf("unknown installer: %s", tool.Installer)
	}
}

// Update upgrades an already-installed tool. Most installers' Install paths
// are idempotent (go install replaces, cargo install is a no-op or upgrade,
// binary refetches latest). pipx, gem, and system need explicit upgrade
// commands — `pipx install` errors on "already installed", `gem install`
// won't bump versions, and `pacman -S --needed` skips packages already
// present.
func Update(tool registry.Tool, d distro.Distro, opts Options) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] would update %s via %s\n", tool.Name, tool.Installer)
		return nil
	}
	switch tool.Installer {
	case "pipx":
		return PipxReinstall(tool)
	case "gem":
		return GemUpgrade(tool)
	case "system":
		return SystemUpdate(tool, d)
	case "custom":
		// Re-running the install script is the standard upgrade path.
		return CustomInstall(tool)
	default:
		return Install(tool, d, opts)
	}
}

func Remove(tool registry.Tool, d distro.Distro) error {
	switch tool.Installer {
	case "pipx":
		return runCmd("pipx", "uninstall", tool.Name)

	case "cargo":
		return runCmd("cargo", "uninstall", tool.Name)

	case "go":
		return removeGoBinary(tool.Name)

	case "gem":
		pkg := tool.Package
		if pkg == "" {
			pkg = tool.Name
		}
		return runCmd("gem", "uninstall", pkg)

	case "npm":
		pkg := tool.Package
		if pkg == "" {
			pkg = tool.Name
		}
		return runCmd("npm", "uninstall", "-g", pkg)

	case "binary":
		// Find via PATH and delete
		path, err := exec.LookPath(tool.Name)
		if err != nil {
			return fmt.Errorf("%s not found in PATH", tool.Name)
		}
		return os.Remove(path)

	case "system":
		switch d {
		case distro.Arch:
			pkg := tool.ArchPackage
			if pkg == "" {
				pkg = tool.Name
			}
			// -Rns: also remove unused dependencies (-s) and skip saving
			// pacman backup files for the package's config (-n).
			return runCmd("sudo", "pacman", "-Rns", "--noconfirm", pkg)
		case distro.Kali, distro.Parrot, distro.Debian:
			pkg := tool.DebianPackage
			if pkg == "" {
				pkg = tool.Name
			}
			return runCmd("sudo", "apt-get", "remove", "-y", pkg)
		}

	case "custom":
		return fmt.Errorf("%s was installed via a custom script; remove it manually", tool.Name)
	}
	return fmt.Errorf("cannot remove %s (installer: %s)", tool.Name, tool.Installer)
}

func removeGoBinary(name string) error {
	// Prefer GOBIN, then GOPATH/bin, then ~/go/bin
	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				home = os.Getenv("HOME")
			}
			gopath = filepath.Join(home, "go")
		}
		gobin = filepath.Join(gopath, "bin")
	}

	target := filepath.Join(gobin, name)
	if err := os.Remove(target); err == nil {
		return nil
	}

	// Fall back to PATH lookup
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found in GOBIN (%s) or PATH", name, gobin)
	}
	return os.Remove(path)
}
