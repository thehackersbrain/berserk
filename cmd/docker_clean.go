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
		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			pterm.Info.Println("Aborted.")
			return nil
		}
	}

	pterm.Info.Println("Starting docker cleanup...")
	pterm.Println()

	var errs []string

	ids := dockerQueryLines("ps", "-q")
	if len(ids) > 0 {
		pterm.Info.Printfln("Stopping %d container(s)...", len(ids))
		if err := dockerStream(append([]string{"stop"}, ids...)...); err != nil {
			errs = append(errs, fmt.Sprintf("docker stop: %v", err))
		}
	} else {
		pterm.Info.Println("No running containers.")
	}

	imgIDs := dockerQueryLines("images", "-q")
	if len(imgIDs) > 0 {
		pterm.Info.Printfln("Removing %d image(s)...", len(imgIDs))
		if err := dockerStream(append([]string{"rmi", "-f"}, imgIDs...)...); err != nil {
			errs = append(errs, fmt.Sprintf("docker rmi: %v", err))
		}
	} else {
		pterm.Info.Println("No images found.")
	}

	pterm.Info.Println("Running docker system prune -f...")
	if err := dockerStream("system", "prune", "-f"); err != nil {
		errs = append(errs, fmt.Sprintf("docker system prune: %v", err))
	}

	dataDir := dockerDataDir()
	for _, sub := range []string{"docker", "containers"} {
		p := filepath.Join(dataDir, sub)
		if _, err := os.Stat(p); err == nil {
			pterm.Info.Printfln("Removing %s...", p)
			if err := os.RemoveAll(p); err != nil {
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

// dockerQueryLines runs `docker <args...>` and returns each non-empty stdout line.
func dockerQueryLines(args ...string) []string {
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// dockerStream runs `docker <args...>` with stdout/stderr wired to the terminal.
func dockerStream(args ...string) error {
	c := exec.Command("docker", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
