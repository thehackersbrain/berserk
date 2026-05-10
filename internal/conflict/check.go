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

// LoadInstalled queries every supported manager once and returns the
// merged snapshot. Errors from any single manager are swallowed — a system
// without `pacman` shouldn't break conflict detection on Debian.
//
// Precedence is first-write-wins per name: if a tool shows up in both pipx
// and the system package list, the manager checked earlier here is
// reported. Order roughly tracks "more likely to be the user's choice".
func LoadInstalled() Snapshot {
	s := Snapshot{}
	addPipx(s)
	addCargo(s)
	addGem(s)
	addPacman(s)
	addDpkg(s)
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

func addPacman(s Snapshot) {
	out, err := exec.Command("pacman", "-Qq").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			if _, ok := s[name]; !ok {
				s[name] = "pacman"
			}
		}
	}
}

func addDpkg(s Snapshot) {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Package} ${Status}\n").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "install ok installed") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			if _, ok := s[fields[0]]; !ok {
				s[fields[0]] = "dpkg"
			}
		}
	}
}
