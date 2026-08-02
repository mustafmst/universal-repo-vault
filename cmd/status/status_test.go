package status

import (
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/app"
)

func TestFormatReportSafe(t *testing.T) {
	report := &app.StatusReport{
		Overall:        app.OverallSafe,
		ConfigOK:       true,
		VaultOK:        true,
		VaultExists:    true,
		KeyMapped:      true,
		KeyName:        "repo-key",
		KeyFileExists:  true,
		KeyLengthValid: true,
		Files: []app.StatusFile{
			{Path: ".env", Status: app.FileUnchanged},
		},
	}

	got := FormatReport(report)

	for _, want := range []string{
		"Overall: safe",
		"Config: ok",
		"Vault: ok",
		"Key: repo-key",
		".env unchanged",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestFormatReportBrokenSetup(t *testing.T) {
	report := &app.StatusReport{
		Overall: app.OverallBrokenSetup,
		Errors:  []string{"vault .urv.vault.yaml is missing", "key for repo not found: /repo"},
	}

	got := FormatReport(report)

	for _, want := range []string{
		"Overall: broken setup",
		"Errors:",
		"vault .urv.vault.yaml is missing",
		"key for repo not found",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestFormatReportNeedsEncryption(t *testing.T) {
	report := &app.StatusReport{
		Overall:  app.OverallNeedsEncryption,
		ConfigOK: true,
		VaultOK:  true,
		Files: []app.StatusFile{
			{Path: ".env", Status: app.FileChanged},
			{Path: "new.secret.yaml", Status: app.FileNew},
		},
		Warnings: []string{"pattern matched no files: *.missing.*"},
	}

	got := FormatReport(report)

	for _, want := range []string{
		"Overall: needs encryption",
		"Warnings:",
		"pattern matched no files",
		".env changed",
		"new.secret.yaml new",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}
