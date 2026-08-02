package decrypt

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/archive"
	"github.com/mustafmst/universal-repo-vault/internal/keystore"
)

func TestFormatDryRunResult(t *testing.T) {
	result := &app.DecryptResult{
		DryRun: true,
		Files: []archive.EntryPlan{
			{Path: ".env", Action: archive.EntryOverwrite},
			{Path: "nested/app.secret.yaml", Action: archive.EntryCreate},
		},
	}

	got := FormatDryRunResult(result)

	for _, want := range []string{"Decrypt dry run:", ".env overwrite", "nested/app.secret.yaml create"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestDecryptCommandDryRunDoesNotWriteFiles(t *testing.T) {
	repoPath := setupDecryptCommandRepo(t)
	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repoPath)

	cmd := NewCommand()
	cmd.SetArgs([]string{"--dry-run"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected dry-run decrypt to succeed, got %v", err)
	}
	for _, want := range []string{"Decrypt dry run:", ".env create"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("expected %q in output, got:\n%s", want, output.String())
		}
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".env")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry run not to restore file, got %v", err)
	}
}

func TestDecryptCommandNoOverwriteFailsBeforeReplacingExistingFile(t *testing.T) {
	repoPath := setupDecryptCommandRepo(t)
	const localContents = "API_KEY=local\n"
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte(localContents), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repoPath)

	cmd := NewCommand()
	cmd.SetArgs([]string{"--no-overwrite"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected no-overwrite decrypt to fail")
	}
	got, readErr := os.ReadFile(filepath.Join(repoPath, ".env"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != localContents {
		t.Fatalf("expected existing file to remain unchanged, got %q", got)
	}
	if strings.Contains(output.String(), "Vault unpacked successfully") {
		t.Fatalf("expected no success output, got:\n%s", output.String())
	}
}

func setupDecryptCommandRepo(t *testing.T) string {
	t.Helper()
	repoPath := t.TempDir()
	homePath := t.TempDir()
	t.Setenv("HOME", homePath)
	runDecryptGit(t, repoPath, "init")
	if err := os.WriteFile(filepath.Join(repoPath, ".urv.yaml"), []byte("secretfiles:\n  - .env\n"), 0o644); err != nil {
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

func runDecryptGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
