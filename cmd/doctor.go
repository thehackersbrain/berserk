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

// home joins rel onto the user's home directory. Returns "" when the
// home dir can't be resolved — callers must check, otherwise we'd happily
// write to relative paths in the cwd (filepath.Join("", ".bashrc") =
// ".bashrc"), which is the kind of footgun that surprises users who run
// berserk via `sudo` without -E.
func home(rel string) string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return ""
	}
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

// loginShellRC returns the rc file for the user's $SHELL. If $SHELL is
// unset or unrecognised, it falls back to ~/.profile (the POSIX default
// any sh-compatible login shell will source). Picking exactly one file
// avoids the "duplicate PATH export in both .bashrc and .zshrc" footgun
// when shells chain-source each other.
func loginShellRC() string {
	switch filepath.Base(os.Getenv("SHELL")) {
	case "bash":
		return home(".bashrc")
	case "zsh":
		return home(".zshrc")
	case "fish":
		return home(".config/fish/config.fish")
	case "ksh", "ksh93", "mksh":
		return home(".kshrc")
	default:
		return home(".profile")
	}
}

// appendToPATH writes the bin dir export to the user's login-shell rc
// (and only that one) if it isn't already present.
func appendToPATH(binDir string) error {
	rc := loginShellRC()
	if rc == "" {
		return fmt.Errorf("could not resolve home directory; set $HOME and rerun")
	}

	existing, _ := os.ReadFile(rc)
	if pathAlreadyExports(string(existing), binDir) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(rc), 0o755); err != nil {
		return fmt.Errorf("preparing %s: %w", filepath.Dir(rc), err)
	}

	var line string
	if strings.HasSuffix(rc, "config.fish") {
		line = fmt.Sprintf("fish_add_path %s", binDir)
	} else {
		line = fmt.Sprintf(`export PATH="%s:$PATH"`, binDir)
	}

	f, err := os.OpenFile(rc, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("%s: %w", rc, err)
	}
	defer f.Close() //nolint:errcheck

	if _, err := fmt.Fprintf(f, "\n# added by berserk doctor\n%s\n", line); err != nil {
		return fmt.Errorf("%s: %w", rc, err)
	}

	pterm.Success.Printfln("added %s to PATH in: %s",
		pterm.Bold.Sprint(binDir), rc)
	pterm.Info.Printfln("reload with: source %s", rc)
	return nil
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verify all installer backends are available",
	RunE: func(cmd *cobra.Command, args []string) error {
		var items []pterm.BulletListItem
		ok := 0
		cargoOK := false
		pipxOK := false
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
			if b.name == "pipx" {
				pipxOK = true
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

		if pipxOK {
			items = append(items, bulletItem(true,
				fmt.Sprintf("  pipx   %s", pterm.Gray("initializing shared venv..."))))
			if err := pterm.DefaultBulletList.WithItems(items).Render(); err != nil {
				return err
			}
			items = nil

			sharedCmd := exec.Command("pipx", "upgrade-shared")
			sharedCmd.Stdout = os.Stdout
			sharedCmd.Stderr = os.Stderr
			if err := sharedCmd.Run(); err != nil {
				pterm.Warning.Printfln("pipx upgrade-shared failed: %v", err)
			} else {
				pterm.Success.Println("pipx shared venv ready — parallel installs will not race on initialization.")
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

// pathAlreadyExports reports whether rcContent contains a non-comment
// reference to binDir as a complete PATH segment. Plain
// strings.Contains gave false positives — `/home/me/.cargo/bin` appeared
// to match a line containing `/home/me/.cargo/bin-old`, so doctor would
// silently skip the addition and the user's PATH stayed broken.
func pathAlreadyExports(rcContent, binDir string) bool {
	for _, line := range strings.Split(rcContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := 0
		for {
			j := strings.Index(line[idx:], binDir)
			if j < 0 {
				break
			}
			end := idx + j + len(binDir)
			// Match only when binDir is followed by a PATH segment
			// boundary: end-of-string, colon (next segment), or a
			// quote (closing the PATH= literal).
			if end >= len(line) {
				return true
			}
			switch line[end] {
			case ':', '"', '\'':
				return true
			}
			idx = end
		}
	}
	return false
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
