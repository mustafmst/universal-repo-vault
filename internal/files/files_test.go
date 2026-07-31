package files

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDiscoveryFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListAllConfiguredFilesReturnsDeterministicDeduplicatedPaths(t *testing.T) {
	dir := t.TempDir()
	writeDiscoveryFile(t, filepath.Join(dir, ".env"))
	writeDiscoveryFile(t, filepath.Join(dir, "a.secret.yaml"))
	writeDiscoveryFile(t, filepath.Join(dir, "nested", "b.secret.yaml"))
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDiscoveryFile(t, filepath.Join(dir, ".git", "ignored.secret.yaml"))

	got, err := ListAllConfiguredFiles(dir, []string{".env", "a.secret.yaml"}, []string{"*.secret.*"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{".env", "a.secret.yaml", "nested/b.secret.yaml"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestListAllConfiguredFilesRejectsInvalidPattern(t *testing.T) {
	dir := t.TempDir()
	writeDiscoveryFile(t, filepath.Join(dir, ".env"))

	_, err := ListAllConfiguredFiles(dir, nil, []string{"["})
	if err == nil {
		t.Fatal("expected invalid pattern error, got nil")
	}
}
