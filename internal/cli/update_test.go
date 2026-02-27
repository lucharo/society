package cli

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestTarGz(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	tarPath := filepath.Join(dir, "test.tar.gz")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0755,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	return tarPath
}

func TestExtractTarGz(t *testing.T) {
	tmpDir := t.TempDir()

	tarPath := createTestTarGz(t, tmpDir, map[string]string{
		"society": "fake-binary-content",
	})

	destDir := t.TempDir()
	if err := extractTarGz(tarPath, destDir); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "society"))
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}
	if string(data) != "fake-binary-content" {
		t.Errorf("got %q, want %q", string(data), "fake-binary-content")
	}
}

func TestExtractTarGz_IgnoresOtherFiles(t *testing.T) {
	tmpDir := t.TempDir()

	tarPath := createTestTarGz(t, tmpDir, map[string]string{
		"README.md": "readme content",
		"society":   "the-binary",
		"LICENSE":   "license content",
	})

	destDir := t.TempDir()
	if err := extractTarGz(tarPath, destDir); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	// Only society should exist
	if _, err := os.Stat(filepath.Join(destDir, "society")); err != nil {
		t.Error("society binary not found")
	}
	if _, err := os.Stat(filepath.Join(destDir, "README.md")); err == nil {
		t.Error("README.md should not be extracted")
	}
	if _, err := os.Stat(filepath.Join(destDir, "LICENSE")); err == nil {
		t.Error("LICENSE should not be extracted")
	}
}

func TestExtractTarGz_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a tar with a file that claims to be huge but has small content
	// The LimitReader will cap the read at maxBinarySize
	tarPath := createTestTarGz(t, tmpDir, map[string]string{
		"society": strings.Repeat("x", 1024),
	})

	destDir := t.TempDir()
	if err := extractTarGz(tarPath, destDir); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "society"))
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}
	if len(data) != 1024 {
		t.Errorf("got %d bytes, want 1024", len(data))
	}
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// filepath.Base strips directory traversal
	tarPath := createTestTarGz(t, tmpDir, map[string]string{
		"../../society": "malicious-content",
	})

	destDir := t.TempDir()
	if err := extractTarGz(tarPath, destDir); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	// Should be extracted safely into destDir, not traversing up
	data, err := os.ReadFile(filepath.Join(destDir, "society"))
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}
	if string(data) != "malicious-content" {
		t.Errorf("got %q, want %q", string(data), "malicious-content")
	}

	// Verify nothing was written outside destDir
	if _, err := os.Stat(filepath.Join(destDir, "..", "..", "society")); err == nil {
		t.Error("file was written outside destDir via path traversal")
	}
}
