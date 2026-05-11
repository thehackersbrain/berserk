package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

const berserkRepo = "thehackersbrain/berserk"

// selfUpdateClient bounds the GitHub release download. http.DefaultClient
// has no timeout, so a stalled mirror would wedge the process forever.
var selfUpdateClient = &http.Client{Timeout: 5 * time.Minute}

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update berserk itself to the latest release",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _, opts, err := loadContext()
		if err != nil {
			return err
		}

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("finding current executable: %w", err)
		}

		url := fmt.Sprintf(
			"https://github.com/%s/releases/latest/download/berserk-%s-%s",
			berserkRepo, runtime.GOOS, runtime.GOARCH,
		)

		printProgress("Downloading latest berserk...")

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("building request: %w", err)
		}
		if opts.GithubToken != "" {
			req.Header.Set("Authorization", "Bearer "+opts.GithubToken)
		}

		resp, err := selfUpdateClient.Do(req)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != 200 {
			return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
		}

		// Stage in the exe's dir when writable (atomic rename), otherwise
		// stage in /tmp and finalize via `sudo install`. The Makefile's
		// default install location is /usr/local/bin which the invoking
		// user typically can't write to without sudo — without this path,
		// `berserk self-update` failed with a confusing "permission denied"
		// for the majority of installed users.
		stageDir := filepath.Dir(exe)
		needsSudo := !dirWritable(stageDir)
		if needsSudo {
			stageDir = "" // /tmp via empty dir to os.CreateTemp
		}

		tmp, err := os.CreateTemp(stageDir, ".berserk-update-*")
		if err != nil {
			return fmt.Errorf("staging download: %w", err)
		}
		defer os.Remove(tmp.Name()) //nolint:errcheck

		if _, err := io.Copy(tmp, resp.Body); err != nil {
			tmp.Close() //nolint:errcheck
			return err
		}
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("closing staged file: %w", err)
		}

		if err := os.Chmod(tmp.Name(), 0o755); err != nil {
			return err
		}

		if needsSudo {
			// `install` copies (not renames) — works across filesystems and
			// over a running binary on Linux (the kernel keeps the inode of
			// the running text segment open; the path swap is fine).
			installCmd := exec.Command("sudo", "install", "-m", "0755", tmp.Name(), exe)
			installCmd.Stdin = os.Stdin
			installCmd.Stdout = os.Stdout
			installCmd.Stderr = os.Stderr
			if err := installCmd.Run(); err != nil {
				return fmt.Errorf("sudo install %s: %w", exe, err)
			}
		} else {
			if err := os.Rename(tmp.Name(), exe); err != nil {
				return fmt.Errorf("replacing binary: %w", err)
			}
		}

		printOK("berserk updated successfully")
		return nil
	},
}

// dirWritable probes whether the invoking user can create files in dir.
// Used to decide between atomic rename and the sudo-install fallback.
func dirWritable(dir string) bool {
	probe, err := os.CreateTemp(dir, ".berserk-probe-*")
	if err != nil {
		return false
	}
	probe.Close()           //nolint:errcheck
	os.Remove(probe.Name()) //nolint:errcheck
	return true
}

func init() {
	rootCmd.AddCommand(selfUpdateCmd)
}
