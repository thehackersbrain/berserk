package installer

import (
	"github.com/thehackersbrain/berserk/internal/registry"
)

// CustomInstall runs the tool's install command through the system shell.
func CustomInstall(tool registry.Tool) error {
	for _, i := range tool.Steps {
		if err := runCmd("sh", "-c", i); err != nil {
			return err
		}
	}

	return nil
}
