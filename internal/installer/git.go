package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thehackersbrain/berserk/internal/registry"
)

// GitInstallDir is the root under which all git-installer repos are cloned.
// berserk doctor creates this directory and chowns it to the current user,
// so clone/pull/remove run without sudo.
const GitInstallDir = "/opt/berserk"

func gitCloneURL(tool registry.Tool) string {
	if strings.Contains(tool.Repo, "://") || strings.HasPrefix(tool.Repo, "git@") {
		return tool.Repo
	}
	return "https://github.com/" + tool.Repo
}

func gitDest(name string) string {
	return filepath.Join(GitInstallDir, name)
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
	return runCmd("git", args...)
}

func GitPull(tool registry.Tool) error {
	return runCmd("git", "-C", gitDest(tool.Name), "pull", "--ff-only")
}

func GitRemove(tool registry.Tool) error {
	dest := gitDest(tool.Name)
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("removing %s: %w", dest, err)
	}
	return nil
}
