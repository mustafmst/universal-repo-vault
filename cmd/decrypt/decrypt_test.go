package decrypt

import (
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/archive"
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
