package installer

import (
	"fmt"
	"os"
	"strings"

	"github.com/thehackersbrain/berserk/internal/registry"
)

// CustomInstall runs the tool's install steps through the system shell.
func CustomInstall(tool registry.Tool) error {
	for i, step := range tool.Steps {
		fmt.Fprintf(os.Stderr, "\n[custom step %d/%d] %s\n", i+1, len(tool.Steps), step)
		if sudoLikely(step) {
			fmt.Fprintf(os.Stderr, "[sudo] this step may prompt for your sudo password\n")
		}
		if err := runCmd("sh", "-c", step); err != nil {
			return err
		}
	}
	return nil
}

// sudoLikely returns true when a shell step is likely to trigger a sudo
// password prompt — either because it calls sudo directly, or because it
// pipes into a shell (curl | bash, bash <(...)) which may call sudo
// internally without berserk being able to see it.
func sudoLikely(step string) bool {
	lower := strings.ToLower(step)
	for _, pat := range []string{"sudo", "| bash", "| sh", "| zsh", "bash <(", "sh <("} {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}
