package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/keystore"
)

func writeStatusFile(t *testing.T, path string, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func statusRepo(t *testing.T) (string, *keystore.FileStore) {
	t.Helper()
	repoPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repoPath, keystore.NewFileStore(t.TempDir())
}

func findStatusFile(files []StatusFile, path string) (StatusFile, bool) {
	for _, file := range files {
		if file.Path == path {
			return file, true
		}
	}
	return StatusFile{}, false
}

func hasStatusMessage(messages []string, want string) bool {
	for _, msg := range messages {
		if strings.Contains(msg, want) {
			return true
		}
	}
	return false
}

func TestStatusRepoReportsMissingConfig(t *testing.T) {
	repoPath, store := statusRepo(t)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallBrokenSetup {
		t.Fatalf("expected broken setup, got %q", got.Overall)
	}
	if got.ConfigOK {
		t.Fatal("expected config to be invalid")
	}
	if !hasStatusMessage(got.Errors, ".urv.yaml") {
		t.Fatalf("expected config error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsMissingVaultAndKey(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\n", 0o644)
	writeStatusFile(t, filepath.Join(repoPath, ".env"), "API_KEY=one\n", 0o600)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallBrokenSetup {
		t.Fatalf("expected broken setup, got %q", got.Overall)
	}
	if !got.ConfigOK {
		t.Fatal("expected config ok")
	}
	if got.VaultExists || got.VaultOK {
		t.Fatalf("expected missing vault, got exists=%v ok=%v", got.VaultExists, got.VaultOK)
	}
	if got.KeyMapped {
		t.Fatal("expected no mapped key")
	}
	if !hasStatusMessage(got.Errors, ".urv.vault.yaml") {
		t.Fatalf("expected missing vault error, got %#v", got.Errors)
	}
	if !hasStatusMessage(got.Errors, "key for repo not found") {
		t.Fatalf("expected missing key error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsUnchangedFiles(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	store := keystore.NewFileStore(homePath)
	services := Services{KeyStore: store}

	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}

	got, err := StatusRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallSafe {
		t.Fatalf("expected safe, got %q with errors %#v", got.Overall, got.Errors)
	}
	env, ok := findStatusFile(got.Files, ".env")
	if !ok {
		t.Fatalf("expected .env in files: %#v", got.Files)
	}
	if env.Status != FileUnchanged {
		t.Fatalf("expected unchanged, got %q", env.Status)
	}
}

func TestStatusRepoReportsChangedAndNewFiles(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	store := keystore.NewFileStore(homePath)
	services := Services{KeyStore: store}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	writeStatusFile(t, filepath.Join(repoPath, ".env"), "API_KEY=changed\n", 0o600)
	writeStatusFile(t, filepath.Join(repoPath, "new.secret.yaml"), "password: two\n", 0o600)

	got, err := StatusRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallNeedsEncryption {
		t.Fatalf("expected needs encryption, got %q", got.Overall)
	}
	env, ok := findStatusFile(got.Files, ".env")
	if !ok || env.Status != FileChanged {
		t.Fatalf("expected changed .env, got %#v found=%v", env, ok)
	}
	newFile, ok := findStatusFile(got.Files, "new.secret.yaml")
	if !ok || newFile.Status != FileNew {
		t.Fatalf("expected new file, got %#v found=%v", newFile, ok)
	}
}

func TestStatusRepoReportsMissingExplicitFile(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	store := keystore.NewFileStore(homePath)
	services := Services{KeyStore: store}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}

	got, err := StatusRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallNeedsEncryption {
		t.Fatalf("expected needs encryption, got %q", got.Overall)
	}
	env, ok := findStatusFile(got.Files, ".env")
	if !ok || env.Status != FileMissing {
		t.Fatalf("expected missing .env, got %#v found=%v", env, ok)
	}
}

func TestStatusRepoReportsVaultOnlyFile(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	store := keystore.NewFileStore(homePath)
	services := Services{KeyStore: store}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallNeedsEncryption {
		t.Fatalf("expected needs encryption, got %q", got.Overall)
	}
	if len(got.Files) != 2 || got.Files[0].Path != ".env" || got.Files[1].Path != "nested/app.secret.yaml" {
		t.Fatalf("expected files sorted by path, got %#v", got.Files)
	}
	vaultOnly, ok := findStatusFile(got.Files, "nested/app.secret.yaml")
	if !ok || vaultOnly.Status != FileVaultOnly {
		t.Fatalf("expected vault-only file, got %#v found=%v", vaultOnly, ok)
	}
}
