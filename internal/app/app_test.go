package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

const appTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func writeFile(t *testing.T, path string, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func setupRepoAndHome(t *testing.T) (repoPath string, homePath string) {
	t.Helper()
	repoPath = t.TempDir()
	homePath = t.TempDir()
	t.Setenv("HOME", homePath)
	if err := os.Mkdir(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\npatterns:\n  - \"*.secret.*\"\n", 0o644)
	writeFile(t, filepath.Join(repoPath, ".env"), "API_KEY=one\n", 0o600)
	writeFile(t, filepath.Join(repoPath, "nested", "app.secret.yaml"), "password: one\n", 0o600)
	if err := vault.SaveKey(appTestKey, repoPath, "repo-key"); err != nil {
		t.Fatal(err)
	}
	return repoPath, homePath
}

func TestEncryptDecryptRepoRoundTrip(t *testing.T) {
	repoPath, _ := setupRepoAndHome(t)

	result, err := EncryptRepo(repoPath)
	if err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	if !result.Encrypted {
		t.Fatal("expected first encrypt to write vault")
	}

	if _, err := os.Stat(filepath.Join(repoPath, vault.VaultFileName)); err != nil {
		t.Fatalf("expected vault file to exist: %v", err)
	}

	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repoPath, "nested", "app.secret.yaml")); err != nil {
		t.Fatal(err)
	}

	if err := DecryptRepo(repoPath); err != nil {
		t.Fatalf("expected decrypt to succeed, got %v", err)
	}

	envData, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(envData) != "API_KEY=one\n" {
		t.Fatalf("unexpected env contents: %q", string(envData))
	}

	secretData, err := os.ReadFile(filepath.Join(repoPath, "nested", "app.secret.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(secretData) != "password: one\n" {
		t.Fatalf("unexpected secret contents: %q", string(secretData))
	}
}

func TestEncryptRepoReturnsUnchangedWhenHashesMatchVault(t *testing.T) {
	repoPath, _ := setupRepoAndHome(t)

	first, err := EncryptRepo(repoPath)
	if err != nil {
		t.Fatalf("expected first encrypt to succeed, got %v", err)
	}
	if !first.Encrypted {
		t.Fatal("expected first encrypt to write vault")
	}

	second, err := EncryptRepo(repoPath)
	if err != nil {
		t.Fatalf("expected second encrypt to succeed, got %v", err)
	}
	if second.Encrypted {
		t.Fatal("expected unchanged hashes to skip encryption")
	}
}
