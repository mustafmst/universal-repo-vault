package archive

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeArchiveFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestZipArchiverPackUnpackRoundTrip(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeArchiveFile(t, filepath.Join(src, ".env"), "API_KEY=one\n")
	writeArchiveFile(t, filepath.Join(src, "nested", "app.secret.yaml"), "password: one\n")

	archiver := NewZipArchiver()
	data, err := archiver.Pack(src, []string{".env", "nested/app.secret.yaml"})
	if err != nil {
		t.Fatalf("expected pack to succeed, got %v", err)
	}

	if err := archiver.Unpack(dst, data, true); err != nil {
		t.Fatalf("expected unpack to succeed, got %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "nested", "app.secret.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "password: one\n" {
		t.Fatalf("unexpected unpacked data: %q", string(got))
	}
}

func TestZipArchiverRejectsUnsafePath(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../outside.env")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("secret")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	err = NewZipArchiver().Unpack(t.TempDir(), buf.Bytes(), true)
	if err == nil {
		t.Fatal("expected unsafe path error, got nil")
	}
	if !strings.Contains(err.Error(), "unsafe zip path") {
		t.Fatalf("expected unsafe path error, got %v", err)
	}
}

func TestZipArchiverHonorsOverwriteFlag(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeArchiveFile(t, filepath.Join(src, ".env"), "new\n")
	writeArchiveFile(t, filepath.Join(dst, ".env"), "old\n")

	data, err := NewZipArchiver().Pack(src, []string{".env"})
	if err != nil {
		t.Fatal(err)
	}

	err = NewZipArchiver().Unpack(dst, data, false)
	if err == nil {
		t.Fatal("expected existing file error, got nil")
	}

	if err := NewZipArchiver().Unpack(dst, data, true); err != nil {
		t.Fatalf("expected overwrite unpack to succeed, got %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("expected overwritten data, got %q", string(got))
	}
}
