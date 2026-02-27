package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Update checks for a new version and replaces the current binary.
func Update(currentVersion string, out io.Writer) error {
	if currentVersion == "" || currentVersion == "dev" {
		return fmt.Errorf("cannot update a dev build — install from a release or use the install script")
	}

	fmt.Fprintln(out, "Checking for updates...")

	resp, err := http.Get("https://api.github.com/repos/lucharo/society/releases/latest")
	if err != nil {
		return fmt.Errorf("checking latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parsing release info: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	if latest == currentVersion {
		fmt.Fprintf(out, "Already up to date (v%s).\n", currentVersion)
		return nil
	}

	fmt.Fprintf(out, "Current: v%s → Latest: v%s\n", currentVersion, latest)

	// Find matching asset
	osName := runtime.GOOS
	arch := runtime.GOARCH
	wantName := fmt.Sprintf("society_%s_%s_%s.tar.gz", latest, osName, arch)

	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == wantName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no release binary for %s/%s — download manually from https://github.com/lucharo/society/releases", osName, arch)
	}

	fmt.Fprintf(out, "Downloading %s...\n", wantName)

	// Download to temp file
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", dlResp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "society-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, wantName)
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		return fmt.Errorf("saving download: %w", err)
	}
	f.Close()

	// Extract tar.gz
	if err := extractTarGz(tarPath, tmpDir); err != nil {
		return fmt.Errorf("extracting: %w", err)
	}

	newBinary := filepath.Join(tmpDir, "society")
	if _, err := os.Stat(newBinary); err != nil {
		return fmt.Errorf("extracted binary not found: %w", err)
	}

	// Replace current binary
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	// Atomic-ish replace: rename old, move new, remove old
	oldPath := currentBinary + ".old"
	if err := os.Rename(currentBinary, oldPath); err != nil {
		return fmt.Errorf("backing up current binary: %w (try with sudo?)", err)
	}

	newData, err := os.ReadFile(newBinary)
	if err != nil {
		// Restore old binary
		os.Rename(oldPath, currentBinary)
		return fmt.Errorf("reading new binary: %w", err)
	}

	if err := os.WriteFile(currentBinary, newData, 0755); err != nil {
		// Restore old binary
		os.Rename(oldPath, currentBinary)
		return fmt.Errorf("writing new binary: %w (try with sudo?)", err)
	}

	os.Remove(oldPath)

	// macOS: ad-hoc code sign
	if runtime.GOOS == "darwin" {
		codesign(currentBinary)
	}

	fmt.Fprintf(out, "Updated to v%s.\n", latest)
	return nil
}

func extractTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
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
		// Only extract the "society" binary, ignore paths
		name := filepath.Base(hdr.Name)
		if name != "society" {
			continue
		}
		out, err := os.OpenFile(filepath.Join(destDir, name), os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		out.Close()
		break
	}
	return nil
}

func codesign(path string) {
	exec.Command("codesign", "-s", "-", path).Run()
}
