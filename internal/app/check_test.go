package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/keystore"
)

func TestCheckRepoSafeWhenStatusSafe(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}

	got, err := CheckRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected check to inspect repo, got %v", err)
	}
	if !got.Safe {
		t.Fatalf("expected safe check, got %#v", got.Messages)
	}
}

func TestCheckRepoUnsafeWhenStatusNeedsEncryption(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := CheckRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected check to inspect repo, got %v", err)
	}
	if got.Safe {
		t.Fatal("expected unsafe check")
	}
	if len(got.Messages) == 0 {
		t.Fatal("expected unsafe messages")
	}
}
