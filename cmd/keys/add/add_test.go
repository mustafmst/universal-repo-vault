package add

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddCommandRejectsMissingKeySource(t *testing.T) {
	inGitRepo(t)
	cmd := NewCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "provide --key or --file") {
		t.Fatalf("expected missing source error, got %v", err)
	}
}

func TestAddCommandRejectsMultipleKeySources(t *testing.T) {
	inGitRepo(t)
	cmd := NewCommand()
	cmd.SetArgs([]string{"--key", "existing", "--file", "./key"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "provide only one of --key or --file") {
		t.Fatalf("expected mutually exclusive source error, got %v", err)
	}
}

func TestAddCommandRejectsPositionalArgs(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"unexpected"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "accepts 0 arg") {
		t.Fatalf("expected positional arg error, got %v", err)
	}
}

func inGitRepo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}
