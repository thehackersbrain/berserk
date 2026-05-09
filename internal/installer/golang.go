package installer

import (
	"fmt"
	"strings"

	"github.com/thehackersbrain/berserk/internal/registry"
)

func goBuildPkg(tool registry.Tool) string {
	if tool.Package != "" {
		if !strings.Contains(tool.Package, "@") {
			return tool.Package + "@latest"
		}
		return tool.Package
	}
	pkg := fmt.Sprintf("github.com/%s", tool.Repo)
	if !strings.Contains(pkg, "@") {
		pkg += "@latest"
	}
	return pkg
}

func Go(tool registry.Tool) error {
	if tool.Package == "" && tool.Repo == "" {
		return fmt.Errorf("go installer: no repo or package defined for %s", tool.Name)
	}
	return runCmd("go", "install", goBuildPkg(tool))
}
