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
	// GitVenvDir holds per-tool Python virtualenvs for entry_scripts tools.
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

// shimName returns the shim filename for a script: strips the file extension.
// "targetedKerberoast.py" → "targetedKerberoast"
func shimName(script string) string {
	return strings.TrimSuffix(script, filepath.Ext(script))
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
	if len(tool.EntryScripts) > 0 && tool.Runtime == "python" {
		return setupPythonRuntime(tool)
	}
	return nil
}

func GitPull(tool registry.Tool) error {
	if err := runCmd("git", "-C", gitDest(tool.Name), "pull", "--ff-only"); err != nil {
		return err
	}
	if len(tool.EntryScripts) > 0 && tool.Runtime == "python" {
		return refreshPythonDeps(tool)
	}
	return nil
}

func GitRemove(tool registry.Tool) error {
	dest := gitDest(tool.Name)
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("removing %s: %w", dest, err)
	}
	if len(tool.EntryScripts) > 0 {
		_ = os.RemoveAll(gitVenvPath(tool.Name))
		for _, script := range tool.EntryScripts {
			_ = os.Remove(gitShimPath(shimName(script)))
		}
	}
	return nil
}

// setupPythonRuntime creates the venv, installs deps (additive: requirements.txt
// then pip_deps), and writes one bash shim per entry_scripts entry.
func setupPythonRuntime(tool registry.Tool) error {
	venv := gitVenvPath(tool.Name)

	if err := runCmd("python3", "-m", "venv", venv); err != nil {
		return fmt.Errorf("creating venv for %s: %w", tool.Name, err)
	}

	if err := refreshPythonDeps(tool); err != nil {
		return err
	}

	if err := os.MkdirAll(GitBinDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", GitBinDir, err)
	}

	python := filepath.Join(venv, "bin", "python")
	dest := gitDest(tool.Name)

	for _, script := range tool.EntryScripts {
		scriptPath := filepath.Join(dest, script)
		shim := fmt.Sprintf("#!/usr/bin/env bash\nexec %s %s \"$@\"\n", python, scriptPath)
		shimPath := gitShimPath(shimName(script))
		if err := os.WriteFile(shimPath, []byte(shim), 0o755); err != nil {
			return fmt.Errorf("writing shim for %s: %w", script, err)
		}
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
