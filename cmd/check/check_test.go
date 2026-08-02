package check

import (
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/app"
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
