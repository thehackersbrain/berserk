package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thehackersbrain/berserk/internal/registry"
)

const (
	// GitInstallDir is the root under which repos are cloned.
	// berserk doctor creates this and chowns it to the current user.
	GitInstallDir = "/opt/berserk"
	// GitVenvDir holds per-tool Python virtualenvs for entry_script tools.
	GitVenvDir = "/opt/berserk/venvs"
	// GitBinDir holds bash shims that invoke scripts inside their venvs.
	// berserk doctor creates this and adds it to PATH.
	GitBinDir = "/opt/berserk/bin"
)

func gitCloneURL(tool registry.Tool) string {
	if strings.Contains(tool.Repo, "://") || strings.HasPrefix(tool.Repo, "git@") {
		return tool.Repo
	}
	return "https://github.com/" + tool.Repo
}

func gitDest(name string) string {
	return filepath.Join(GitInstallDir, name)
}

func gitVenvPath(name string) string {
	return filepath.Join(GitVenvDir, name)
}

func gitShimPath(name string) string {
	return filepath.Join(GitBinDir, name)
}

func GitClone(tool registry.Tool) error {
	dest := gitDest(tool.Name)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already cloned at %s; run update to pull latest", tool.Name, dest)
	}
	args := []string{"clone"}
	if tool.Branch != "" {
		args = append(args, "--branch", tool.Branch)
	}
	args = append(args, gitCloneURL(tool), dest)
	if err := runCmd("git", args...); err != nil {
		return err
	}
	if tool.EntryScript != "" && tool.Runtime == "python" {
		return setupPythonRuntime(tool)
	}
	return nil
}

func GitPull(tool registry.Tool) error {
	if err := runCmd("git", "-C", gitDest(tool.Name), "pull", "--ff-only"); err != nil {
		return err
	}
	if tool.EntryScript != "" && tool.Runtime == "python" {
		return refreshPythonDeps(tool)
	}
	return nil
}

func GitRemove(tool registry.Tool) error {
	dest := gitDest(tool.Name)
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("removing %s: %w", dest, err)
	}
	if tool.EntryScript != "" {
		_ = os.RemoveAll(gitVenvPath(tool.Name))
		_ = os.Remove(gitShimPath(tool.Name))
	}
	return nil
}

// setupPythonRuntime creates the venv, installs deps (additive: requirements.txt
// then pip_deps), and writes the bash shim to GitBinDir.
func setupPythonRuntime(tool registry.Tool) error {
	venv := gitVenvPath(tool.Name)
	dest := gitDest(tool.Name)

	if err := runCmd("python3", "-m", "venv", venv); err != nil {
		return fmt.Errorf("creating venv for %s: %w", tool.Name, err)
	}

	if err := refreshPythonDeps(tool); err != nil {
		return err
	}

	python := filepath.Join(venv, "bin", "python")
	script := filepath.Join(dest, tool.EntryScript)
	shim := fmt.Sprintf("#!/usr/bin/env bash\nexec %s %s \"$@\"\n", python, script)

	if err := os.MkdirAll(GitBinDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", GitBinDir, err)
	}
	if err := os.WriteFile(gitShimPath(tool.Name), []byte(shim), 0o755); err != nil {
		return fmt.Errorf("writing shim for %s: %w", tool.Name, err)
	}
	return nil
}

// refreshPythonDeps installs/updates deps into the existing venv. Runs both
// requirements.txt (if present) and pip_deps (if any) — they are additive.
func refreshPythonDeps(tool registry.Tool) error {
	pip := filepath.Join(gitVenvPath(tool.Name), "bin", "pip")

	reqsFile := filepath.Join(gitDest(tool.Name), "requirements.txt")
	if _, err := os.Stat(reqsFile); err == nil {
		if err := runCmd(pip, "install", "-r", reqsFile); err != nil {
			return fmt.Errorf("pip install -r requirements.txt for %s: %w", tool.Name, err)
		}
	}

	if len(tool.PipDeps) > 0 {
		args := append([]string{"install"}, tool.PipDeps...)
		if err := runCmd(pip, args...); err != nil {
			return fmt.Errorf("pip install deps for %s: %w", tool.Name, err)
		}
	}

	return nil
}
