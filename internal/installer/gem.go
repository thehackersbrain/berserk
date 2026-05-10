package installer

import "github.com/thehackersbrain/berserk/internal/registry"

func Gem(tool registry.Tool) error {
	pkg := tool.Package
	if pkg == "" {
		pkg = tool.Name
	}
	return runCmd("gem", "install", "--user-install", pkg)
}

func GemUpgrade(tool registry.Tool) error {
	pkg := tool.Package
	if pkg == "" {
		pkg = tool.Name
	}
	return runCmd("gem", "update", "--user-install", pkg)
}
