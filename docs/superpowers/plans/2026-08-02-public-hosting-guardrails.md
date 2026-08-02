# Public Hosting Guardrails Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make URV safer for public-hosted personal and small-team repositories by adding safe decrypt controls, a scriptable safety check, consistent key validation, and public-hosting documentation.

**Architecture:** Keep Cobra commands thin and place workflow decisions in `internal/app`. Reuse the existing status model for `urv check`, extend archive/decrypt boundaries only enough to support dry-run and no-overwrite behavior, and centralize key parsing inside `internal/keystore`.

**Tech Stack:** Go 1.26.1 module, Cobra CLI, existing `internal/*` packages, standard library tests, no new runtime dependencies.

## Global Constraints

- Serve personal homelab users and small teams using public or private Git hosting.
- Do not change `.urv.yaml`, `.urv.vault.yaml`, key files, or mapping file formats.
- Do not migrate away from ZIP or AES-GCM.
- Do not delete plaintext secret files.
- Do not print secret key material.
- Keep `cmd/*` thin: parse flags, call `internal/app`, format output.
- Keep `internal/app` responsible for workflows and safety decisions.
- Keep `internal/archive` responsible for ZIP archive inspection and unpacking.
- Keep `internal/keystore` responsible for local key parsing and health.
- Preserve default `urv decrypt` overwrite behavior for compatibility in this milestone; add safer flags without changing the default.
- Defer vault atomic writes, archive decompression limits, and full key rotation to separate plans.

---

## File Structure

- Modify `internal/keystore/keystore.go`: add shared key validation helper used by `KeyForRepo` and `HealthForRepo`.
- Modify `internal/keystore/keystore_test.go`: cover non-hex keys through `KeyForRepo`.
- Modify `internal/archive/archive.go`: add archive entry planning so decrypt dry-run can report planned writes without writing files.
- Modify `internal/archive/archive_test.go`: cover planned create/overwrite states and unsafe path rejection during planning.
- Modify `internal/app/decrypt.go`: add `DecryptOptions`, `DecryptResult`, `PlanDecryptRepo`, and option-aware decrypt execution.
- Modify `internal/app/app_test.go`: cover decrypt dry-run/no-overwrite/default-overwrite workflows.
- Modify `cmd/decrypt/decrypt.go`: add `--dry-run` and `--no-overwrite` flags and format dry-run output.
- Add `cmd/decrypt/decrypt_test.go`: cover dry-run output formatting and the command's new flag surface.
- Modify `internal/app/status.go`: expose enough status data for `urv check` without moving formatting into `internal/app`.
- Add `internal/app/check.go`: convert `StatusReport` into scriptable check results.
- Add `internal/app/check_test.go`: cover exit-worthy unsafe states and warnings.
- Add `cmd/check/check.go`: add `urv check`.
- Add `cmd/check/check_test.go`: cover formatter and command behavior.
- Modify `cmd/root.go` and `cmd/root_test.go`: wire `check`.
- Modify `internal/repo/checks.go`: add focused Git helpers for ignored and staged configured files.
- Modify `internal/repo/checks_test.go`: cover ignored and staged plaintext detection using temporary Git repositories.
- Modify `README.md`: document public-hosting warnings, safe decrypt flags, `urv check`, metadata disclosure, dummy examples, and safe manual key transfer.

---

### Task 1: Centralize Key Validation

**Files:**
- Modify: `internal/keystore/keystore.go`
- Modify: `internal/keystore/keystore_test.go`

**Interfaces:**
- Consumes: existing `KeyLength`, `KeyForRepo`, `HealthForRepo`
- Produces:
  - `func validateHexKey(keyFile string, raw []byte) (string, error)`
  - `KeyForRepo` rejects non-hex 64-character keys with an `invalid key encoding` error.
  - `HealthForRepo` uses the same helper without returning decoded key bytes.

- [ ] **Step 1: Write the failing test**

Add this test to `internal/keystore/keystore_test.go`:

```go
func TestFileStoreKeyForRepoRejectsNonHexKey(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(t.TempDir(), "repo")
	configPath := filepath.Join(home, ".config", "urv")
	if err := os.MkdirAll(filepath.Join(configPath, "keys"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "mapping.yaml"), []byte(repoPath+": repo-key\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "keys", "repo-key"), []byte(strings.Repeat("z", 64)), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := NewFileStore(home).KeyForRepo(repoPath)

	if err == nil {
		t.Fatal("expected invalid key encoding error, got nil")
	}
	if got != "" {
		t.Fatalf("expected empty key, got %q", got)
	}
	if !strings.Contains(err.Error(), "invalid key encoding") {
		t.Fatalf("expected invalid encoding error, got %v", err)
	}
}
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/keystore`

Expected: FAIL because `KeyForRepo` currently accepts a 64-character non-hex key and returns it.

- [ ] **Step 3: Implement shared validation**

Add this helper to `internal/keystore/keystore.go`:

```go
func validateHexKey(keyFile string, raw []byte) (string, error) {
	key := strings.TrimSpace(string(raw))
	if len(key) != 2*KeyLength {
		return "", fmt.Errorf("reading key from: %s, expected key len: %d, read: %d", keyFile, 2*KeyLength, len(key))
	}
	keyBytes, err := hex.DecodeString(key)
	if err != nil || len(keyBytes) != KeyLength {
		return "", fmt.Errorf("reading key from: %s, invalid key encoding", keyFile)
	}
	return key, nil
}
```

Update `KeyForRepo`:

```go
raw, err := os.ReadFile(keyFile)
if err != nil {
	return "", err
}
return validateHexKey(keyFile, raw)
```

Update `HealthForRepo`:

```go
health.KeyFileExists = true
if _, err := validateHexKey(keyFile, raw); err != nil {
	health.Err = err
	return health
}
health.KeyLengthValid = true
return health
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/keystore`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/keystore/keystore.go internal/keystore/keystore_test.go
git commit -m "fix: share key validation"
```

---

### Task 2: Add Decrypt Planning And Safe Flags

**Files:**
- Modify: `internal/archive/archive.go`
- Modify: `internal/archive/archive_test.go`
- Modify: `internal/app/decrypt.go`
- Modify: `internal/app/app_test.go`
- Modify: `cmd/decrypt/decrypt.go`
- Add: `cmd/decrypt/decrypt_test.go`

**Interfaces:**
- Produces:
  - `type EntryAction string`
  - `const EntryCreate EntryAction = "create"`
  - `const EntryOverwrite EntryAction = "overwrite"`
  - `type EntryPlan struct { Path string; Action EntryAction; Mode os.FileMode }`
  - `func (za *ZipArchiver) PlanUnpack(basePath string, data []byte) ([]EntryPlan, error)`
  - `type DecryptOptions struct { DryRun bool; Overwrite bool }`
  - `type DecryptResult struct { DryRun bool; Files []archive.EntryPlan }`
  - `func PlanDecryptRepo(repoPath string) (*DecryptResult, error)`
  - `func DecryptRepoWithOptions(repoPath string, options DecryptOptions) (*DecryptResult, error)`
  - `func DecryptRepoWithServicesAndOptions(repoPath string, services Services, options DecryptOptions) (*DecryptResult, error)`

- [ ] **Step 1: Write archive planning tests**

Add tests to `internal/archive/archive_test.go`:

```go
func TestZipArchiverPlanUnpackReportsCreateAndOverwrite(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeArchiveFile(t, filepath.Join(src, ".env"), "new\n")
	writeArchiveFile(t, filepath.Join(src, "nested", "app.secret.yaml"), "secret\n")
	writeArchiveFile(t, filepath.Join(dst, ".env"), "old\n")

	data, err := NewZipArchiver().Pack(src, []string{".env", "nested/app.secret.yaml"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := NewZipArchiver().PlanUnpack(dst, data)

	if err != nil {
		t.Fatalf("expected plan to succeed, got %v", err)
	}
	want := []EntryPlan{
		{Path: ".env", Action: EntryOverwrite, Mode: 0o644},
		{Path: "nested/app.secret.yaml", Action: EntryCreate, Mode: 0o644},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
	for i := range want {
		if got[i].Path != want[i].Path || got[i].Action != want[i].Action {
			t.Fatalf("expected %#v, got %#v", want, got)
		}
	}
}

func TestZipArchiverPlanUnpackRejectsUnsafePath(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../outside.env")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("secret")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = NewZipArchiver().PlanUnpack(t.TempDir(), buf.Bytes())

	if err == nil {
		t.Fatal("expected unsafe path error, got nil")
	}
	if !strings.Contains(err.Error(), "unsafe zip path") {
		t.Fatalf("expected unsafe path error, got %v", err)
	}
}
```

- [ ] **Step 2: Run archive tests to verify they fail**

Run: `go test ./internal/archive`

Expected: FAIL because `PlanUnpack`, `EntryPlan`, and constants are undefined.

- [ ] **Step 3: Implement archive planning**

Add to `internal/archive/archive.go`:

```go
type EntryAction string

const (
	EntryCreate    EntryAction = "create"
	EntryOverwrite EntryAction = "overwrite"
)

type EntryPlan struct {
	Path   string
	Action EntryAction
	Mode   os.FileMode
}

func (za *ZipArchiver) PlanUnpack(basePath string, data []byte) ([]EntryPlan, error) {
	dataReader := bytes.NewReader(data)
	zr, err := zip.NewReader(dataReader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("creating zip reader from bytes data: %w", err)
	}

	plans := make([]EntryPlan, 0, len(zr.File))
	for _, zf := range zr.File {
		cleanName := filepath.ToSlash(filepath.Clean(zf.Name))
		if !filepath.IsLocal(cleanName) {
			return nil, fmt.Errorf("unsafe zip path: %s", zf.Name)
		}

		action := EntryCreate
		if info, err := os.Stat(filepath.Join(basePath, filepath.FromSlash(cleanName))); err == nil && !info.IsDir() {
			action = EntryOverwrite
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspecting unpack target %s: %w", cleanName, err)
		}

		plans = append(plans, EntryPlan{Path: cleanName, Action: action, Mode: zf.Mode()})
	}
	return plans, nil
}
```

- [ ] **Step 4: Write app decrypt option tests**

Add to `internal/app/app_test.go`:

```go
func TestDecryptRepoWithOptionsDryRunDoesNotWriteFiles(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}

	result, err := DecryptRepoWithServicesAndOptions(repoPath, services, DecryptOptions{DryRun: true, Overwrite: true})

	if err != nil {
		t.Fatalf("expected dry run to succeed, got %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected dry run result")
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".env")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry run not to restore file, got %v", err)
	}
}

func TestDecryptRepoWithOptionsNoOverwriteFailsOnExistingFile(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}

	_, err := DecryptRepoWithServicesAndOptions(repoPath, services, DecryptOptions{Overwrite: false})

	if err == nil {
		t.Fatal("expected no-overwrite decrypt to fail")
	}
	if !strings.Contains(err.Error(), "file was not replaced") && !strings.Contains(err.Error(), "file exists") {
		t.Fatalf("expected existing file error, got %v", err)
	}
}

func TestDecryptRepoDefaultStillOverwrites(t *testing.T) {
	repoPath, homePath := setupRepoAndHome(t)
	services := Services{KeyStore: keystore.NewFileStore(homePath)}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repoPath, ".env"), "LOCAL=changed\n", 0o600)

	if _, err := DecryptRepoWithServicesAndOptions(repoPath, services, DecryptOptions{Overwrite: true}); err != nil {
		t.Fatalf("expected overwrite decrypt to succeed, got %v", err)
	}
	got, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "API_KEY=one\n" {
		t.Fatalf("expected vault contents, got %q", string(got))
	}
}
```

Add missing imports if needed:

```go
import (
	"errors"
	"strings"
)
```

- [ ] **Step 5: Implement app decrypt options**

Modify `internal/app/decrypt.go`:

```go
type DecryptOptions struct {
	DryRun    bool
	Overwrite bool
}

type DecryptResult struct {
	DryRun bool
	Files  []archive.EntryPlan
}

func DecryptRepo(repoPath string) error {
	_, err := DecryptRepoWithOptions(repoPath, DecryptOptions{Overwrite: true})
	return err
}

func DecryptRepoWithOptions(repoPath string, options DecryptOptions) (*DecryptResult, error) {
	return DecryptRepoWithServicesAndOptions(repoPath, DefaultServices(), options)
}

func PlanDecryptRepo(repoPath string) (*DecryptResult, error) {
	return DecryptRepoWithOptions(repoPath, DecryptOptions{DryRun: true, Overwrite: true})
}
```

Move the existing decrypt workflow body into `DecryptRepoWithServicesAndOptions`. After decrypting the archive bytes:

```go
files, err := services.Archiver.PlanUnpack(repoPath, decryptedArch)
if err != nil {
	return nil, err
}
result := &DecryptResult{DryRun: options.DryRun, Files: files}
if options.DryRun {
	return result, nil
}
return result, services.Archiver.Unpack(repoPath, decryptedArch, options.Overwrite)
```

Update `archive.Archiver` in `internal/archive/archive.go`:

```go
type Archiver interface {
	Pack(basePath string, relPaths []string) ([]byte, error)
	PlanUnpack(basePath string, data []byte) ([]EntryPlan, error)
	Unpack(basePath string, data []byte, overwrite bool) error
}
```

- [ ] **Step 6: Add decrypt command flags and tests**

Create `cmd/decrypt/decrypt_test.go`:

```go
package decrypt

import (
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/archive"
	"github.com/mustafmst/universal-repo-vault/internal/app"
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
```

Modify `cmd/decrypt/decrypt.go`:

```go
cmd := &cobra.Command{
	Use:   "decrypt",
	Short: "Decrypt vault files into the repository",
	Args:  cobra.NoArgs,
	RunE:  runDecrypt,
}
cmd.Flags().Bool("dry-run", false, "show files that would be decrypted without writing them")
cmd.Flags().Bool("no-overwrite", false, "fail if decrypt would replace an existing file")
return cmd
```

In `runDecrypt`, read flags and call the option-aware app function:

```go
dryRun, err := cmd.Flags().GetBool("dry-run")
if err != nil {
	return err
}
noOverwrite, err := cmd.Flags().GetBool("no-overwrite")
if err != nil {
	return err
}

result, err := app.DecryptRepoWithOptions(repoPath, app.DecryptOptions{DryRun: dryRun, Overwrite: !noOverwrite})
if err != nil {
	return err
}
if dryRun {
	_, err = fmt.Fprint(cmd.OutOrStdout(), FormatDryRunResult(result))
	return err
}
_, err = fmt.Fprintln(cmd.OutOrStdout(), "Vault unpacked successfully")
return err
```

Add formatter:

```go
func FormatDryRunResult(result *app.DecryptResult) string {
	var b strings.Builder
	b.WriteString("Decrypt dry run:\n")
	for _, file := range result.Files {
		fmt.Fprintf(&b, "  %s %s\n", file.Path, file.Action)
	}
	return b.String()
}
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/archive ./internal/app ./cmd/decrypt
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/archive/archive.go internal/archive/archive_test.go internal/app/decrypt.go internal/app/app_test.go cmd/decrypt/decrypt.go cmd/decrypt/decrypt_test.go
git commit -m "feat: add safe decrypt options"
```

---

### Task 3: Add Scriptable `urv check`

**Files:**
- Add: `internal/app/check.go`
- Add: `internal/app/check_test.go`
- Add: `cmd/check/check.go`
- Add: `cmd/check/check_test.go`
- Modify: `cmd/root.go`
- Modify: `cmd/root_test.go`

**Interfaces:**
- Produces:
  - `type CheckResult struct { Report *StatusReport; Safe bool; Messages []string }`
  - `func CheckRepo(repoPath string) (*CheckResult, error)`
  - `func CheckRepoWithServices(repoPath string, services Services) (*CheckResult, error)`
  - `func NewCommand() *cobra.Command` in `cmd/check`

- [ ] **Step 1: Write app check tests**

Create `internal/app/check_test.go`:

```go
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/keystore"
)

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
```

- [ ] **Step 2: Run app check tests to verify they fail**

Run: `go test ./internal/app`

Expected: FAIL because `CheckRepoWithServices` is undefined.

- [ ] **Step 3: Implement app check**

Create `internal/app/check.go`:

```go
package app

import "fmt"

type CheckResult struct {
	Report   *StatusReport
	Safe     bool
	Messages []string
}

func CheckRepo(repoPath string) (*CheckResult, error) {
	return CheckRepoWithServices(repoPath, DefaultServices())
}

func CheckRepoWithServices(repoPath string, services Services) (*CheckResult, error) {
	report, err := StatusRepoWithServices(repoPath, services)
	if err != nil {
		return nil, err
	}

	result := &CheckResult{Report: report, Safe: report.Overall == OverallSafe}
	result.Messages = append(result.Messages, report.Errors...)
	for _, file := range report.Files {
		if file.Status != FileUnchanged {
			result.Messages = append(result.Messages, fmt.Sprintf("%s %s", file.Path, file.Status))
		}
	}
	return result, nil
}
```

- [ ] **Step 4: Write command tests**

Create `cmd/check/check_test.go`:

```go
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
```

Update `cmd/root_test.go` expected root commands:

```go
want := []string{"init", "encrypt", "decrypt", "keys", "status", "check"}
```

- [ ] **Step 5: Implement command**

Create `cmd/check/check.go`:

```go
package check

import (
	"fmt"
	"strings"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Fail when repository vault safety status is unsafe",
		Args:  cobra.NoArgs,
		RunE:  runCheck,
	}
}

func runCheck(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}

	result, err := app.CheckRepo(repoPath)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprint(cmd.OutOrStdout(), FormatCheckResult(result)); err != nil {
		return err
	}
	if !result.Safe {
		return fmt.Errorf("repository is not safe to commit")
	}
	return nil
}

func FormatCheckResult(result *app.CheckResult) string {
	var b strings.Builder
	if result.Safe {
		b.WriteString("URV check passed\n")
		return b.String()
	}
	b.WriteString("URV check failed\n")
	for _, msg := range result.Messages {
		fmt.Fprintf(&b, "  %s\n", msg)
	}
	return b.String()
}
```

Wire root in `cmd/root.go`:

```go
import "github.com/mustafmst/universal-repo-vault/cmd/check"
```

```go
cmd.AddCommand(check.NewCommand())
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/app ./cmd ./cmd/check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/check.go internal/app/check_test.go cmd/check/check.go cmd/check/check_test.go cmd/root.go cmd/root_test.go
git commit -m "feat: add repository safety check"
```

---

### Task 4: Add Git Public-Hosting Checks

**Files:**
- Modify: `internal/repo/checks.go`
- Modify: `internal/repo/checks_test.go`
- Modify: `internal/app/status.go`
- Modify: `internal/app/status_test.go`
- Modify: `internal/app/check.go`
- Modify: `internal/app/check_test.go`

**Interfaces:**
- Produces:
  - `func IgnoredFiles(repoPath string, relPaths []string) (map[string]bool, error)`
  - `func StagedFiles(repoPath string, relPaths []string) (map[string]bool, error)`
  - `StatusReport.Warnings` includes configured plaintext files not ignored by Git.
  - `CheckResult.Safe` is false when configured plaintext files are staged.

- [ ] **Step 1: Write repo Git helper tests**

Add tests to `internal/repo/checks_test.go`:

```go
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
```

If `runGit` does not exist in the file, add:

```go
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
```

- [ ] **Step 2: Implement repo helpers**

Add to `internal/repo/checks.go`:

```go
func IgnoredFiles(repoPath string, relPaths []string) (map[string]bool, error) {
	result := map[string]bool{}
	for _, path := range relPaths {
		result[path] = false
		cmd := exec.Command("git", "check-ignore", "--quiet", "--", path)
		cmd.Dir = repoPath
		err := cmd.Run()
		if err == nil {
			result[path] = true
			continue
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			continue
		}
		return nil, fmt.Errorf("checking ignored file %s: %w", path, err)
	}
	return result, nil
}

func StagedFiles(repoPath string, relPaths []string) (map[string]bool, error) {
	result := map[string]bool{}
	for _, path := range relPaths {
		result[path] = false
	}

	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing staged files: %w", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := result[line]; ok {
			result[line] = true
		}
	}
	return result, nil
}
```

Add imports:

```go
import "os/exec"
```

- [ ] **Step 3: Extend status and check tests**

Add to `internal/app/status_test.go`:

```go
func TestStatusRepoWarnsWhenConfiguredFileIsNotIgnored(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - secret.env\n", 0o644)
	writeStatusFile(t, filepath.Join(repoPath, "secret.env"), "secret\n", 0o600)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected status to inspect repo, got %v", err)
	}
	if !hasStatusMessage(got.Warnings, "configured plaintext file is not ignored by Git") {
		t.Fatalf("expected not-ignored warning, got %#v", got.Warnings)
	}
}
```

Add to `internal/app/check_test.go`:

```go
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
```

Add helper if needed:

```go
func runAppGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
```

- [ ] **Step 4: Implement app Git checks**

In `internal/app/status.go`, after file discovery and reserved-file validation, call `repo.IgnoredFiles`:

```go
ignored, err := repo.IgnoredFiles(repoPath, foundFiles)
if err != nil {
	return nil, err
}
for _, path := range foundFiles {
	if !ignored[path] {
		report.Warnings = append(report.Warnings, fmt.Sprintf("configured plaintext file is not ignored by Git: %s", path))
	}
}
```

In `internal/app/check.go`, before returning, call `repo.StagedFiles` for `report.Files` paths:

```go
paths := make([]string, 0, len(report.Files))
for _, file := range report.Files {
	if file.Status != FileVaultOnly {
		paths = append(paths, file.Path)
	}
}
staged, err := repo.StagedFiles(repoPath, paths)
if err != nil {
	return nil, err
}
for _, path := range paths {
	if staged[path] {
		result.Safe = false
		result.Messages = append(result.Messages, fmt.Sprintf("configured plaintext file is staged: %s", path))
	}
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/repo ./internal/app
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/repo/checks.go internal/repo/checks_test.go internal/app/status.go internal/app/status_test.go internal/app/check.go internal/app/check_test.go
git commit -m "feat: check git public hosting safety"
```

---

### Task 5: Document Public Hosting Release Guardrails

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: `urv decrypt --dry-run`, `urv decrypt --no-overwrite`, `urv check`, key validation behavior, and Git public-hosting warnings.
- Produces: README sections for safe decrypt, public hosting checklist, vault metadata disclosure, dummy examples, and manual key transfer.

- [ ] **Step 1: Update command list**

In README Commands section, add:

````markdown
Check whether the repository is safe for commit or automation:

```sh
urv check
```

Preview decrypt writes without changing files:

```sh
urv decrypt --dry-run
```

Decrypt without replacing existing files:

```sh
urv decrypt --no-overwrite
```
````

- [ ] **Step 2: Add public-hosting warning section**

Add this section before Key Management:

````markdown
## Public Hosting Notes

URV is designed to commit `.urv.yaml` and `.urv.vault.yaml`, but those files are not fully private metadata. `.urv.vault.yaml` exposes protected repository-relative file paths and SHA-256 hashes of plaintext file contents. Do not use sensitive hostnames, service names, or environment names in secret file paths if that metadata should stay private.

Before pushing to a public repository:

```sh
urv status
urv check
```

`urv check` exits non-zero when the repository is not safe to commit. It is intended for local scripts, CI, and pre-commit hooks.

The files under `example-files/` contain dummy values only. Do not copy real secrets into tracked example files.
````

- [ ] **Step 3: Add safe key sharing note**

In Key Management, add:

```markdown
For small teams, transfer key files only through a private channel that is already trusted for secrets. Do not paste URV keys into public issues, pull requests, chat rooms, shell history, or commit messages. After importing or mapping a key, run `urv status` to verify that the repository can see a valid mapped key.
```

- [ ] **Step 4: Run final verification**

Run:

```bash
gofmt -w internal/archive/archive.go internal/archive/archive_test.go internal/app/decrypt.go internal/app/app_test.go internal/app/check.go internal/app/check_test.go internal/app/status.go internal/app/status_test.go internal/keystore/keystore.go internal/keystore/keystore_test.go internal/repo/checks.go internal/repo/checks_test.go cmd/decrypt/decrypt.go cmd/decrypt/decrypt_test.go cmd/check/check.go cmd/check/check_test.go cmd/root.go cmd/root_test.go
go test ./...
go vet ./...
go run ./main.go status
go run ./main.go check
```

Expected:

- `gofmt` produces no output.
- `go test ./...` passes.
- `go vet ./...` passes.
- `go run ./main.go status` exits 0 and prints an `Overall:` line.
- `go run ./main.go check` may exit non-zero in this repository if the current local key mapping is missing or public-hosting checks fail; document the exact output in the task report rather than treating an unsafe status as a failed smoke test.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: document public hosting guardrails"
```

---

## Final Verification

- [ ] Run: `gofmt -w internal/archive/archive.go internal/archive/archive_test.go internal/app/decrypt.go internal/app/app_test.go internal/app/check.go internal/app/check_test.go internal/app/status.go internal/app/status_test.go internal/keystore/keystore.go internal/keystore/keystore_test.go internal/repo/checks.go internal/repo/checks_test.go cmd/decrypt/decrypt.go cmd/decrypt/decrypt_test.go cmd/check/check.go cmd/check/check_test.go cmd/root.go cmd/root_test.go`
- [ ] Run: `go test ./...`
- [ ] Run: `go vet ./...`
- [ ] Run: `go run ./main.go status`
- [ ] Run: `go run ./main.go check`
- [ ] Confirm `git status --short` is clean after commits.

## Deferred Follow-Up Plans

- Atomic vault writes.
- Vault file permission adjustment from `0664` to `0644`.
- Archive decompressed-size and file-count limits.
- Key fingerprint and `urv keys current`.
- Key rotation.
- Future vault format that encrypts metadata or replaces plaintext hashes with keyed hashes.

## Self-Review Notes

- Spec coverage: this plan addresses both release blockers, vault metadata documentation, shared key validation, public-hosting guidance, safe manual key transfer documentation, and the recommended next milestone.
- Scope check: atomic vault writes and archive limits are intentionally deferred because they are independent hardening tasks.
- Type consistency: decrypt planning types live in `internal/archive`, decrypt workflow types live in `internal/app`, and CLI formatters consume app results without duplicating workflow logic.
