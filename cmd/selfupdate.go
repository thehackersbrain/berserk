package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

const berserkRepo = "thehackersbrain/berserk"

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

		req, _ := http.NewRequest("GET", url, nil)
		if opts.GithubToken != "" {
			req.Header.Set("Authorization", "Bearer "+opts.GithubToken)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
		}

		// Stage in same dir as exe so rename is atomic (same filesystem).
		tmp, err := os.CreateTemp(filepath.Dir(exe), ".berserk-update-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())

		if _, err := io.Copy(tmp, resp.Body); err != nil {
			tmp.Close()
			return err
		}
		tmp.Close()

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
