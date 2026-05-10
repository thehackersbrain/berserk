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
