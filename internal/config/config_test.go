package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string) string
		want       *Config
		wantErr    bool
		errMessage string
	}{
		{
			name: "loads secret files from config",
			setup: func(t *testing.T, dir string) string {
				t.Helper()

				contents := []byte("secretfiles:\n  - .env\n  - config/secrets.yaml\n")
				if err := os.WriteFile(filepath.Join(dir, configFileName), contents, 0o644); err != nil {
					t.Fatal(err)
				}

				return dir
			},
			want: &Config{SecretFiles: []string{".env", "config/secrets.yaml"}},
		},
		{
			name: "wraps missing config errors",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				return dir
			},
			wantErr:    true,
			errMessage: "reading config",
		},
		{
			name: "wraps invalid yaml errors",
			setup: func(t *testing.T, dir string) string {
				t.Helper()

				if err := os.WriteFile(filepath.Join(dir, configFileName), []byte("secretfiles: [\n"), 0o644); err != nil {
					t.Fatal(err)
				}

				return dir
			},
			wantErr:    true,
			errMessage: "reading config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			got, err := Load(tt.setup(t, dir))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMessage != "" && !strings.Contains(err.Error(), tt.errMessage) {
					t.Fatalf("expected error to contain %q, got %v", tt.errMessage, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected %#v, got %#v", tt.want, got)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string)
		wantErr    bool
		errMessage string
	}{
		{
			name: "creates empty config",
			setup: func(t *testing.T, dir string) {
				t.Helper()
			},
		},
		{
			name: "does not overwrite existing config",
			setup: func(t *testing.T, dir string) {
				t.Helper()

				if err := os.WriteFile(filepath.Join(dir, configFileName), []byte("secretfiles:\n  - .env\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr:    true,
			errMessage: "initializing config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			err := Initialize(dir)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMessage != "" && !strings.Contains(err.Error(), tt.errMessage) {
					t.Fatalf("expected error to contain %q, got %v", tt.errMessage, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			got, err := Load(dir)
			if err != nil {
				t.Fatalf("expected initialized config to load, got %v", err)
			}

			want := &Config{SecretFiles: []string{".env"}, Patterns: []string{"*.secret.*"}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("expected %#v, got %#v", want, got)
			}
		})
	}
}
