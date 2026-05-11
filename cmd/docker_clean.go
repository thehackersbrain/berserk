package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// runDockerClean is called when berserk --docker-clean is passed.
func runDockerClean(cmd *cobra.Command) error {
	assumeYes := false
	if f := cmd.PersistentFlags().Lookup("yes"); f != nil {
		assumeYes, _ = strconv.ParseBool(f.Value.String())
	}

	if !assumeYes {
		pterm.Warning.Println("This will stop ALL containers, remove ALL images, run docker system prune,")
		pterm.Warning.Println("and delete ~/berserk/docker and ~/berserk/containers.")
		fmt.Print("Continue? [y/N] ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		resp := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if resp != "y" && resp != "yes" {
			pterm.Info.Println("Aborted.")
			return nil
		}
	}

	pterm.Info.Println("Starting docker cleanup...")
	pterm.Println()

	var errs []string

	ids, err := dockerQueryLines("ps", "-q")
	switch {
	case err != nil:
		errs = append(errs, err.Error())
	case len(ids) > 0:
		pterm.Info.Printfln("Stopping %d container(s)...", len(ids))
		if err := dockerStream(append([]string{"stop"}, ids...)...); err != nil {
			errs = append(errs, fmt.Sprintf("docker stop: %v", err))
		}
	default:
		pterm.Info.Println("No running containers.")
	}

	imgIDs, err := dockerQueryLines("images", "-q")
	switch {
	case err != nil:
		errs = append(errs, err.Error())
	case len(imgIDs) > 0:
		pterm.Info.Printfln("Removing %d image(s)...", len(imgIDs))
		if err := dockerStream(append([]string{"rmi", "-f"}, imgIDs...)...); err != nil {
			errs = append(errs, fmt.Sprintf("docker rmi: %v", err))
		}
	default:
		pterm.Info.Println("No images found.")
	}

	pterm.Info.Println("Running docker system prune -f...")
	if err := dockerStream("system", "prune", "-f"); err != nil {
		errs = append(errs, fmt.Sprintf("docker system prune: %v", err))
	}

	dataDir := dockerDataDir(configDir())
	for _, sub := range []string{"docker", "containers"} {
		p := filepath.Join(dataDir, sub)
		if _, err := os.Stat(p); err == nil {
			pterm.Info.Printfln("Removing %s...", p)
			if err := os.RemoveAll(p); err != nil {
				// Docker containers running as root often leave root-owned
				// files under user-owned volumes. Fall back to `sudo rm -rf`
				// so cleanup actually completes (matches btweak behavior).
				if os.IsPermission(err) {
					pterm.Info.Printfln("  (root-owned files detected; retrying with sudo)")
					sudoCmd := exec.Command("sudo", "rm", "-rf", p)
					sudoCmd.Stdout = os.Stdout
					sudoCmd.Stderr = os.Stderr
					sudoCmd.Stdin = os.Stdin
					if serr := sudoCmd.Run(); serr != nil {
						errs = append(errs, fmt.Sprintf("sudo rm -rf %s: %v", p, serr))
					}
					continue
				}
				errs = append(errs, fmt.Sprintf("remove %s: %v", p, err))
			}
		}
	}

	pterm.Println()
	if len(errs) == 0 {
		pterm.Success.Println("Done.")
	} else {
		for _, e := range errs {
			pterm.Warning.Println(e)
		}
		return fmt.Errorf("cleanup finished with %d error(s)", len(errs))
	}
	return nil
}

// dockerQueryLines runs `docker <args...>` and returns each non-empty stdout
// line. The error is surfaced rather than swallowed so a broken docker daemon
// doesn't masquerade as an empty result — callers want to record that as a
// cleanup failure, not print "No running containers."
func dockerQueryLines(args ...string) ([]string, error) {
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("docker %s: %w", strings.Join(args, " "), err)
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// dockerStream runs `docker <args...>` with stdout/stderr wired to the terminal.
func dockerStream(args ...string) error {
	c := exec.Command("docker", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
