package installer

import (
	"fmt"

	"github.com/thehackersbrain/berserk/internal/registry"
)

func Pipx(tool registry.Tool) error {
	return pipxInstall(tool, false)
}

// PipxReinstall forces pipx to replace an existing install. Used by Update so
// `berserk update <pipx-tool>` doesn't crash on "already installed".
func PipxReinstall(tool registry.Tool) error {
	return pipxInstall(tool, true)
}

func pipxInstall(tool registry.Tool, force bool) error {
	// Repo always means a GitHub owner/name pair. Package means a PyPI
	// distribution name. If neither is set we fall back to the tool's own
	// name (treated as a PyPI lookup).
	var src string
	switch {
	case tool.Repo != "":
		src = fmt.Sprintf("git+https://github.com/%s", tool.Repo)
	case tool.Package != "":
		src = tool.Package
	default:
		src = tool.Name
	}

	args := []string{"install"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, src)
	if tool.PythonVersion != "" {
		args = append(args, "--python", "python"+tool.PythonVersion)
	}

	return runCmd("pipx", args...)
}

func UpdatePipx() error {
	return runCmd("pipx", "upgrade-all")
}
