package conflict

import (
	"os/exec"
	"strings"
)

// Check returns the manager name if the tool is already installed by a
// recognised package manager, or empty string otherwise. A bare PATH match
// is NOT a conflict — many tools ship as part of base distros.
func Check(toolName string) string {
	if mgr := identifyManager(toolName); mgr != "" {
		return mgr
	}
	return ""
}

func identifyManager(name string) string {
	if out, err := exec.Command("pipx", "list", "--short").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] == name {
				return "pipx"
			}
		}
	}

	if out, err := exec.Command("pacman", "-Qq", name).Output(); err == nil {
		if strings.TrimSpace(string(out)) == name {
			return "pacman"
		}
	}

	if out, err := exec.Command("dpkg-query", "-W", "-f=${Status}", name).Output(); err == nil {
		if strings.Contains(string(out), "install ok installed") {
			return "dpkg"
		}
	}

	if out, err := exec.Command("gem", "list", "-i", name).Output(); err == nil {
		if strings.TrimSpace(string(out)) == "true" {
			return "gem"
		}
	}

	if out, err := exec.Command("cargo", "install", "--list").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, name+" v") {
				return "cargo"
			}
		}
	}

	return ""
}
