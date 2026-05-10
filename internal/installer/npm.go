package installer

import "github.com/thehackersbrain/berserk/internal/registry"

func Npm(tool registry.Tool) error {
	pkg := tool.Package
	if pkg == "" {
		pkg = tool.Name
	}
	return runCmd("sudo", "npm", "install", "-g", pkg)
}

func UpdateNpm() error {
	return runCmd("sudo", "npm", "update", "-g")
}
