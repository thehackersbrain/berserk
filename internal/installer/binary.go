package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thehackersbrain/berserk/internal/registry"
)

// httpClient bounds every GitHub fetch/download. A long-running install
// holding open a stalled HTTPS connection is a real problem under
// `parallel: true`, where one bad release server stalls the whole batch.
var httpClient = &http.Client{Timeout: 5 * time.Minute}

type ghRelease struct {
	Assets []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func Binary(tool registry.Tool, installDir, githubToken string) error {
	if tool.Repo == "" {
		return fmt.Errorf("binary installer: no repo defined for %s", tool.Name)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", tool.Repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching release for %s: %w", tool.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, tool.Repo)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("decoding release: %w", err)
	}

	pattern := tool.AssetPattern
	if pattern == "" {
		pattern = tool.Name
	}

	var downloadURL, assetName string
	for _, a := range release.Assets {
		if strings.Contains(a.Name, pattern) {
			downloadURL = a.BrowserDownloadURL
			assetName = a.Name
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no asset matching %q found for %s", pattern, tool.Name)
	}

	tmp, err := os.CreateTemp("", "berserk-*-"+assetName)
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	dlReq, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	if githubToken != "" {
		dlReq.Header.Set("Authorization", "Bearer "+githubToken)
	}

	dlResp, err := httpClient.Do(dlReq)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", assetName, err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != 200 {
		return fmt.Errorf("downloading %s: HTTP %d", assetName, dlResp.StatusCode)
	}

	if _, err := io.Copy(tmp, dlResp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("writing download: %w", err)
	}
	tmp.Close()

	finalDest := filepath.Join(installDir, tool.Name)

	// If we can't write to installDir, stage the binary in a writable temp
	// location and use `sudo install` to place it. This keeps things simple
	// for the common /usr/local/bin case.
	stagingDest := finalDest
	needsSudo := !canWrite(installDir)
	if needsSudo {
		staging, err := os.CreateTemp("", "berserk-staged-"+tool.Name+"-*")
		if err != nil {
			return err
		}
		staging.Close()
		stagingDest = staging.Name()
		defer os.Remove(stagingDest)
	}

	switch {
	case strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz"):
		if err := extractTarGz(tmpPath, tool.Name, stagingDest); err != nil {
			return fmt.Errorf("extracting %s: %w", assetName, err)
		}
	case strings.HasSuffix(assetName, ".zip"):
		if err := extractZip(tmpPath, tool.Name, stagingDest); err != nil {
			return fmt.Errorf("extracting %s: %w", assetName, err)
		}
	default:
		if err := moveFile(tmpPath, stagingDest); err != nil {
			return err
		}
	}

	if err := os.Chmod(stagingDest, 0755); err != nil {
		return fmt.Errorf("chmod %s: %w", stagingDest, err)
	}

	if needsSudo {
		return runRoot("install", "-m", "0755", stagingDest, finalDest)
	}
	return nil
}

// extractTarGz finds the first file named `binaryName` inside a .tar.gz and
// writes it to dest. If no exact match, it picks the first regular file.
func extractTarGz(archivePath, binaryName, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	return extractFromTar(tr, binaryName, dest)
}

func extractFromTar(tr *tar.Reader, binaryName, dest string) error {
	var firstEntry string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base == binaryName || strings.TrimSuffix(base, ".exe") == binaryName {
			return writeTarEntry(tr, dest)
		}
		if firstEntry == "" {
			firstEntry = hdr.Name
		}
	}
	if firstEntry != "" {
		return fmt.Errorf("binary %q not found in archive (first entry: %s)", binaryName, firstEntry)
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}

func writeTarEntry(r io.Reader, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func extractZip(archivePath, binaryName, dest string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if base == binaryName || strings.TrimSuffix(base, ".exe") == binaryName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return writeTarEntry(rc, dest)
		}
	}
	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
