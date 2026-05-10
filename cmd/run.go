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

		results := docker.Search(groups, args[0])
		switch len(results) {
		case 0:
			return fmt.Errorf("no container matched %q", args[0])
		case 1:
			// continue
		default:
			pterm.Warning.Printfln("Multiple containers match %q — be more specific:", args[0])
			for _, r := range results {
				pterm.DefaultBasicText.Printfln("  %s", pterm.Green(r.Container.Name))
			}
			return fmt.Errorf("ambiguous: %d matches", len(results))
		}

		c := results[0].Container
		runCmd := expandRunCmd(c.Run)

		if runFlags != "" {
			runCmd = strings.Replace(runCmd, "docker run", "docker run "+runFlags, 1)
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

		pterm.DefaultBasicText.Printfln("%s %s", pterm.Bold.Sprint(">>>"), pterm.Yellow(runCmd))
		pterm.Println()

		if err := ensureVolumeDirs(extractVolumes(runCmd)); err != nil {
			pterm.Warning.Printfln("could not create volume dirs: %v", err)
		}

		parts := strings.Fields(runCmd)
		if len(parts) == 0 {
			return fmt.Errorf("empty run command for %s", c.Name)
		}

		if runTerminal {
			kittyArgs := append([]string{"--hold", "tmux", "new-session"}, parts...)
			return exec.Command("kitty", kittyArgs...).Start()
		}

		dockerPath, err := exec.LookPath(parts[0])
		if err != nil {
			return fmt.Errorf("docker not found in PATH: %w", err)
		}
		return syscall.Exec(dockerPath, parts, os.Environ())
	},
}

func init() {
	runCmd.Flags().BoolVarP(&runTerminal, "terminal", "t", false, "run in a new kitty terminal window")
	runCmd.Flags().StringVarP(&runFlags, "flags", "f", "", "extra flags to pass to docker run")
	rootCmd.AddCommand(runCmd)
}
