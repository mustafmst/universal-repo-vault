package repo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func TestCheckDirGitRepo(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string) string
		want       bool
		wantErr    bool
		errMessage string
	}{
		{
			name: "returns true when git directory exists",
			setup: func(t *testing.T, dir string) string {
				t.Helper()

				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}

				return dir
			},
			want: true,
		},
		{
			name: "returns false when git directory does not exist",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				return dir
			},
			want: false,
		},
		{
			name: "returns true when git path is a file",
			setup: func(t *testing.T, dir string) string {
				t.Helper()

				if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ../real-git-dir"), 0o644); err != nil {
					t.Fatal(err)
				}

				return dir
			},
			want: true,
		},
		{
			name: "wraps stat errors",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				return "bad\x00path"
			},
			want:       false,
			wantErr:    true,
			errMessage: "git repo check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			got, err := checkDirGitRepo(tt.setup(t, dir))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMessage != "" && !strings.Contains(err.Error(), tt.errMessage) {
					t.Fatalf("expected error to contain %q, got %v", tt.errMessage, err)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestGetRepoPathForPath(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string) (startPath string, wantPath string)
		wantErr error
	}{
		{
			name: "returns repo root for root path",
			setup: func(t *testing.T, dir string) (string, string) {
				t.Helper()

				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}

				return dir, dir
			},
		},
		{
			name: "walks up from nested directory",
			setup: func(t *testing.T, dir string) (string, string) {
				t.Helper()

				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}

				nestedDir := filepath.Join(dir, "one", "two")
				if err := os.MkdirAll(nestedDir, 0o755); err != nil {
					t.Fatal(err)
				}

				return nestedDir, dir
			},
		},
		{
			name: "returns ErrNoRepoFound when no repo exists",
			setup: func(t *testing.T, dir string) (string, string) {
				t.Helper()
				return dir, ""
			},
			wantErr: ErrNoRepoFound,
		},
		{
			name: "returns ErrNoRepoFound for relative path when no repo exists",
			setup: func(t *testing.T, dir string) (string, string) {
				t.Helper()
				t.Chdir(dir)
				return ".", ""
			},
			wantErr: ErrNoRepoFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			startPath, wantPath := tt.setup(t, dir)

			got, err := getRepoPathForPath(startPath)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got != wantPath {
				t.Fatalf("expected %q, got %q", wantPath, got)
			}
		})
	}
}

func TestCheckGitignore(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, dir string)
		wantContent string
	}{
		{
			name: "creates missing gitignore",
			setup: func(t *testing.T, dir string) {
				t.Helper()
			},
			wantContent: ".urvtemp\n",
		},
		{
			name: "appends to existing gitignore",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules\n"), 0o664); err != nil {
					t.Fatal(err)
				}
			},
			wantContent: "node_modules\n.urvtemp\n",
		},
		{
			name: "adds newline before appending when needed",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules"), 0o664); err != nil {
					t.Fatal(err)
				}
			},
			wantContent: "node_modules\n.urvtemp\n",
		},
		{
			name: "does not duplicate existing entry",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules\n.urvtemp\n"), 0o664); err != nil {
					t.Fatal(err)
				}
			},
			wantContent: "node_modules\n.urvtemp\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			if err := CheckGitignore(dir); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}

			if string(data) != tt.wantContent {
				t.Fatalf("expected %q, got %q", tt.wantContent, string(data))
			}
		})
	}
}

func TestGetCurrentRepoPath(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string) string
	}{
		{
			name: "returns repo root from nested current directory",
			setup: func(t *testing.T, dir string) string {
				t.Helper()

				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}

				nestedDir := filepath.Join(dir, "one", "two")
				if err := os.MkdirAll(nestedDir, 0o755); err != nil {
					t.Fatal(err)
				}

				t.Chdir(nestedDir)

				return dir
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			wantPath := tt.setup(t, dir)

			got, err := GetCurrentRepoPath()
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got != wantPath {
				t.Fatalf("expected %q, got %q", wantPath, got)
			}
		})
	}
}

func TestIgnoredFilesReportsGitIgnoredConfiguredFiles(t *testing.T) {
	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	if err := os.WriteFile(filepath.Join(repoPath, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := IgnoredFiles(repoPath, []string{".env", "plain.txt"})

	if err != nil {
		t.Fatalf("expected ignored check to succeed, got %v", err)
	}
	if !got[".env"] {
		t.Fatalf("expected .env ignored, got %#v", got)
	}
	if got["plain.txt"] {
		t.Fatalf("expected plain.txt not ignored, got %#v", got)
	}
}

func TestStagedFilesReportsConfiguredFilesInIndex(t *testing.T) {
	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.email", "test@example.invalid")
	runGit(t, repoPath, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoPath, "add", ".env")

	got, err := StagedFiles(repoPath, []string{".env", "plain.txt"})

	if err != nil {
		t.Fatalf("expected staged check to succeed, got %v", err)
	}
	if !got[".env"] {
		t.Fatalf("expected .env staged, got %#v", got)
	}
	if got["plain.txt"] {
		t.Fatalf("expected plain.txt unstaged, got %#v", got)
	}
}

func TestStagedFilesPreservesWhitespaceInConfiguredPath(t *testing.T) {
	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	fileName := " secret.env "
	if err := os.WriteFile(filepath.Join(repoPath, fileName), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoPath, "add", fileName)

	got, err := StagedFiles(repoPath, []string{fileName})

	if err != nil {
		t.Fatalf("expected staged check to succeed, got %v", err)
	}
	if !got[fileName] {
		t.Fatalf("expected whitespace path staged, got %#v", got)
	}
}
