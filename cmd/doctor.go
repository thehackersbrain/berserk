package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type backend struct {
	name        string
	versionArgs []string
	binPath     func() string // resolves the backend's tool install dir; nil = not applicable
}

func home(rel string) string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, rel)
}

func cmdOut(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var backends = []backend{
	{
		name:        "pipx",
		versionArgs: []string{"--version"},
		binPath:     func() string { return home(".local/bin") },
	},
	{
		name:        "cargo",
		versionArgs: []string{"--version"},
		binPath:     func() string { return home(".cargo/bin") },
	},
	{
		name:        "go",
		versionArgs: []string{"version"},
		binPath:     func() string { return home("go/bin") },
	},
	{
		name:        "gem",
		versionArgs: []string{"--version"},
		binPath: func() string {
			out := cmdOut("gem", "env")
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "USER INSTALLATION DIRECTORY") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						return filepath.Join(strings.TrimSpace(parts[1]), "bin")
					}
				}
			}
			return ""
		},
	},
	{
		name:        "npm",
		versionArgs: []string{"--version"},
		binPath: func() string {
			if prefix := cmdOut("npm", "config", "get", "prefix"); prefix != "" {
				return filepath.Join(prefix, "bin")
			}
			return ""
		},
	},
	{
		name:        "git",
		versionArgs: []string{"--version"},
		binPath:     nil,
	},
}

func pathDirs() map[string]bool {
	dirs := map[string]bool{}
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		dirs[filepath.Clean(d)] = true
	}
	return dirs
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verify all installer backends are available",
	RunE: func(cmd *cobra.Command, args []string) error {
		var items []pterm.BulletListItem
		ok := 0
		cargoOK := false
		inPATH := pathDirs()

		for _, b := range backends {
			path, err := exec.LookPath(b.name)
			if err != nil {
				items = append(items, bulletItem(false,
					fmt.Sprintf("%-6s not found in PATH", b.name)))
				continue
			}

			out, err := exec.Command(path, b.versionArgs...).Output()
			if err != nil {
				items = append(items, bulletItem(false,
					fmt.Sprintf("%-6s found at %s but failed to run: %v", b.name, path, err)))
				continue
			}

			version := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
			items = append(items, bulletItem(true,
				fmt.Sprintf("%-6s %s", b.name, pterm.Gray(version))))
			ok++

			if b.name == "cargo" {
				cargoOK = true
			}

			// Check that the backend's tool bin dir is on PATH.
			if b.binPath != nil {
				binDir := b.binPath()
				if binDir == "" {
					continue
				}
				if !inPATH[filepath.Clean(binDir)] {
					items = append(items, bulletItem(false,
						fmt.Sprintf("  %s bin dir %s is not in PATH — tools installed via %s won't be found",
							b.name, pterm.Yellow(binDir), b.name)))
				} else {
					items = append(items, bulletItem(true,
						fmt.Sprintf("  %s bin dir %s", b.name, pterm.Gray(binDir))))
				}
			}
		}

		if cargoOK {
			if _, err := exec.LookPath("cargo-install-update"); err != nil {
				items = append(items, bulletItem(false,
					"cargo-install-update  not found — installing cargo-update..."))
				if err := pterm.DefaultBulletList.WithItems(items).Render(); err != nil {
					return err
				}
				items = nil

				installCmd := exec.Command("cargo", "install", "cargo-update")
				installCmd.Stdout = os.Stdout
				installCmd.Stderr = os.Stderr
				if err := installCmd.Run(); err != nil {
					pterm.Warning.Printfln("cargo install cargo-update failed: %v", err)
				} else {
					pterm.Success.Println("cargo-update installed — `cargo install-update -a` now available.")
				}
			} else {
				items = append(items, bulletItem(true,
					fmt.Sprintf("  cargo  %s", pterm.Gray("cargo install-update -a available"))))
			}
		}

		if err := pterm.DefaultBulletList.WithItems(items).Render(); err != nil {
			return err
		}
		pterm.Println()

		if ok == len(backends) {
			pterm.Success.Println("All backends available.")
		} else {
			pterm.Warning.Printfln("%d/%d backends missing — install them to use all tool types.",
				len(backends)-ok, len(backends))
		}
		return nil
	},
}

func bulletItem(success bool, text string) pterm.BulletListItem {
	if success {
		return pterm.BulletListItem{
			Bullet:      "✓",
			BulletStyle: pterm.NewStyle(pterm.FgGreen),
			Text:        text,
		}
	}
	return pterm.BulletListItem{
		Bullet:      "✗",
		BulletStyle: pterm.NewStyle(pterm.FgRed),
		Text:        text,
		TextStyle:   pterm.NewStyle(pterm.FgRed),
	}
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
