package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/thehackersbrain/berserk/internal/docker"
)

var (
	runTerminal bool
	runFlags    string
)

var runCmd = &cobra.Command{
	Use:   "run <container>",
	Short: "Run a Docker container from the catalog",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		groups, err := loadDockerGroups()
		if err != nil {
			return err
		}
		if len(groups) == 0 {
			return fmt.Errorf("no Docker catalog found at %s/containers — run `berserk sync` or check --config", configDir())
		}

		results := docker.Search(groups, args[0])
		if len(results) == 0 {
			return fmt.Errorf("no container matched %q", args[0])
		}

		// Prefer an exact name match when the substring search returns
		// multiple hits — `berserk run nmap` should run the container
		// literally named "nmap" even if "nmap-scan" also matches.
		var picked *docker.SearchResult
		for i := range results {
			if results[i].Container.Name == args[0] {
				picked = &results[i]
				break
			}
		}
		if picked == nil {
			if len(results) > 1 {
				pterm.Warning.Printfln("Multiple containers match %q — be more specific:", args[0])
				for _, r := range results {
					pterm.DefaultBasicText.Printfln("  %s", pterm.Green(r.Container.Name))
				}
				return fmt.Errorf("ambiguous: %d matches", len(results))
			}
			picked = &results[0]
		}

		c := picked.Container
		if strings.TrimSpace(c.Run) == "" {
			return fmt.Errorf("container %q has empty run command in catalog", c.Name)
		}
		runStr := expandRunCmd(c.Run)

		if runFlags != "" {
			// strings.Replace silently returns the input when the pattern
			// isn't present — that would drop the user's --flags without a
			// peep. Detect the no-op and error out instead.
			newStr := strings.Replace(runStr, "docker run", "docker run "+runFlags, 1)
			if newStr == runStr {
				return fmt.Errorf("cannot inject --flags: container %q run command has no literal 'docker run' to insert after", c.Name)
			}
			runStr = newStr
		}

		if len(c.RuntimeComments) > 0 {
			pterm.Println()
			pterm.DefaultBasicText.Println(pterm.Bold.Sprint("=== Runtime Information ==="))
			for _, line := range c.RuntimeComments {
				pterm.DefaultBasicText.Println("  " + pterm.Green(expandRunCmd(line)))
			}
			pterm.DefaultBasicText.Println(pterm.Bold.Sprint("==========================="))
			pterm.Println()
		}

		pterm.DefaultBasicText.Printfln("%s %s", pterm.Bold.Sprint(">>>"), pterm.Yellow(runStr))
		pterm.Println()

		// MkdirAll non-shell-expanded volume paths so docker can mount them.
		// Shell-expanded volumes ($(pwd), $HOME/...) are handled by sh below.
		if err := ensureVolumeDirs(extractVolumes(runStr)); err != nil {
			pterm.Warning.Printfln("could not create volume dirs: %v", err)
		}

		// Exec through /bin/sh so shell constructs in the catalog work:
		// $(pwd), $DISPLAY, $HOME, quoted arg values, etc. btweak relied
		// on Python's shell=True for the same reason.
		if runTerminal {
			// tmux runs its command argument via the user's shell when
			// given as a single string, so $(pwd) etc. expand correctly.
			return exec.Command("kitty", "--hold", "tmux", "new-session", runStr).Start()
		}
		return syscall.Exec("/bin/sh", []string{"sh", "-c", runStr}, os.Environ())
	},
}

func init() {
	runCmd.Flags().BoolVarP(&runTerminal, "terminal", "t", false, "run in a new kitty terminal window")
	runCmd.Flags().StringVarP(&runFlags, "flags", "f", "", "extra flags to pass to docker run")
	rootCmd.AddCommand(runCmd)
}
