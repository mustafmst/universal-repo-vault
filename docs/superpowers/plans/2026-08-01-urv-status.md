# URV Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `urv status`, a read-only command that reports repository vault safety state without changing files.

**Architecture:** Add a shared status aggregation model in `internal/app`, then expose it through a thin Cobra command in `cmd/status`. The status model reads config, vault metadata, local key mapping, configured files, and file hashes, but command packages own human-readable formatting.

**Tech Stack:** Go 1.26.1 module, Cobra CLI, Viper/YAML config, existing internal packages under `internal/*`, standard library tests.

## Global Constraints

- Serve a single-user homelab repository first.
- Stay simple, local-first, and hard to misuse.
- Do not change `.urv.yaml`, `.urv.vault.yaml`, key files, or mapping files.
- Do not migrate away from ZIP or AES-GCM.
- Do not delete plaintext secret files.
- `cmd/*` remains thin: parse flags, call `internal/app`, format output.
- `internal/app` owns safety workflows and status aggregation.
- `internal/files` owns configured file discovery and hashing.
- `internal/vault` owns vault file parsing, validation, and metadata access.
- `internal/keystore` owns local key and mapping health.
- `urv status` returns success when inspection completes, even when it reports unsafe states.

---

## File Structure

- Create `internal/app/status.go`: status types, `StatusRepo`, `StatusRepoWithServices`, config validation helpers, file classification, and key/vault status aggregation.
- Create `internal/app/status_test.go`: focused model tests for setup, vault, key, config, and file states.
- Modify `internal/keystore/keystore.go`: add non-secret key health helpers so status can inspect mappings without returning key material.
- Modify `internal/keystore/keystore_test.go`: test the new key health helper for mapped, missing mapping, missing file, and invalid key length states.
- Create `cmd/status/status.go`: Cobra command and human-readable formatter.
- Create `cmd/status/status_test.go`: command output tests using `app.StatusReport` fixtures.
- Modify `cmd/root.go`: wire `status.NewCommand()`.
- Modify `cmd/root_test.go`: assert root includes `status`.
- Modify `README.md`: document the new status workflow.

---

### Task 1: Add Key Health Helper

**Files:**
- Modify: `internal/keystore/keystore.go`
- Modify: `internal/keystore/keystore_test.go`

**Interfaces:**
- Consumes: `(*FileStore).Mapping() (*KeyMapping, error)`, `(*KeyMapping).Get(repo string) (string, error)`, `(*FileStore).keyPath(keyName string) string`
- Produces:
  - `type KeyHealth struct { Mapped bool; KeyName string; KeyFileExists bool; KeyLengthValid bool; Err error }`
  - `func (fs *FileStore) HealthForRepo(repoPath string) KeyHealth`

- [ ] **Step 1: Write failing tests for key health**

Add these tests to `internal/keystore/keystore_test.go`:

```go
func TestFileStoreHealthForRepoMappedValidKey(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(t.TempDir(), "repo")
	store := NewFileStore(home)
	if err := store.SaveKey(keystoreTestKey, repoPath, "repo-key"); err != nil {
		t.Fatal(err)
	}

	got := store.HealthForRepo(repoPath)

	if got.Err != nil {
		t.Fatalf("expected no error, got %v", got.Err)
	}
	if !got.Mapped {
		t.Fatal("expected mapped key")
	}
	if got.KeyName != "repo-key" {
		t.Fatalf("expected key name repo-key, got %q", got.KeyName)
	}
	if !got.KeyFileExists {
		t.Fatal("expected key file to exist")
	}
	if !got.KeyLengthValid {
		t.Fatal("expected valid key length")
	}
}

func TestFileStoreHealthForRepoMissingMapping(t *testing.T) {
	got := NewFileStore(t.TempDir()).HealthForRepo("/tmp/missing")

	if got.Mapped {
		t.Fatal("expected no mapping")
	}
	if got.KeyFileExists {
		t.Fatal("expected no key file")
	}
	if got.KeyLengthValid {
		t.Fatal("expected invalid key length")
	}
	if got.Err == nil || !strings.Contains(got.Err.Error(), "key for repo not found") {
		t.Fatalf("expected missing mapping error, got %v", got.Err)
	}
}

func TestFileStoreHealthForRepoMissingKeyFile(t *testing.T) {
	home := t.TempDir()
	repoPath := "/tmp/example-repo"
	configPath := filepath.Join(home, ".config", "urv")
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "mapping.yaml"), []byte(repoPath+": repo-key\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := NewFileStore(home).HealthForRepo(repoPath)

	if !got.Mapped {
		t.Fatal("expected mapping")
	}
	if got.KeyName != "repo-key" {
		t.Fatalf("expected repo-key, got %q", got.KeyName)
	}
	if got.KeyFileExists {
		t.Fatal("expected missing key file")
	}
	if got.KeyLengthValid {
		t.Fatal("expected invalid key length")
	}
	if got.Err == nil || !strings.Contains(got.Err.Error(), "key file") {
		t.Fatalf("expected missing key file error, got %v", got.Err)
	}
}

func TestFileStoreHealthForRepoInvalidKeyLength(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(t.TempDir(), "repo")
	configPath := filepath.Join(home, ".config", "urv")
	if err := os.MkdirAll(filepath.Join(configPath, "keys"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "mapping.yaml"), []byte(repoPath+": repo-key\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "keys", "repo-key"), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := NewFileStore(home).HealthForRepo(repoPath)

	if !got.Mapped {
		t.Fatal("expected mapping")
	}
	if !got.KeyFileExists {
		t.Fatal("expected key file")
	}
	if got.KeyLengthValid {
		t.Fatal("expected invalid key length")
	}
	if got.Err == nil || !strings.Contains(got.Err.Error(), "expected key len") {
		t.Fatalf("expected invalid length error, got %v", got.Err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/keystore`

Expected: FAIL because `HealthForRepo` and `KeyHealth` are undefined.

- [ ] **Step 3: Implement key health**

Add this type and method to `internal/keystore/keystore.go`:

```go
type KeyHealth struct {
	Mapped         bool
	KeyName        string
	KeyFileExists  bool
	KeyLengthValid bool
	Err            error
}

func (fs *FileStore) HealthForRepo(repoPath string) KeyHealth {
	mapping, err := fs.Mapping()
	if err != nil {
		return KeyHealth{Err: err}
	}

	keyName, err := mapping.Get(repoPath)
	if err != nil {
		return KeyHealth{Err: err}
	}

	health := KeyHealth{Mapped: true, KeyName: keyName}
	keyFile := fs.keyPath(keyName)
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			health.Err = fmt.Errorf("key file %s does not exists: %w", keyFile, err)
			return health
		}
		health.Err = err
		return health
	}

	health.KeyFileExists = true
	key := strings.TrimSpace(string(raw))
	if len(key) != 2*KeyLength {
		health.Err = fmt.Errorf("reading key from: %s, expected key len: %d, read: %d", keyFile, 2*KeyLength, len(key))
		return health
	}

	health.KeyLengthValid = true
	return health
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/keystore`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/keystore/keystore.go internal/keystore/keystore_test.go
git commit -m "feat: add key health inspection"
```

---

### Task 2: Add App Status Model

**Files:**
- Create: `internal/app/status.go`
- Create: `internal/app/status_test.go`

**Interfaces:**
- Consumes:
  - `config.Load(repoPath string) (*config.Config, error)`
  - `files.ListAllConfiguredFiles(basePath string, fileList []string, patternlist []string) ([]string, error)`
  - `files.NewFileHashCollection(basePath string, files []string) (*files.FileHashCollection, error)`
  - `vault.NewVaultFromFilePath(filePath string) (*vault.Vault, error)`
  - `(*keystore.FileStore).HealthForRepo(repoPath string) keystore.KeyHealth`
- Produces:
  - `type OverallStatus string`
  - `const OverallSafe OverallStatus = "safe"`
  - `const OverallNeedsEncryption OverallStatus = "needs encryption"`
  - `const OverallBrokenSetup OverallStatus = "broken setup"`
  - `type FileStatus string`
  - `const FileUnchanged FileStatus = "unchanged"`
  - `const FileChanged FileStatus = "changed"`
  - `const FileNew FileStatus = "new"`
  - `const FileMissing FileStatus = "missing"`
  - `const FileVaultOnly FileStatus = "vault-only"`
  - `type StatusFile struct { Path string; Status FileStatus }`
  - `type StatusReport struct { Overall OverallStatus; ConfigOK bool; VaultOK bool; VaultExists bool; KeyMapped bool; KeyName string; KeyFileExists bool; KeyLengthValid bool; Files []StatusFile; Warnings []string; Errors []string }`
  - `func StatusRepo(repoPath string) (*StatusReport, error)`
  - `func StatusRepoWithServices(repoPath string, services Services) (*StatusReport, error)`

- [ ] **Step 1: Write failing tests for missing setup states**

Create `internal/app/status_test.go` with these helpers and tests:

```go
package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/keystore"
)

func writeStatusFile(t *testing.T, path string, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func statusRepo(t *testing.T) (string, *keystore.FileStore) {
	t.Helper()
	repoPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repoPath, keystore.NewFileStore(t.TempDir())
}

func findStatusFile(files []StatusFile, path string) (StatusFile, bool) {
	for _, file := range files {
		if file.Path == path {
			return file, true
		}
	}
	return StatusFile{}, false
}

func hasStatusMessage(messages []string, want string) bool {
	for _, msg := range messages {
		if strings.Contains(msg, want) {
			return true
		}
	}
	return false
}

func TestStatusRepoReportsMissingConfig(t *testing.T) {
	repoPath, store := statusRepo(t)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallBrokenSetup {
		t.Fatalf("expected broken setup, got %q", got.Overall)
	}
	if got.ConfigOK {
		t.Fatal("expected config to be invalid")
	}
	if !hasStatusMessage(got.Errors, ".urv.yaml") {
		t.Fatalf("expected config error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsMissingVaultAndKey(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\n", 0o644)
	writeStatusFile(t, filepath.Join(repoPath, ".env"), "API_KEY=one\n", 0o600)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallBrokenSetup {
		t.Fatalf("expected broken setup, got %q", got.Overall)
	}
	if !got.ConfigOK {
		t.Fatal("expected config ok")
	}
	if got.VaultExists || got.VaultOK {
		t.Fatalf("expected missing vault, got exists=%v ok=%v", got.VaultExists, got.VaultOK)
	}
	if got.KeyMapped {
		t.Fatal("expected no mapped key")
	}
	if !hasStatusMessage(got.Errors, ".urv.vault.yaml") {
		t.Fatalf("expected missing vault error, got %#v", got.Errors)
	}
	if !hasStatusMessage(got.Errors, "key for repo not found") {
		t.Fatalf("expected missing key error, got %#v", got.Errors)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app`

Expected: FAIL because status types and functions are undefined.

- [ ] **Step 3: Implement initial status model**

Create `internal/app/status.go`:

```go
package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mustafmst/universal-repo-vault/internal/config"
	"github.com/mustafmst/universal-repo-vault/internal/files"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

type OverallStatus string

const (
	OverallSafe            OverallStatus = "safe"
	OverallNeedsEncryption OverallStatus = "needs encryption"
	OverallBrokenSetup     OverallStatus = "broken setup"
)

type FileStatus string

const (
	FileUnchanged FileStatus = "unchanged"
	FileChanged   FileStatus = "changed"
	FileNew       FileStatus = "new"
	FileMissing   FileStatus = "missing"
	FileVaultOnly FileStatus = "vault-only"
)

type StatusFile struct {
	Path   string
	Status FileStatus
}

type StatusReport struct {
	Overall        OverallStatus
	ConfigOK       bool
	VaultOK        bool
	VaultExists    bool
	KeyMapped      bool
	KeyName        string
	KeyFileExists  bool
	KeyLengthValid bool
	Files          []StatusFile
	Warnings       []string
	Errors         []string
}

func StatusRepo(repoPath string) (*StatusReport, error) {
	return StatusRepoWithServices(repoPath, DefaultServices())
}

func StatusRepoWithServices(repoPath string, services Services) (*StatusReport, error) {
	services = services.withDefaults()
	report := &StatusReport{Overall: OverallSafe}

	cfg, err := config.Load(repoPath)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("config .urv.yaml is missing or invalid: %v", err))
		report.finish()
		return report, nil
	}
	report.ConfigOK = true

	keyHealth := services.KeyStore.HealthForRepo(repoPath)
	report.KeyMapped = keyHealth.Mapped
	report.KeyName = keyHealth.KeyName
	report.KeyFileExists = keyHealth.KeyFileExists
	report.KeyLengthValid = keyHealth.KeyLengthValid
	if keyHealth.Err != nil {
		report.Errors = append(report.Errors, keyHealth.Err.Error())
	}

	v, err := vault.NewVaultFromFilePath(filepath.Join(repoPath, vault.VaultFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.Errors = append(report.Errors, "vault .urv.vault.yaml is missing")
		} else {
			report.Errors = append(report.Errors, fmt.Sprintf("vault .urv.vault.yaml is invalid: %v", err))
		}
	} else {
		report.VaultExists = true
		if err := v.ValidateForDecrypt(); err != nil {
			report.Errors = append(report.Errors, err.Error())
		} else if _, err := v.GetByteData(); err != nil {
			report.Errors = append(report.Errors, err.Error())
		} else {
			report.VaultOK = true
		}
	}

	foundFiles, err := files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
		report.finish()
		return report, nil
	}

	hashes := map[string]string{}
	if len(foundFiles) > 0 {
		hashCollection, err := files.NewFileHashCollection(repoPath, foundFiles)
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
		} else {
			hashes = hashCollection.Hashes
		}
	}

	report.Files = classifyStatusFiles(repoPath, cfg.SecretFiles, foundFiles, hashes, vaultHashes(v))
	report.finish()
	return report, nil
}

func vaultHashes(v *vault.Vault) map[string]string {
	if v == nil || v.Hashes == nil {
		return map[string]string{}
	}
	return v.Hashes
}

func classifyStatusFiles(repoPath string, explicit []string, discovered []string, currentHashes map[string]string, oldHashes map[string]string) []StatusFile {
	discoveredSet := map[string]struct{}{}
	for _, path := range discovered {
		discoveredSet[path] = struct{}{}
	}

	resultByPath := map[string]FileStatus{}
	for _, path := range discovered {
		oldHash, ok := oldHashes[path]
		switch {
		case !ok:
			resultByPath[path] = FileNew
		case currentHashes[path] != oldHash:
			resultByPath[path] = FileChanged
		default:
			resultByPath[path] = FileUnchanged
		}
	}

	for _, path := range explicit {
		cleanPath := filepath.ToSlash(filepath.Clean(path))
		if _, ok := discoveredSet[cleanPath]; ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(cleanPath))); errors.Is(err, os.ErrNotExist) {
			resultByPath[cleanPath] = FileMissing
		}
	}

	for path := range oldHashes {
		if _, ok := discoveredSet[path]; !ok {
			if _, already := resultByPath[path]; !already {
				resultByPath[path] = FileVaultOnly
			}
		}
	}

	paths := make([]string, 0, len(resultByPath))
	for path := range resultByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	result := make([]StatusFile, 0, len(paths))
	for _, path := range paths {
		result = append(result, StatusFile{Path: path, Status: resultByPath[path]})
	}
	return result
}

func (r *StatusReport) finish() {
	if len(r.Errors) > 0 {
		r.Overall = OverallBrokenSetup
		return
	}
	for _, file := range r.Files {
		if file.Status == FileChanged || file.Status == FileNew || file.Status == FileMissing || file.Status == FileVaultOnly {
			r.Overall = OverallNeedsEncryption
			return
		}
	}
	r.Overall = OverallSafe
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app`

Expected: PASS for the new missing config/vault/key tests.

- [ ] **Step 5: Commit**

```bash
git add internal/app/status.go internal/app/status_test.go
git commit -m "feat: add repository status model"
```

---

### Task 3: Complete File Classification Tests

**Files:**
- Modify: `internal/app/status_test.go`
- Modify: `internal/app/status.go`

**Interfaces:**
- Consumes:
  - `func StatusRepoWithServices(repoPath string, services Services) (*StatusReport, error)`
  - `type StatusFile struct { Path string; Status FileStatus }`
- Produces: complete, deterministic classification for `unchanged`, `changed`, `new`, `missing`, and `vault-only`

- [ ] **Step 1: Add tests for all file states**

Append these tests to `internal/app/status_test.go`:

```go
func TestStatusRepoReportsUnchangedFiles(t *testing.T) {
	repoPath, store := setupRepoAndHome(t)
	services := Services{KeyStore: store}

	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}

	got, err := StatusRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallSafe {
		t.Fatalf("expected safe, got %q with errors %#v", got.Overall, got.Errors)
	}
	env, ok := findStatusFile(got.Files, ".env")
	if !ok {
		t.Fatalf("expected .env in files: %#v", got.Files)
	}
	if env.Status != FileUnchanged {
		t.Fatalf("expected unchanged, got %q", env.Status)
	}
}

func TestStatusRepoReportsChangedAndNewFiles(t *testing.T) {
	repoPath, store := setupRepoAndHome(t)
	services := Services{KeyStore: store}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	writeStatusFile(t, filepath.Join(repoPath, ".env"), "API_KEY=changed\n", 0o600)
	writeStatusFile(t, filepath.Join(repoPath, "new.secret.yaml"), "password: two\n", 0o600)

	got, err := StatusRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallNeedsEncryption {
		t.Fatalf("expected needs encryption, got %q", got.Overall)
	}
	env, ok := findStatusFile(got.Files, ".env")
	if !ok || env.Status != FileChanged {
		t.Fatalf("expected changed .env, got %#v found=%v", env, ok)
	}
	newFile, ok := findStatusFile(got.Files, "new.secret.yaml")
	if !ok || newFile.Status != FileNew {
		t.Fatalf("expected new file, got %#v found=%v", newFile, ok)
	}
}

func TestStatusRepoReportsMissingExplicitFile(t *testing.T) {
	repoPath, store := setupRepoAndHome(t)
	services := Services{KeyStore: store}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}

	got, err := StatusRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallNeedsEncryption {
		t.Fatalf("expected needs encryption, got %q", got.Overall)
	}
	env, ok := findStatusFile(got.Files, ".env")
	if !ok || env.Status != FileMissing {
		t.Fatalf("expected missing .env, got %#v found=%v", env, ok)
	}
}

func TestStatusRepoReportsVaultOnlyFile(t *testing.T) {
	repoPath, store := setupRepoAndHome(t)
	services := Services{KeyStore: store}
	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, services)

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	vaultOnly, ok := findStatusFile(got.Files, "nested/app.secret.yaml")
	if !ok || vaultOnly.Status != FileVaultOnly {
		t.Fatalf("expected vault-only file, got %#v found=%v", vaultOnly, ok)
	}
}
```

- [ ] **Step 2: Run tests to verify behavior**

Run: `go test ./internal/app`

Expected: PASS if Task 2 classification already handles every state. If it fails, failures should point to one specific status classification.

- [ ] **Step 3: Fix classification if needed**

If needed, adjust only `classifyStatusFiles` and `(*StatusReport).finish` in `internal/app/status.go` so:

```go
FileChanged, FileNew, FileMissing, and FileVaultOnly set OverallNeedsEncryption when no setup errors exist.
FileUnchanged only contributes to OverallSafe.
Files are sorted by Path before returning.
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/status.go internal/app/status_test.go
git commit -m "test: cover status file states"
```

---

### Task 4: Add Config Safety Validation to Status

**Files:**
- Modify: `internal/app/status.go`
- Modify: `internal/app/status_test.go`

**Interfaces:**
- Consumes:
  - `(*config.Config).ArchiverName() string`
  - `(*config.Config).CipherName() string`
  - `filepath.IsLocal(path string) bool`
- Produces:
  - `func validateStatusConfig(cfg *config.Config) (warnings []string, errors []string)`

- [ ] **Step 1: Add failing config validation tests**

Append these tests to `internal/app/status_test.go`:

```go
func TestStatusRepoReportsInvalidPattern(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "patterns:\n  - \"[\"\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if got.Overall != OverallBrokenSetup {
		t.Fatalf("expected broken setup, got %q", got.Overall)
	}
	if !hasStatusMessage(got.Errors, "invalid file pattern") {
		t.Fatalf("expected invalid pattern error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsUnsupportedArchiverAndCypher(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\narchiver: tar\ncypher: age\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !hasStatusMessage(got.Errors, "unsupported archiver") {
		t.Fatalf("expected unsupported archiver error, got %#v", got.Errors)
	}
	if !hasStatusMessage(got.Errors, "unsupported cypher") {
		t.Fatalf("expected unsupported cypher error, got %#v", got.Errors)
	}
}

func TestStatusRepoReportsUnsafeExplicitPaths(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - ../outside.env\n  - /tmp/absolute.env\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !hasStatusMessage(got.Errors, "unsafe explicit file path") {
		t.Fatalf("expected unsafe path error, got %#v", got.Errors)
	}
}

func TestStatusRepoWarnsWhenPatternMatchesNothing(t *testing.T) {
	repoPath, store := statusRepo(t)
	writeStatusFile(t, filepath.Join(repoPath, ".urv.yaml"), "patterns:\n  - \"*.secret.*\"\n", 0o644)

	got, err := StatusRepoWithServices(repoPath, Services{KeyStore: store})

	if err != nil {
		t.Fatalf("expected inspection to succeed, got %v", err)
	}
	if !hasStatusMessage(got.Warnings, "pattern matched no files") {
		t.Fatalf("expected no-match warning, got %#v", got.Warnings)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app`

Expected: FAIL because unsupported archiver/cypher, unsafe paths, and pattern no-match warnings are not all reported.

- [ ] **Step 3: Implement config validation**

In `internal/app/status.go`, add:

```go
func validateStatusConfig(cfg *config.Config) (warnings []string, errors []string) {
	for _, pattern := range cfg.Patterns {
		if _, err := filepath.Match(pattern, ""); err != nil {
			errors = append(errors, fmt.Sprintf("invalid file pattern %q: %v", pattern, err))
		}
	}

	if cfg.ArchiverName() != "zip" {
		errors = append(errors, fmt.Sprintf("unsupported archiver %q", cfg.ArchiverName()))
	}
	if cfg.CipherName() != "aes-gcm" {
		errors = append(errors, fmt.Sprintf("unsupported cypher %q", cfg.CipherName()))
	}

	for _, path := range cfg.SecretFiles {
		cleanPath := filepath.ToSlash(filepath.Clean(path))
		if !filepath.IsLocal(cleanPath) {
			errors = append(errors, fmt.Sprintf("unsafe explicit file path %q", path))
		}
		switch cleanPath {
		case ".urv.yaml", ".urv.vault.yaml", ".git":
			errors = append(errors, fmt.Sprintf("reserved path configured as secret file %q", path))
		}
		if cleanPath == files.LockFileName {
			errors = append(errors, fmt.Sprintf("reserved path configured as secret file %q", path))
		}
	}

	return warnings, errors
}
```

Call validation immediately after config loads:

```go
warnings, validationErrors := validateStatusConfig(cfg)
report.Warnings = append(report.Warnings, warnings...)
report.Errors = append(report.Errors, validationErrors...)
```

After `foundFiles` is available, add pattern no-match warnings:

```go
for _, pattern := range cfg.Patterns {
	matched := false
	for _, path := range foundFiles {
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			matched = true
			break
		}
	}
	if !matched {
		report.Warnings = append(report.Warnings, fmt.Sprintf("pattern matched no files: %s", pattern))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/status.go internal/app/status_test.go
git commit -m "feat: validate config in status"
```

---

### Task 5: Add `urv status` Command

**Files:**
- Create: `cmd/status/status.go`
- Create: `cmd/status/status_test.go`
- Modify: `cmd/root.go`
- Modify: `cmd/root_test.go`

**Interfaces:**
- Consumes:
  - `app.StatusRepo(repoPath string) (*app.StatusReport, error)`
  - `type app.StatusReport`
  - `repo.GetCurrentRepoPath() (string, error)`
- Produces:
  - `func NewCommand() *cobra.Command`
  - `func FormatReport(report *app.StatusReport) string`

- [ ] **Step 1: Write failing command formatter tests**

Create `cmd/status/status_test.go`:

```go
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
		Overall: app.OverallNeedsEncryption,
		ConfigOK: true,
		VaultOK: true,
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
```

- [ ] **Step 2: Update root command test**

Modify `cmd/root_test.go` so the expected root commands are:

```go
want := []string{"init", "encrypt", "decrypt", "keys", "status"}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./cmd/...`

Expected: FAIL because `cmd/status` and root wiring do not exist.

- [ ] **Step 4: Implement status command**

Create `cmd/status/status.go`:

```go
package status

import (
	"fmt"
	"strings"

	"github.com/mustafmst/universal-repo-vault/internal/app"
	"github.com/mustafmst/universal-repo-vault/internal/repo"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository vault safety status",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	repoPath, err := repo.GetCurrentRepoPath()
	if err != nil {
		return err
	}

	report, err := app.StatusRepo(repoPath)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(cmd.OutOrStdout(), FormatReport(report))
	return err
}

func FormatReport(report *app.StatusReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Overall: %s\n", report.Overall)

	if report.ConfigOK {
		b.WriteString("Config: ok\n")
	} else {
		b.WriteString("Config: not ok\n")
	}

	if report.VaultOK {
		b.WriteString("Vault: ok\n")
	} else if report.VaultExists {
		b.WriteString("Vault: not ok\n")
	} else {
		b.WriteString("Vault: missing\n")
	}

	if report.KeyMapped {
		fmt.Fprintf(&b, "Key: %s", report.KeyName)
		if !report.KeyFileExists {
			b.WriteString(" (missing file)")
		} else if !report.KeyLengthValid {
			b.WriteString(" (invalid length)")
		}
		b.WriteString("\n")
	} else {
		b.WriteString("Key: not mapped\n")
	}

	if len(report.Files) > 0 {
		b.WriteString("Files:\n")
		for _, file := range report.Files {
			fmt.Fprintf(&b, "  %s %s\n", file.Path, file.Status)
		}
	}

	if len(report.Warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "  %s\n", warning)
		}
	}

	if len(report.Errors) > 0 {
		b.WriteString("Errors:\n")
		for _, err := range report.Errors {
			fmt.Fprintf(&b, "  %s\n", err)
		}
	}

	return b.String()
}
```

Modify `cmd/root.go`:

```go
import (
	"github.com/mustafmst/universal-repo-vault/cmd/decrypt"
	"github.com/mustafmst/universal-repo-vault/cmd/encrypt"
	"github.com/mustafmst/universal-repo-vault/cmd/initcmd"
	"github.com/mustafmst/universal-repo-vault/cmd/keys"
	"github.com/mustafmst/universal-repo-vault/cmd/status"
	"github.com/spf13/cobra"
)
```

Add the command:

```go
cmd.AddCommand(status.NewCommand())
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/root_test.go cmd/status/status.go cmd/status/status_test.go
git commit -m "feat: add status command"
```

---

### Task 6: Document and Verify `urv status`

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: `urv status` command behavior from Task 5
- Produces: README sections documenting status usage and output meaning

- [ ] **Step 1: Update README command list**

In `README.md`, add `urv status` to the current implementation list:

```markdown
- Inspect repository vault safety status without changing files.
```

Add it to the Commands section:

````markdown
Show whether the repository is safe, needs encryption, or has broken setup:

```sh
urv status
```
````

- [ ] **Step 2: Add status workflow guidance**

In `README.md`, after the encrypt workflow, add:

````markdown
Check repository safety before committing:

```sh
urv status
```

`safe` means the configured files match the vault metadata. `needs encryption` means at least one configured file is new, changed, missing, or only present in the vault metadata. `broken setup` means URV could not validate required setup such as config, vault, or local key mapping.
````

- [ ] **Step 3: Run formatting and tests**

Run: `gofmt -w internal/app/status.go internal/app/status_test.go internal/keystore/keystore.go internal/keystore/keystore_test.go cmd/root.go cmd/root_test.go cmd/status/status.go cmd/status/status_test.go`

Expected: no output.

Run: `go test ./...`

Expected: PASS.

Run: `go vet ./...`

Expected: PASS.

- [ ] **Step 4: Manual smoke test**

Run:

```bash
go run ./main.go status
```

Expected in this repository after the feature is implemented: command exits 0 and prints an `Overall:` line. The exact state depends on whether this repo has local URV config, vault, and key mapping on the developer machine.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: document status command"
```

---

## Final Verification

- [ ] Run: `gofmt -w internal/app/status.go internal/app/status_test.go internal/keystore/keystore.go internal/keystore/keystore_test.go cmd/root.go cmd/root_test.go cmd/status/status.go cmd/status/status_test.go`
- [ ] Run: `go test ./...`
- [ ] Run: `go vet ./...`
- [ ] Run: `go run ./main.go status`
- [ ] Confirm `git status --short` shows only intentional changes or is clean after the final commit.

## Self-Review Notes

- Spec coverage: this plan implements the first milestone, `urv status`, including config state, vault state, key mapping state, key file state, configured file discovery, per-file hash comparison, warnings, errors, command output, tests, and README documentation.
- Deferred by spec: `urv check`, safe decrypt flags, gitignore/plaintext assistant, key health commands, key rotation, and vault metadata inspection remain separate later roadmap items.
- Type consistency: `StatusReport`, `StatusFile`, `OverallStatus`, `FileStatus`, and `HealthForRepo` signatures are defined before downstream command tasks consume them.
