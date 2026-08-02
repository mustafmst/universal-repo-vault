package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/keystore"
)

func runAppGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

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

func TestCheckRepoUnsafeWhenConfiguredPlaintextFileIsStaged(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}
	runAppGit(t, repoPath, "add", ".env")

	got, err := CheckRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected check to inspect repo, got %v", err)
	}
	if got.Safe {
		t.Fatal("expected staged plaintext file to fail check")
	}
}
