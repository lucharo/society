package cli

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// maxBinarySize is the maximum size of the extracted binary (200 MB).
const maxBinarySize = 200 * 1024 * 1024

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

	if resp.StatusCode == 403 {
		return fmt.Errorf("GitHub API rate limit exceeded — try again later or download from https://github.com/lucharo/society/releases")
	}
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

	// Find matching asset and checksums
	osName := runtime.GOOS
	arch := runtime.GOARCH
	wantName := fmt.Sprintf("society_%s_%s_%s.tar.gz", latest, osName, arch)

	var downloadURL, checksumsURL string
	for _, a := range release.Assets {
		switch a.Name {
		case wantName:
			downloadURL = a.BrowserDownloadURL
		case "SHA256SUMS":
			checksumsURL = a.BrowserDownloadURL
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
	if _, err := io.Copy(f, io.LimitReader(dlResp.Body, maxBinarySize)); err != nil {
		f.Close()
		return fmt.Errorf("saving download: %w", err)
	}
	f.Close()

	// Verify checksum if available
	if checksumsURL != "" {
		if err := verifyChecksum(tarPath, wantName, checksumsURL); err != nil {
			return err
		}
		fmt.Fprintln(out, "Checksum verified.")
	}

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

	// Atomic replace: write new binary to temp file in same dir, then rename over current
	destDir := filepath.Dir(currentBinary)
	tmpBin, err := os.CreateTemp(destDir, "society-new-*")
	if err != nil {
		return fmt.Errorf("creating temp file for new binary: %w (try with sudo?)", err)
	}
	tmpBinPath := tmpBin.Name()

	newData, err := os.ReadFile(newBinary)
	if err != nil {
		tmpBin.Close()
		os.Remove(tmpBinPath)
		return fmt.Errorf("reading new binary: %w", err)
	}

	if _, err := tmpBin.Write(newData); err != nil {
		tmpBin.Close()
		os.Remove(tmpBinPath)
		return fmt.Errorf("writing new binary: %w (try with sudo?)", err)
	}
	tmpBin.Close()

	if err := os.Chmod(tmpBinPath, 0755); err != nil {
		os.Remove(tmpBinPath)
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := os.Rename(tmpBinPath, currentBinary); err != nil {
		os.Remove(tmpBinPath)
		return fmt.Errorf("replacing binary: %w (try with sudo?)", err)
	}

	// macOS: ad-hoc code sign
	if runtime.GOOS == "darwin" {
		codesign(currentBinary, out)
	}

	fmt.Fprintf(out, "Updated to v%s.\n", latest)
	return nil
}

func verifyChecksum(filePath, fileName, checksumsURL string) error {
	resp, err := http.Get(checksumsURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("checksums download returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	// Parse SHA256SUMS: "<hash>  <filename>" per line
	var expectedHash string
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == fileName {
			expectedHash = parts[0]
			break
		}
	}
	if expectedHash == "" {
		return fmt.Errorf("checksum for %s not found in SHA256SUMS", fileName)
	}
	if len(expectedHash) != 64 {
		return fmt.Errorf("invalid checksum length for %s: expected 64 hex chars, got %d", fileName, len(expectedHash))
	}

	// Hash the downloaded file
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hashing file: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

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
		if _, err := io.Copy(out, io.LimitReader(tr, maxBinarySize)); err != nil {
			out.Close()
			return err
		}
		out.Close()
		break
	}
	return nil
}

func codesign(path string, out io.Writer) {
	if err := exec.Command("codesign", "-s", "-", path).Run(); err != nil {
		slog.Warn("ad-hoc code signing failed", "error", err)
		fmt.Fprintln(out, "Warning: ad-hoc code signing failed — you may need to allow the binary in System Settings > Privacy & Security.")
	}
}
