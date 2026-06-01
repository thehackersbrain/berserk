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
	// AssumeYes routes `--yes` through to the underlying system package
	// managers as `--noconfirm` (pacman) or `-y` (apt). Off by default —
	// non-interactive mode masks legitimate prompts (replace-on-conflict,
	// downgrade confirmations) that the user often needs to see.
	AssumeYes bool
}

// SetVerbose toggles the package-level Verbose flag that runCmd reads.
// Call once at startup before any Install/Update/Remove invocation —
// it's not threaded through Options because verbosity isn't a per-tool
// decision.
func SetVerbose(v bool) { Verbose = v }

func Install(tool registry.Tool, d distro.Distro, opts Options) error {
	if opts.DryRun {
		if err := installDeps(tool, d, true, opts.AssumeYes); err != nil {
			return err
		}
		fmt.Printf("[dry-run] would install %s via %s\n", tool.Name, tool.Installer)
		return nil
	}

	if err := installDeps(tool, d, false, opts.AssumeYes); err != nil {
		return fmt.Errorf("installing deps for %s: %w", tool.Name, err)
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
	case "git":
		return GitClone(tool)
	case "system":
		return System(tool, d, opts.AssumeYes)
	case "custom":
		return CustomInstall(tool)
	default:
		return fmt.Errorf("unknown installer: %s", tool.Installer)
	}
}

// Update upgrades an already-installed tool. Some installers' Install paths
// are idempotent on upgrade (go install replaces, binary refetches latest);
// pipx, cargo, gem, and system need explicit upgrade commands — `pipx install`
// errors on "already installed", `cargo install` skips already-installed
// crates without --force, `gem install` won't bump versions, and `pacman -S
// --needed` skips packages already present.
func Update(tool registry.Tool, d distro.Distro, opts Options) error {
	if opts.DryRun {
		if err := installDeps(tool, d, true, opts.AssumeYes); err != nil {
			return err
		}
		fmt.Printf("[dry-run] would update %s via %s\n", tool.Name, tool.Installer)
		return nil
	}
	// Re-run deps before update so a tool that grew a new dependency in the
	// catalog gets it on next `berserk update <tool>`. pacman --needed and
	// apt are idempotent on already-present packages.
	if err := installDeps(tool, d, false, opts.AssumeYes); err != nil {
		return fmt.Errorf("installing deps for %s: %w", tool.Name, err)
	}
	switch tool.Installer {
	case "pipx":
		return PipxReinstall(tool)
	case "cargo":
		return CargoReinstall(tool)
	case "gem":
		return GemUpgrade(tool)
	case "git":
		return GitPull(tool)
	case "system":
		return SystemUpdate(tool, d, opts.AssumeYes)
	case "custom":
		// Re-running the install script is the standard upgrade path.
		return CustomInstall(tool)
	default:
		return Install(tool, d, opts)
	}
}

func Remove(tool registry.Tool, d distro.Distro, opts Options) error {
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
		return removeBinary(tool.Name, opts.InstallDir)

	case "system":
		switch d {
		case distro.Arch:
			pkg := tool.ArchPackage
			if pkg == "" {
				pkg = tool.Name
			}
			// -Rns: also remove unused dependencies (-s) and skip saving
			// pacman backup files for the package's config (-n).
			args := []string{"pacman", "-Rns"}
			if opts.AssumeYes {
				args = append(args, "--noconfirm")
			}
			args = append(args, pkg)
			return runPkgMgrCmd(opts.AssumeYes, args...)
		case distro.Kali, distro.Parrot, distro.Debian:
			pkg := tool.DebianPackage
			if pkg == "" {
				pkg = tool.Name
			}
			args := []string{"apt", "remove"}
			if opts.AssumeYes {
				args = append(args, "-y")
			}
			args = append(args, pkg)
			return runPkgMgrCmd(opts.AssumeYes, args...)
		default:
			return fmt.Errorf("system installer: unsupported distro %s", d)
		}

	case "git":
		return GitRemove(tool)

	case "custom":
		return fmt.Errorf("%s was installed via a custom script; remove it manually", tool.Name)
	}
	return fmt.Errorf("cannot remove %s (installer: %s)", tool.Name, tool.Installer)
}

// removeBinary deletes a binary-installer artifact. Prefer the staged path
// at <installDir>/<name>; fall back to PATH only when the resolved entry
// lives under installDir. Without that gate, a system-installed binary of
// the same name (apt, brew, etc.) shadowing ours on PATH would get deleted.
func removeBinary(name, installDir string) error {
	if installDir == "" {
		installDir = "/usr/local/bin"
	}
	target := filepath.Join(installDir, name)
	if err := os.Remove(target); err == nil {
		return nil
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found in installDir (%s) or PATH", name, installDir)
	}
	if filepath.Clean(filepath.Dir(path)) != filepath.Clean(installDir) {
		return fmt.Errorf("%s on PATH at %s is not under installDir (%s); refusing to remove", name, path, installDir)
	}
	return os.Remove(path)
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

	// PATH fallback only fires for binaries that resolve back to a
	// Go-managed directory. Without that gate, a system-installed binary
	// of the same name (apt, brew, etc.) would get deleted instead.
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found in GOBIN (%s) or PATH", name, gobin)
	}
	if filepath.Clean(filepath.Dir(path)) != filepath.Clean(gobin) {
		return fmt.Errorf("%s on PATH at %s is not under GOBIN (%s); refusing to remove", name, path, gobin)
	}
	return os.Remove(path)
}
