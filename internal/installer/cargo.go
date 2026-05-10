package installer

import "github.com/thehackersbrain/berserk/internal/registry"

func Cargo(tool registry.Tool) error {
	pkg := tool.Package
	if pkg == "" {
		pkg = tool.Name
	}
	return runCmd("cargo", "install", pkg)
}

// CargoReinstall forces cargo to rebuild an already-installed crate. Plain
// `cargo install` is a no-op when the crate is already present, so without
// --force `berserk update <cargo-tool>` would never actually upgrade.
func CargoReinstall(tool registry.Tool) error {
	pkg := tool.Package
	if pkg == "" {
		pkg = tool.Name
	}
	return runCmd("cargo", "install", "--force", pkg)
}

func UpdateCargo() error {
	// cargo-update crate provides install-update
	return runCmd("cargo", "install-update", "-a")
}
