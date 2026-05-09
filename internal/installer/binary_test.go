package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func makeTarGz(t *testing.T, dir, binaryName, content string) string {
	t.Helper()
	path := filepath.Join(dir, "archive.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte(content)
	tw.WriteHeader(&tar.Header{Name: binaryName, Size: int64(len(body)), Mode: 0755, Typeflag: tar.TypeReg})
	tw.Write(body)
	tw.Close()
	gz.Close()
	return path
}

func makeZip(t *testing.T, dir, binaryName, content string) string {
	t.Helper()
	path := filepath.Join(dir, "archive.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create(binaryName)
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(content))
	zw.Close()
	return path
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	archive := makeTarGz(t, dir, "mytool", "#!/bin/sh\necho hi")
	dest := filepath.Join(dir, "mytool")

	if err := extractTarGz(archive, "mytool", dest); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "#!/bin/sh\necho hi" {
		t.Errorf("content mismatch: %q", got)
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	archive := makeZip(t, dir, "mytool", "binary content")
	dest := filepath.Join(dir, "mytool")

	if err := extractZip(archive, "mytool", dest); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary content" {
		t.Errorf("content mismatch: %q", got)
	}
}

func TestExtractTarGzNotFound(t *testing.T) {
	dir := t.TempDir()
	archive := makeTarGz(t, dir, "other-binary", "content")
	dest := filepath.Join(dir, "mytool")

	err := extractTarGz(archive, "mytool", dest)
	if err == nil {
		t.Error("expected error when binary not found in archive")
	}
}

func TestExtractZipNotFound(t *testing.T) {
	dir := t.TempDir()
	archive := makeZip(t, dir, "other-binary", "content")
	dest := filepath.Join(dir, "mytool")

	err := extractZip(archive, "mytool", dest)
	if err == nil {
		t.Error("expected error when binary not found in zip")
	}
}

func TestTarGzNestedPath(t *testing.T) {
	// Binaries are often nested: frp_linux_amd64/frp
	dir := t.TempDir()
	path := filepath.Join(dir, "nested.tar.gz")
	f, _ := os.Create(path)
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("the binary")
	tw.WriteHeader(&tar.Header{
		Name:     "frp_linux_amd64/frp",
		Size:     int64(len(body)),
		Mode:     0755,
		Typeflag: tar.TypeReg,
	})
	tw.Write(body)
	tw.Close()
	gz.Close()
	f.Close()

	dest := filepath.Join(dir, "frp")
	if err := extractTarGz(path, "frp", dest); err != nil {
		t.Fatalf("extractTarGz nested: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "the binary" {
		t.Errorf("nested extract: got %q", got)
	}
}
