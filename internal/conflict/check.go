// Package conflict detects whether a tool is already installed by some
// other package manager so the install command can warn the user before
// overwriting/duplicating it.
package conflict

import (
	"os/exec"
	"strings"
)

// Snapshot maps a tool name to the manager that already owns it. Building
// it once and consulting it via Check avoids spawning O(installers × tools)
// subprocesses during a parallel install.
type Snapshot map[string]string

// Check reports the manager name that owns the tool, or "" if none does.
// A nil receiver is safe and reports "".
func (s Snapshot) Check(name string) string {
	if s == nil {
		return ""
	}
	return s[name]
}

// LoadInstalled queries every supported source-based manager once and
// returns the merged snapshot. Errors from any single manager are
// swallowed — a system without `pipx` shouldn't break conflict detection
// on a system that has cargo or gem.
//
// We deliberately don't check dpkg or pacman here: berserk focuses on
// source-based installs, and the OS package universe is too broad to give
// meaningful "already installed via X" warnings without a flood of false
// positives (every system has hundreds of OS-managed packages, almost
// none of them tools the user wants berserk to manage).
//
// Precedence is first-write-wins per name.
func LoadInstalled() Snapshot {
	s := Snapshot{}
	addPipx(s)
	addCargo(s)
	addGem(s)
	return s
}

func addPipx(s Snapshot) {
	out, err := exec.Command("pipx", "list", "--short").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			if _, ok := s[fields[0]]; !ok {
				s[fields[0]] = "pipx"
			}
		}
	}
}

func addCargo(s Snapshot) {
	out, err := exec.Command("cargo", "install", "--list").Output()
	if err != nil {
		return
	}
	// cargo install --list output:
	//   crate-name v0.1.0:
	//       binary-1
	//       binary-2
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[1], "v") {
			name := fields[0]
			if _, ok := s[name]; !ok {
				s[name] = "cargo"
			}
		}
	}
}

func addGem(s Snapshot) {
	out, err := exec.Command("gem", "list").Output()
	if err != nil {
		return
	}
	// gem list output: "name (1.0.0, 2.0.0)"
	for _, line := range strings.Split(string(out), "\n") {
		if idx := strings.IndexByte(line, ' '); idx > 0 {
			name := line[:idx]
			if _, ok := s[name]; !ok {
				s[name] = "gem"
			}
		}
	}
}

