package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/keystore"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

const statusTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

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

func TestStatusRepoInspectsVaultAndKeyWhenConfigIsInvalid(t *testing.T) {
	repoPath, store := statusRepo(t)
	if err := store.SaveKey(statusTestKey, repoPath, "repo-key"); err != nil {
		t.Fatal(err)
	}
	if err := vault.NewVaultFromData([]byte("vault data"), nil).SaveToFile(filepath.Join(repoPath, vault.VaultFileName)); err != nil {
		t.Fatal(err)
	}
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles: [\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.ConfigOK {
		t.Fatal("expected invalid config")
	}
	if !got.VaultExists || !got.VaultOK {
		t.Fatalf("expected valid vault, got exists=%v ok=%v", got.VaultExists, got.VaultOK)
	}
	if !got.KeyMapped || !got.KeyFileExists || !got.KeyLengthValid {
		t.Fatalf("expected valid mapped key, got %#v", got)
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

func TestStatusRepoReportsMalformedExistingVault(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\n", 0o644)
	writeStatusFile(t, filepath.Join(repoPath, vault.VaultFileName), "[", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !got.VaultExists {
		t.Fatal("expected malformed vault to be reported as existing")
	}
	if got.VaultOK {
		t.Fatal("expected malformed vault to be invalid")
	}
	if !hasStatusMessage(got.Errors, "vault .urv.vault.yaml is invalid") {
		t.Fatalf("expected malformed vault error, got %#v", got.Errors)
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

func TestStatusRepoReportsInvalidPattern(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "patterns:\n  - \"[\"\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallBrokenSetup {
		t.Fatalf("expected broken setup, got %q", got.Overall)
	}
	if !hasStatusMessage(got.Errors, "invalid file pattern") {
		t.Fatalf("expected invalid pattern error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsUnsupportedArchiverAndCypher(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\narchiver: tar\ncypher: age\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !hasStatusMessage(got.Errors, "unsupported archiver") {
		t.Fatalf("expected unsupported archiver error, got %#v", got.Errors)
	}
	if !hasStatusMessage(got.Errors, "unsupported cypher") {
		t.Fatalf("expected unsupported cypher error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsUnsafeExplicitPaths(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - ../outside.env\n  - /tmp/absolute.env\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !hasStatusMessage(got.Errors, "unsafe explicit file path") {
		t.Fatalf("expected unsafe path error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsReservedExplicitDescendant(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".git", "config"), "[core]\n", 0o644)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .git/config\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !hasStatusMessage(got.Errors, ".git/config") {
		t.Fatalf("expected reserved descendant error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsExplicitDirectory(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, "secrets", "app.env"), "API_KEY=one\n", 0o600)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - secrets\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !hasStatusMessage(got.Errors, "configured secret file is a directory") {
		t.Fatalf("expected explicit directory error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsMetadataMatchedByPattern(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "patterns:\n  - \"*\"\n", 0o644)
	if err := vault.NewVaultFromData([]byte("vault data"), nil).SaveToFile(filepath.Join(repoPath, vault.VaultFileName)); err != nil {
		t.Fatal(err)
	}
	writeStatusFile(t, filepath.Join(repoPath, ".urv.lock"), ".env: hash\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	for _, path := range []string{".urv.yaml", ".urv.vault.yaml", ".urv.lock"} {
		if !hasStatusMessage(got.Errors, path) {
			t.Fatalf("expected metadata pattern error for %q, got %#v", path, got.Errors)
		}
	}
}

func TestStatusRepoReturnsErrorWhenHashingConfiguredFileFails(t *testing.T) {
	repoPath, store := statusRepo(t)
	secretPath := filepath.Join(repoPath, ".env")
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\n", 0o644)
	writeStatusFile(t, secretPath, "API_KEY=one\n", 0o600)
	if err := os.Chmod(secretPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(secretPath, 0o600); err != nil && !os.IsNotExist(err) {
			t.Errorf("restoring secret permissions: %v", err)
		}
	})

	_, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err == nil {
		t.Fatal("expected hashing failure to make inspection fail")
	}
}

func TestStatusRepoWarnsWhenPatternMatchesNothing(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "patterns:\n  - \"*.secret.*\"\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !hasStatusMessage(got.Warnings, "pattern matched no files") {
		t.Fatalf("expected no-match warning, got %#v", got.Warnings)
	}
}
