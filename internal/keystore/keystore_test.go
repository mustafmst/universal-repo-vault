package keystore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const keystoreTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestGenerateKeyReturns64HexCharacters(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(key) != 64 {
		t.Fatalf("expected 64 hex characters, got %d", len(key))
	}
}

func TestFileStoreSaveKeyCreates0600KeyAndMapping(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(t.TempDir(), "repo")
	store := NewFileStore(home)

	if err := store.SaveKey(keystoreTestKey, repoPath, "repo-key"); err != nil {
		t.Fatalf("expected save key to succeed, got %v", err)
	}

	keyPath := filepath.Join(home, ".config", "urv", "keys", "repo-key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected key mode 0600, got %o", info.Mode().Perm())
	}

	got, err := store.KeyForRepo(repoPath)
	if err != nil {
		t.Fatalf("expected mapped key, got %v", err)
	}
	if got != keystoreTestKey {
		t.Fatalf("expected key %q, got %q", keystoreTestKey, got)
	}
}

func TestFileStoreReadsExistingMappingFormat(t *testing.T) {
	home := t.TempDir()
	repoPath := "/tmp/example-repo"
	writePath := filepath.Join(home, ".config", "urv")
	if err := os.MkdirAll(filepath.Join(writePath, "keys"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(writePath, "keys", "repo-key"), []byte(keystoreTestKey), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(writePath, "mapping.yaml"), []byte(repoPath+": repo-key\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewFileStore(home).KeyForRepo(repoPath)
	if err != nil {
		t.Fatalf("expected mapped key, got %v", err)
	}
	if got != keystoreTestKey {
		t.Fatalf("expected key %q, got %q", keystoreTestKey, got)
	}
}

func TestFileStoreMissingKeyForRepoReturnsContext(t *testing.T) {
	_, err := NewFileStore(t.TempDir()).KeyForRepo("/tmp/missing")
	if err == nil {
		t.Fatal("expected missing mapping error, got nil")
	}
	if !strings.Contains(err.Error(), "key for repo not found") {
		t.Fatalf("expected missing mapping error, got %v", err)
	}
}
