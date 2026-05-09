package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const defaultBerserkRepo = "thehackersbrain/berserk"

var selfUpdateClient = &http.Client{Timeout: 5 * time.Minute}

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update berserk itself to the latest release",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _, opts, err := loadContext()
		if err != nil {
			return err
		}
		repo := defaultBerserkRepo
		if r := os.Getenv("BERSERK_SELF_REPO"); r != "" {
			repo = r
		}
		return selfUpdate(repo, opts.GithubToken)
	},
}

type ghReleaseSelf struct {
	TagName string        `json:"tag_name"`
	Assets  []ghAssetSelf `json:"assets"`
}

type ghAssetSelf struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func selfUpdate(repo, githubToken string) error {
	printProgress("Checking for latest berserk release...")

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}

	resp, err := selfUpdateClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("no releases found at github.com/%s — publish a release first", repo)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghReleaseSelf
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("decoding release: %w", err)
	}

	// Find asset for current OS/arch
	pattern := fmt.Sprintf("berserk-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, a := range release.Assets {
		if strings.Contains(a.Name, pattern) {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no binary asset matching %q in release %s", pattern, release.TagName)
	}

	printProgress("Downloading %s...", release.TagName)

	dlReq, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	if githubToken != "" {
		dlReq.Header.Set("Authorization", "Bearer "+githubToken)
	}

	dlResp, err := selfUpdateClient.Do(dlReq)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != 200 {
		return fmt.Errorf("downloading binary: HTTP %d", dlResp.StatusCode)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current executable: %w", err)
	}

	// Stage the new binary on the SAME filesystem as the running exe so the
	// final swap can be a rename. Opening the running exe for write returns
	// ETXTBSY on Linux, so a cross-fs copy fallback would fail anyway.
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".berserk-update-*")
	if err != nil {
		return fmt.Errorf("creating staging file in %s: %w", filepath.Dir(exe), err)
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, dlResp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("writing download: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		return fmt.Errorf("replacing binary at %s: %w", exe, err)
	}

	printOK("berserk updated to %s", release.TagName)
	return nil
}

func init() {
	rootCmd.AddCommand(selfUpdateCmd)
}
