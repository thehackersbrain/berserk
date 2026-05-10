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
	binPath     func() string
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

// shellRCs returns all shell rc files that exist on disk.
func shellRCs() []string {
	candidates := []string{
		home(".bashrc"),
		home(".zshrc"),
		home(".config/fish/config.fish"),
		home(".kshrc"),
		home(".profile"),
	}
	var found []string
	for _, rc := range candidates {
		if _, err := os.Stat(rc); err == nil {
			found = append(found, rc)
		}
	}
	return found
}

// appendToPATH writes the bin dir export to every shell rc file that doesn't
// already contain it.
func appendToPATH(binDir string) error {
	rcs := shellRCs()
	if len(rcs) == 0 {
		return fmt.Errorf("no known shell rc files found — add %s to PATH manually", binDir)
	}

	var errs []string
	var updated []string

	for _, rc := range rcs {
		existing, _ := os.ReadFile(rc)
		if strings.Contains(string(existing), binDir) {
			continue
		}

		var line string
		if strings.HasSuffix(rc, "config.fish") {
			line = fmt.Sprintf("fish_add_path %s", binDir)
		} else {
			line = fmt.Sprintf(`export PATH="%s:$PATH"`, binDir)
		}

		f, err := os.OpenFile(rc, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", rc, err))
			continue
		}
		_, err = fmt.Fprintf(f, "\n# added by berserk doctor\n%s\n", line)
		f.Close() //nolint:errcheck
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", rc, err))
			continue
		}
		updated = append(updated, rc)
	}

	if len(updated) > 0 {
		pterm.Success.Printfln("added %s to PATH in: %s",
			pterm.Bold.Sprint(binDir), strings.Join(updated, ", "))
		pterm.Info.Printfln("reload with: source %s", updated[0])
	}
	if len(errs) > 0 {
		return fmt.Errorf("some rc files could not be updated:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
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

			if b.binPath == nil {
				continue
			}

			binDir := b.binPath()
			if binDir == "" {
				continue
			}

			if !inPATH[filepath.Clean(binDir)] {
				items = append(items, bulletItem(false,
					fmt.Sprintf("  %s bin dir %s is not in PATH — attempting to fix...",
						b.name, pterm.Yellow(binDir))))

				if err := pterm.DefaultBulletList.WithItems(items).Render(); err != nil {
					return err
				}
				items = nil

				if err := appendToPATH(binDir); err != nil {
					pterm.Warning.Printfln("could not add %s to PATH: %v", binDir, err)
				}
			} else {
				items = append(items, bulletItem(true,
					fmt.Sprintf("  %s bin dir %s", b.name, pterm.Gray(binDir))))
			}
		}

		if cargoOK {
			if _, err := exec.LookPath("cargo-install-update"); err != nil {
				items = append(items, bulletItem(false,
					"cargo-install-update not found — installing cargo-update..."))
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
