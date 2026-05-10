package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
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

		// Stage in same dir as exe so rename is atomic (same filesystem).
		tmp, err := os.CreateTemp(filepath.Dir(exe), ".berserk-update-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name()) //nolint:errcheck

		if _, err := io.Copy(tmp, resp.Body); err != nil {
			tmp.Close() //nolint:errcheck
			return err
		}
		tmp.Close() //nolint:errcheck

		if err := os.Chmod(tmp.Name(), 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp.Name(), exe); err != nil {
			return fmt.Errorf("replacing binary: %w", err)
		}

		printOK("berserk updated successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(selfUpdateCmd)
}
