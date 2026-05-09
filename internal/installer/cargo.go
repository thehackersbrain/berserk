package installer

import "github.com/thehackersbrain/berserk/internal/registry"

func Cargo(tool registry.Tool) error {
	pkg := tool.Package
	if pkg == "" {
		pkg = tool.Name
	}
	return runCmd("cargo", "install", pkg)
}

func UpdateCargo() error {
	// cargo-update crate provides install-update
	return runCmd("cargo", "install-update", "-a")
}
