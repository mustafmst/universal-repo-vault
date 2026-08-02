package check

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/keystore"
)

func TestFormatCheckResultSafe(t *testing.T) {
	got := FormatCheckResult(&app.CheckResult{Safe: true})

	if !strings.Contains(got, "URV check passed") {
		t.Fatalf("expected pass output, got %q", got)
	}
}

func TestFormatCheckResultUnsafe(t *testing.T) {
	got := FormatCheckResult(&app.CheckResult{Safe: false, Messages: []string{".env changed", "key missing"}})

	for _, want := range []string{"URV check failed", ".env changed", "key missing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestCheckCommandSucceedsForSafeRepository(t *testing.T) {
	repoPath := setupCheckCommandRepo(t)
	t.Chdir(repoPath)

	cmd := NewCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected safe check to succeed, got %v", err)
	}
	if !strings.Contains(output.String(), "URV check passed") {
		t.Fatalf("expected pass output, got:\n%s", output.String())
	}
}

func TestCheckCommandFailsForUnsafeRepository(t *testing.T) {
	repoPath := setupCheckCommandRepo(t)
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("API_KEY=changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repoPath)

	cmd := NewCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unsafe check to fail")
	}
	if !strings.Contains(err.Error(), "repository is not safe to commit") {
		t.Fatalf("expected unsafe check error, got %v", err)
	}
	for _, want := range []string{"URV check failed", ".env changed"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("expected %q in output, got:\n%s", want, output.String())
		}
	}
}

func setupCheckCommandRepo(t *testing.T) string {
	t.Helper()
	repoPath := t.TempDir()
	homePath := t.TempDir()
	t.Setenv("HOME", homePath)
	runCheckGit(t, repoPath, "init")
	if err := os.WriteFile(filepath.Join(repoPath, ".urv.yaml"), []byte("secretfiles:\n  - .env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("API_KEY=one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := keystore.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := keystore.NewDefaultFileStore().SaveKey(key, repoPath, "repo-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EncryptRepo(repoPath); err != nil {
		t.Fatal(err)
	}
	return repoPath
}

func runCheckGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
