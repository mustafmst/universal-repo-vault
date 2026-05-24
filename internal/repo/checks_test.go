package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
