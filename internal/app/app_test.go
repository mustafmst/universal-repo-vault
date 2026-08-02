package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/archive"
	"github.com/mustafmst/universal-repo-vault/internal/keystore"
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
	if err := keystore.NewDefaultFileStore().SaveKey(appTestKey, repoPath, "repo-key"); err != nil {
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

func TestEncryptDecryptRepoWithServicesRoundTrip(t *testing.T) {
	repoPath := t.TempDir()
	homePath := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\n", 0o644)
	writeFile(t, filepath.Join(repoPath, ".env"), "API_KEY=two\n", 0o600)

	store := keystore.NewFileStore(homePath)
	if err := store.SaveKey(appTestKey, repoPath, "repo-key"); err != nil {
		t.Fatal(err)
	}

	services := Services{
		Archiver: archive.NewZipArchiver(),
		KeyStore: store,
	}

	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := DecryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected decrypt to succeed, got %v", err)
	}
	got, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "API_KEY=two\n" {
		t.Fatalf("unexpected env contents: %q", string(got))
	}
}

func TestDecryptRepoWithOptionsDryRunDoesNotWriteFiles(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}

	result, err := DecryptRepoWithServicesAndOptions(repoPath, services, DecryptOptions{DryRun: true, Overwrite: true})

	if err != nil {
		t.Fatalf("expected dry run to succeed, got %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected dry run result")
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".env")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry run not to restore file, got %v", err)
	}
}

func TestDecryptRepoWithOptionsNoOverwriteFailsOnExistingFile(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}

	_, err := DecryptRepoWithServicesAndOptions(repoPath, services, DecryptOptions{Overwrite: false})

	if err == nil {
		t.Fatal("expected no-overwrite decrypt to fail")
	}
	if !strings.Contains(err.Error(), "file was not replaced") && !strings.Contains(err.Error(), "file exists") {
		t.Fatalf("expected existing file error, got %v", err)
	}
}

func TestDecryptRepoDefaultStillOverwrites(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repoPath, ".env"), "LOCAL=changed\n", 0o600)

	if _, err := DecryptRepoWithServicesAndOptions(repoPath, services, DecryptOptions{Overwrite: true}); err != nil {
		t.Fatalf("expected overwrite decrypt to succeed, got %v", err)
	}
	got, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "API_KEY=one\n" {
		t.Fatalf("expected vault contents, got %q", string(got))
	}
}
