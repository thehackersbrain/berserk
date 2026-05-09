// Package installer -- handles all the install actions
package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Verbose controls whether commands are echoed to stderr before running.
var Verbose bool

// UseSudo when true, prepends `sudo` to runRoot calls if the process is not root.
var UseSudo = true

func runCmd(name string, args ...string) error {
	if Verbose {
		fmt.Fprintf(os.Stderr, "  $ %s %s\n", name, strings.Join(args, " "))
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}

// runRoot runs a command, prepending sudo if needed and available.
func runRoot(name string, args ...string) error {
	if os.Geteuid() == 0 {
		return runCmd(name, args...)
	}
	if !UseSudo {
		return fmt.Errorf("%s requires root; rerun as root or set use_sudo=true in config", name)
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("%s requires root and sudo is not available", name)
	}
	return runCmd("sudo", append([]string{name}, args...)...)
}

// canWrite reports whether the current user can write to dir.
func canWrite(dir string) bool {
	tmp, err := os.CreateTemp(dir, ".berserk-probe-*")
	if err != nil {
		return false
	}
	tmp.Close()           //nolint:errcheck
	os.Remove(tmp.Name()) //nolint:errcheck
	return true
}
