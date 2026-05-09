package installer

import "github.com/thehackersbrain/berserk/internal/registry"

func Gem(tool registry.Tool) error {
	pkg := tool.Package
	if pkg == "" {
		pkg = tool.Name
	}
	return runCmd("gem", "install", pkg)
}

// GemUpgrade upgrades a single gem to the latest version. `gem install`
// won't bump an installed gem's version, so `update` is needed for the
// per-tool update path.
func GemUpgrade(tool registry.Tool) error {
	pkg := tool.Package
	if pkg == "" {
		pkg = tool.Name
	}
	return runCmd("gem", "update", pkg)
}

func UpdateGem() error {
	return runCmd("gem", "update")
}
