package installer

import (
	"github.com/thehackersbrain/berserk/internal/registry"
)

// Oneliner runs the tool's install command through the system shell.
// These are typically vendor-supplied scripts like `curl https://… | sudo bash`.
func Oneliner(tool registry.Tool) error {
	return runCmd("sh", "-c", tool.Oneliner)
}
