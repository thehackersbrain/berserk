package cmd

import (
	"os/exec"

	"github.com/thehackersbrain/berserk/internal/registry"
	"github.com/thehackersbrain/berserk/internal/state"
)

// installedSet decides whether each tool counts as installed. The state file
// is the source of truth — we only record entries on confirmed install
// success, so it doesn't suffer from the impacket-style problem where the
// catalog name doesn't match any binary on PATH. PATH lookup is kept as a
// fallback for tools the user installed before berserk knew about them.
//
// st may be nil; in that case only the PATH heuristic applies.
func installedSet(tools []registry.Tool, st *state.State) map[string]bool {
	out := make(map[string]bool, len(tools))
	for _, t := range tools {
		if st != nil && st.Has(t.Name) {
			out[t.Name] = true
			continue
		}
		if _, err := exec.LookPath(t.Name); err == nil {
			out[t.Name] = true
		}
	}
	return out
}

// loadStateOrWarn loads state and surfaces a non-fatal warning on failure.
// Read paths shouldn't abort just because the state file is unreadable —
// PATH fallback still gives a usable (if less accurate) answer.
func loadStateOrWarn() *state.State {
	st, err := state.Load()
	if err != nil {
		printWarn("could not load install state: %v (falling back to PATH lookup)", err)
		return nil
	}
	return st
}
