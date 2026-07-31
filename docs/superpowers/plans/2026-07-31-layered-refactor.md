# Layered Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor URV into layered compatibility adapters for config, vault metadata, archive, crypto, and keystore while preserving current CLI behavior and all existing storage formats.

**Architecture:** Cobra commands stay thin and call `internal/app`. `internal/app` composes focused packages: `config` for `.urv.yaml`, `vault` for `.urv.vault.yaml`, `archive` for ZIP bytes, `crypto` for AES-GCM, `keystore` for local keys and mappings, `files` for discovery and hashes, and `repo` for repository detection.

**Tech Stack:** Go 1.26.1, Cobra, Viper, `go.yaml.in/yaml/v3`, standard-library `archive/zip`, standard-library `crypto/aes` and `crypto/cipher`.

## Global Constraints

- Keep the current CLI behavior and command names.
- Keep existing `.urv.yaml` files valid.
- Keep existing `.urv.vault.yaml` files valid.
- Keep existing local keys under `~/.config/urv/keys/<key-name>` valid.
- Keep existing repo mapping file `~/.config/urv/mapping.yaml` valid.
- Keep ZIP payload entries as repository-relative file paths.
- No breaking vault format migration.
- No new supported archiver beyond ZIP in the first refactor.
- No new supported cipher beyond AES-GCM in the first refactor.
- No new CLI command structure.
- No removal of old `.urv.lock` read compatibility during this refactor.
- The current config field name `cypher` remains accepted.
- New key files are created with `0600` permissions.
- Run `gofmt -w` on changed Go files before each commit.
- Run `go test ./...` before each commit.

---

## File Structure

- Create `internal/archive/archive.go`: defines `Archiver`, `ZipArchiver`, `NewZipArchiver`, and ZIP pack/unpack methods.
- Create `internal/archive/archive_test.go`: ZIP round-trip, nested paths, overwrite, and unsafe path tests.
- Create `internal/crypto/crypto.go`: defines `Cipher`, `AesGcmCipher`, `NewAesGcm`, `NewAesGcmFromHexKey`, and `NewCipher`.
- Create `internal/crypto/crypto_test.go`: moved AES-GCM tests plus factory tests.
- Create `internal/keystore/keystore.go`: defines `FileStore`, `KeyMapping`, key generation, mapping load/save, key save/use/list/get.
- Create `internal/keystore/keystore_test.go`: key generation, file permissions, mapping compatibility, missing key behavior.
- Create `internal/app/app_test.go`: workflow tests for encrypt/decrypt, unchanged-hash behavior, and missing inputs.
- Modify `internal/app/encrypt.go`: compose `config`, `files`, `vault`, `archive`, `crypto`, and `keystore`.
- Modify `internal/app/decrypt.go`: compose `vault`, `crypto`, `archive`, and `keystore`.
- Modify `internal/config/config.go`: converge loading behavior and retain `.urv.yaml` compatibility.
- Modify `internal/config/config_test.go`: cover `Load`, `ConfigProvider`, defaults, and existing fields.
- Modify `internal/files/files.go`: deterministic de-duplicated discovery, invalid glob errors, close file handles in `SaveDataToFile`.
- Modify `internal/files/hash.go`: close files in `GetFileHash`; rename internal misspelling if it does not create unnecessary churn.
- Modify `internal/files/hash_test.go`: extend hash and lockfile coverage.
- Modify `internal/vault/vault.go`: keep vault YAML format and validation only; remove ZIP responsibilities after archive extraction.
- Modify `internal/vault/vault_test.go`: keep vault YAML/hex/version/algo tests.
- Modify `internal/vault/crypto.go`, `internal/vault/crypto_test.go`, `internal/vault/key.go`: remove after replacement or convert temporarily to wrappers only if needed for incremental compilation.
- Modify `cmd/keys/gen/gen.go`, `cmd/keys/add/add.go`, `cmd/keys/list/list.go`: switch imports from `internal/vault` key helpers to `internal/keystore`.
- Modify `README.md`: update project structure after refactor.
- Modify `doc/review.md`: mark fixed cleanup findings that this implementation resolves.
- Modify `doc/designs/solid-refactor.md`: replace stale rough plan with a pointer to the accepted spec and implemented package layout.

---

### Task 1: Characterize Current Workflows And File Discovery

**Files:**
- Create: `internal/app/app_test.go`
- Modify: `internal/files/hash_test.go`
- Modify: `internal/files/files_test.go`

**Interfaces:**
- Consumes: existing `app.EncryptRepo(repoPath string) (*EncryptResult, error)`, `app.DecryptRepo(repoPath string) error`, `files.ListAllConfiguredFiles(basePath string, fileList []string, patternlist []string) ([]string, error)`.
- Produces: tests that lock expected workflow behavior before refactoring.

- [ ] **Step 1: Add app workflow test helpers**

Add these helpers to `internal/app/app_test.go`:

```go
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

const appTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func writeFile(t *testing.T, path string, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func setupRepoAndHome(t *testing.T) (repoPath string, homePath string) {
	t.Helper()
	repoPath = t.TempDir()
	homePath = t.TempDir()
	t.Setenv("HOME", homePath)
	if err := os.Mkdir(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\npatterns:\n  - \"*.secret.*\"\n", 0o644)
	writeFile(t, filepath.Join(repoPath, ".env"), "API_KEY=one\n", 0o600)
	writeFile(t, filepath.Join(repoPath, "nested", "app.secret.yaml"), "password: one\n", 0o600)
	if err := vault.SaveKey(appTestKey, repoPath, "repo-key"); err != nil {
		t.Fatal(err)
	}
	return repoPath, homePath
}
```

- [ ] **Step 2: Add encrypt/decrypt round-trip characterization**

Add this test to `internal/app/app_test.go`:

```go
func TestEncryptDecryptRepoRoundTrip(t *testing.T) {
	repoPath, _ := setupRepoAndHome(t)

	result, err := EncryptRepo(repoPath)
	if err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	if !result.Encrypted {
		t.Fatal("expected first encrypt to write vault")
	}

	if _, err := os.Stat(filepath.Join(repoPath, vault.VaultFileName)); err != nil {
		t.Fatalf("expected vault file to exist: %v", err)
	}

	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repoPath, "nested", "app.secret.yaml")); err != nil {
		t.Fatal(err)
	}

	if err := DecryptRepo(repoPath); err != nil {
		t.Fatalf("expected decrypt to succeed, got %v", err)
	}

	envData, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(envData) != "API_KEY=one\n" {
		t.Fatalf("unexpected env contents: %q", string(envData))
	}

	secretData, err := os.ReadFile(filepath.Join(repoPath, "nested", "app.secret.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(secretData) != "password: one\n" {
		t.Fatalf("unexpected secret contents: %q", string(secretData))
	}
}
```

- [ ] **Step 3: Add unchanged-hash characterization**

Add this test to `internal/app/app_test.go`:

```go
func TestEncryptRepoReturnsUnchangedWhenHashesMatchVault(t *testing.T) {
	repoPath, _ := setupRepoAndHome(t)

	first, err := EncryptRepo(repoPath)
	if err != nil {
		t.Fatalf("expected first encrypt to succeed, got %v", err)
	}
	if !first.Encrypted {
		t.Fatal("expected first encrypt to write vault")
	}

	second, err := EncryptRepo(repoPath)
	if err != nil {
		t.Fatalf("expected second encrypt to succeed, got %v", err)
	}
	if second.Encrypted {
		t.Fatal("expected unchanged hashes to skip encryption")
	}
}
```

- [ ] **Step 4: Add desired file discovery tests**

Create `internal/files/files_test.go` with tests for the desired post-refactor behavior:

```go
package files

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDiscoveryFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListAllConfiguredFilesReturnsDeterministicDeduplicatedPaths(t *testing.T) {
	dir := t.TempDir()
	writeDiscoveryFile(t, filepath.Join(dir, ".env"))
	writeDiscoveryFile(t, filepath.Join(dir, "a.secret.yaml"))
	writeDiscoveryFile(t, filepath.Join(dir, "nested", "b.secret.yaml"))
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDiscoveryFile(t, filepath.Join(dir, ".git", "ignored.secret.yaml"))

	got, err := ListAllConfiguredFiles(dir, []string{".env", "a.secret.yaml"}, []string{"*.secret.*"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{".env", "a.secret.yaml", "nested/b.secret.yaml"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestListAllConfiguredFilesRejectsInvalidPattern(t *testing.T) {
	dir := t.TempDir()
	writeDiscoveryFile(t, filepath.Join(dir, ".env"))

	_, err := ListAllConfiguredFiles(dir, nil, []string{"["})
	if err == nil {
		t.Fatal("expected invalid pattern error, got nil")
	}
}
```

- [ ] **Step 5: Run tests to verify characterization state**

Run: `go test ./...`

Expected: app workflow tests pass; the invalid-pattern and de-duplication file discovery tests fail against current implementation. The failing tests describe the intended cleanup behavior.

- [ ] **Step 6: Implement minimal file discovery and file close fixes**

Modify `internal/files/files.go`:

```go
func ListAllConfiguredFiles(basePath string, fileList []string, patternlist []string) ([]string, error) {
	explicit := map[string]struct{}{}
	for _, f := range fileList {
		explicit[filepath.Clean(f)] = struct{}{}
	}

	for _, p := range patternlist {
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("invalid file pattern %q: %w", p, err)
		}
	}

	seen := map[string]struct{}{}
	result := []string{}
	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(filepath.Clean(relPath))

		matched := false
		if _, ok := explicit[relPath]; ok {
			matched = true
		}
		for _, p := range patternlist {
			ok, err := filepath.Match(p, d.Name())
			if err != nil {
				return fmt.Errorf("invalid file pattern %q: %w", p, err)
			}
			if ok {
				matched = true
			}
		}
		if matched {
			seen[relPath] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing files: %w", err)
	}

	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}
```

Add `sort` to imports. In `SaveDataToFile`, close the file:

```go
f, err := os.Create(fullPath)
if err != nil {
	return 0, fmt.Errorf("creating file %s: %w", fullPath, err)
}
defer f.Close()

n, err := f.Write(data)
if err != nil {
	return n, err
}
return n, nil
```

Modify `internal/files/hash.go` to close opened files:

```go
f, err := os.Open(absPath)
if err != nil {
	return nil, fmt.Errorf("opening file for hashing: %w", err)
}
defer f.Close()
```

- [ ] **Step 7: Run tests to verify Task 1 passes**

Run: `gofmt -w internal/app/app_test.go internal/files/files.go internal/files/files_test.go internal/files/hash.go`

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 8: Commit Task 1**

```bash
git add internal/app/app_test.go internal/files/files.go internal/files/files_test.go internal/files/hash.go
git commit -m "test: characterize vault workflows"
```

---

### Task 2: Extract ZIP Archive Layer

**Files:**
- Create: `internal/archive/archive.go`
- Create: `internal/archive/archive_test.go`
- Modify: `internal/vault/vault.go`
- Modify: `internal/vault/vault_test.go`
- Modify: `internal/app/encrypt.go`
- Modify: `internal/app/decrypt.go`

**Interfaces:**
- Consumes: `files.ListAllConfiguredFiles`, current ZIP behavior in `internal/vault`.
- Produces:
  - `type Archiver interface { Pack(basePath string, relPaths []string) ([]byte, error); Unpack(basePath string, data []byte, overwrite bool) error }`
  - `func NewZipArchiver() *ZipArchiver`
  - `func (za *ZipArchiver) Pack(basePath string, relPaths []string) ([]byte, error)`
  - `func (za *ZipArchiver) Unpack(basePath string, data []byte, overwrite bool) error`

- [ ] **Step 1: Write archive tests**

Create `internal/archive/archive_test.go`:

```go
package archive

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeArchiveFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestZipArchiverPackUnpackRoundTrip(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeArchiveFile(t, filepath.Join(src, ".env"), "API_KEY=one\n")
	writeArchiveFile(t, filepath.Join(src, "nested", "app.secret.yaml"), "password: one\n")

	archiver := NewZipArchiver()
	data, err := archiver.Pack(src, []string{".env", "nested/app.secret.yaml"})
	if err != nil {
		t.Fatalf("expected pack to succeed, got %v", err)
	}

	if err := archiver.Unpack(dst, data, true); err != nil {
		t.Fatalf("expected unpack to succeed, got %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "nested", "app.secret.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "password: one\n" {
		t.Fatalf("unexpected unpacked data: %q", string(got))
	}
}

func TestZipArchiverRejectsUnsafePath(t *testing.T) {
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

	err = NewZipArchiver().Unpack(t.TempDir(), buf.Bytes(), true)
	if err == nil {
		t.Fatal("expected unsafe path error, got nil")
	}
	if !strings.Contains(err.Error(), "unsafe zip path") {
		t.Fatalf("expected unsafe path error, got %v", err)
	}
}

func TestZipArchiverHonorsOverwriteFlag(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeArchiveFile(t, filepath.Join(src, ".env"), "new\n")
	writeArchiveFile(t, filepath.Join(dst, ".env"), "old\n")

	data, err := NewZipArchiver().Pack(src, []string{".env"})
	if err != nil {
		t.Fatal(err)
	}

	err = NewZipArchiver().Unpack(dst, data, false)
	if err == nil {
		t.Fatal("expected existing file error, got nil")
	}

	if err := NewZipArchiver().Unpack(dst, data, true); err != nil {
		t.Fatalf("expected overwrite unpack to succeed, got %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("expected overwritten data, got %q", string(got))
	}
}
```

- [ ] **Step 2: Run archive tests to verify missing package failure**

Run: `go test ./internal/archive`

Expected: FAIL because `NewZipArchiver` is undefined.

- [ ] **Step 3: Implement archive package**

Create `internal/archive/archive.go` by moving ZIP logic from `internal/vault/vault.go` and keeping repository-relative entry names:

```go
package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Archiver interface {
	Pack(basePath string, relPaths []string) ([]byte, error)
	Unpack(basePath string, data []byte, overwrite bool) error
}

type ZipArchiver struct{}

func NewZipArchiver() *ZipArchiver {
	return &ZipArchiver{}
}

func (za *ZipArchiver) Pack(basePath string, relPaths []string) ([]byte, error) {
	var buff bytes.Buffer
	w := zip.NewWriter(&buff)

	errs := []error{}
	for _, relPath := range relPaths {
		if err := writeFileToZip(w, basePath, relPath); err != nil {
			errs = append(errs, err)
		}
	}

	if err := w.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return buff.Bytes(), nil
}

func writeFileToZip(zw *zip.Writer, basePath string, relPath string) error {
	cleanPath := filepath.ToSlash(filepath.Clean(relPath))
	if !filepath.IsLocal(cleanPath) {
		return fmt.Errorf("unsafe archive path: %s", relPath)
	}
	f, err := os.Open(filepath.Join(basePath, filepath.FromSlash(cleanPath)))
	if err != nil {
		return err
	}
	defer f.Close()

	entry, err := zw.Create(cleanPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(entry, f); err != nil {
		return err
	}
	return nil
}

func (za *ZipArchiver) Unpack(basePath string, data []byte, overwrite bool) error {
	dataReader := bytes.NewReader(data)
	zr, err := zip.NewReader(dataReader, int64(len(data)))
	if err != nil {
		return fmt.Errorf("creating zip reader from bytes data: %w", err)
	}

	errs := []error{}
	for _, zf := range zr.File {
		if err := extractFileFromZip(zf, basePath, overwrite); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func extractFileFromZip(zf *zip.File, basePath string, overwrite bool) error {
	cleanName := filepath.Clean(zf.Name)
	if !filepath.IsLocal(cleanName) {
		return fmt.Errorf("unsafe zip path: %s", zf.Name)
	}
	fullPath := filepath.Join(basePath, cleanName)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}

	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	f, err := os.OpenFile(fullPath, flags, zf.Mode())
	if err != nil {
		return fmt.Errorf("opening file %s for unpack: %w", fullPath, err)
	}
	defer f.Close()

	zfr, err := zf.Open()
	if err != nil {
		return fmt.Errorf("opening zip file read: %w", err)
	}
	defer zfr.Close()

	if _, err := io.Copy(f, zfr); err != nil {
		return fmt.Errorf("copying zipped file data to a file: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Update app workflows to use archive package**

In `internal/app/encrypt.go`, add import:

```go
"github.com/mustafmst/universal-repo-vault/internal/archive"
```

Replace:

```go
data, err := vault.CreateZipVaultData(repoPath, foundFiles)
```

with:

```go
data, err := archive.NewZipArchiver().Pack(repoPath, foundFiles)
```

In `internal/app/decrypt.go`, add import:

```go
"github.com/mustafmst/universal-repo-vault/internal/archive"
```

Replace:

```go
return vault.UnpackZipVaultData(repoPath, decryptedArch)
```

with:

```go
return archive.NewZipArchiver().Unpack(repoPath, decryptedArch, true)
```

- [ ] **Step 5: Remove ZIP code from vault**

In `internal/vault/vault.go`, remove imports that are only needed for ZIP handling: `archive/zip`, `bytes`, `io`, and `log`.

Delete these functions from `internal/vault/vault.go`:

```go
func CreateZipVaultData(basePath string, filePaths []string) ([]byte, error)
func writeFileToZip(zw *zip.Writer, basePath string, filePath string) error
func UnpackZipVaultData(basePath string, data []byte) error
func extractFileFromZip(zf *zip.File, basePath string, forceReplace bool) error
```

- [ ] **Step 6: Run tests**

Run: `gofmt -w internal/archive/archive.go internal/archive/archive_test.go internal/app/encrypt.go internal/app/decrypt.go internal/vault/vault.go`

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 7: Commit Task 2**

```bash
git add internal/archive internal/app/encrypt.go internal/app/decrypt.go internal/vault/vault.go
git commit -m "refactor: extract zip archive layer"
```

---

### Task 3: Extract AES-GCM Crypto Layer

**Files:**
- Create: `internal/crypto/crypto.go`
- Create: `internal/crypto/crypto_test.go`
- Modify: `internal/app/encrypt.go`
- Modify: `internal/app/decrypt.go`
- Modify: `internal/vault/crypto.go`
- Modify: `internal/vault/crypto_test.go`

**Interfaces:**
- Consumes: current AES-GCM behavior from `internal/vault/crypto.go`.
- Produces:
  - `type Cipher interface { Encrypt(data []byte) ([]byte, error); Decrypt(data []byte) ([]byte, error); Name() string }`
  - `func NewAesGcmFromHexKey(hexKey string) (*AesGcmCipher, error)`
  - `func NewAesGcm(key []byte) (*AesGcmCipher, error)`
  - `func NewCipher(name string, hexKey string) (Cipher, error)`
  - `func AesGcmEncrypt(key string, data []byte) ([]byte, error)`
  - `func AesGcmDecrypt(key string, data []byte) ([]byte, error)`

- [ ] **Step 1: Move AES-GCM tests**

Copy the contents of `internal/vault/crypto_test.go` into `internal/crypto/crypto_test.go`, change the package line to:

```go
package crypto
```

Add a factory test:

```go
func TestNewCipherRejectsUnsupportedCipher(t *testing.T) {
	_, err := NewCipher("other", testKey)
	if err == nil {
		t.Fatal("expected unsupported cipher error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported cipher") {
		t.Fatalf("expected unsupported cipher error, got %v", err)
	}
}
```

- [ ] **Step 2: Run crypto tests to verify missing package failure**

Run: `go test ./internal/crypto`

Expected: FAIL because the crypto package implementation is missing.

- [ ] **Step 3: Implement crypto package**

Create `internal/crypto/crypto.go`:

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

const AesGcmName = "aes-gcm"

type Cipher interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
	Name() string
}

type AesGcmCipher struct {
	key []byte
}

func (agc *AesGcmCipher) Name() string {
	return AesGcmName
}

func (agc *AesGcmCipher) Encrypt(data []byte) ([]byte, error) {
	b, err := aes.NewCipher(agc.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func (agc *AesGcmCipher) Decrypt(data []byte) ([]byte, error) {
	b, err := aes.NewCipher(agc.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("cipher too small")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func NewAesGcmFromHexKey(hexKey string) (*AesGcmCipher, error) {
	byteKey, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decoding AES GCM key from hex to bytes: %w", err)
	}
	return NewAesGcm(byteKey)
}

func NewAesGcm(key []byte) (*AesGcmCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("expected key length is 32 bytes, got: %d", len(key))
	}
	return &AesGcmCipher{key: key}, nil
}

func NewCipher(name string, hexKey string) (Cipher, error) {
	switch name {
	case AesGcmName:
		return NewAesGcmFromHexKey(hexKey)
	default:
		return nil, fmt.Errorf("unsupported cipher: %s", name)
	}
}

func AesGcmEncrypt(key string, data []byte) ([]byte, error) {
	c, err := NewAesGcmFromHexKey(key)
	if err != nil {
		return nil, err
	}
	return c.Encrypt(data)
}

func AesGcmDecrypt(key string, data []byte) ([]byte, error) {
	c, err := NewAesGcmFromHexKey(key)
	if err != nil {
		return nil, err
	}
	return c.Decrypt(data)
}
```

- [ ] **Step 4: Update app workflows to use crypto package**

In `internal/app/encrypt.go`, import:

```go
urvcrypto "github.com/mustafmst/universal-repo-vault/internal/crypto"
```

Replace:

```go
encryptedData, err := vault.AesGcmEncrypt(key, data)
```

with:

```go
cipher, err := urvcrypto.NewCipher(vault.VaultAlgo, key)
if err != nil {
	return nil, fmt.Errorf("creating cipher: %w", err)
}
encryptedData, err := cipher.Encrypt(data)
```

In `internal/app/decrypt.go`, import:

```go
urvcrypto "github.com/mustafmst/universal-repo-vault/internal/crypto"
```

Replace:

```go
decryptedArch, err := vault.AesGcmDecrypt(key, vaultData)
```

with:

```go
cipher, err := urvcrypto.NewCipher(v.Algo, key)
if err != nil {
	return err
}
decryptedArch, err := cipher.Decrypt(vaultData)
```

- [ ] **Step 5: Remove or wrap old vault crypto file**

Preferred final state: delete `internal/vault/crypto.go` and `internal/vault/crypto_test.go`.

If deletion creates too much churn in dependent code during this task, temporarily replace `internal/vault/crypto.go` with wrappers:

```go
package vault

import urvcrypto "github.com/mustafmst/universal-repo-vault/internal/crypto"

func AesGcmEncrypt(key string, data []byte) ([]byte, error) {
	return urvcrypto.AesGcmEncrypt(key, data)
}

func AesGcmDecrypt(key string, data []byte) ([]byte, error) {
	return urvcrypto.AesGcmDecrypt(key, data)
}
```

Before committing this task, prefer deleting the wrappers if no code imports them.

- [ ] **Step 6: Run tests**

Run: `gofmt -w internal/crypto/crypto.go internal/crypto/crypto_test.go internal/app/encrypt.go internal/app/decrypt.go`

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 7: Commit Task 3**

```bash
git add internal/crypto internal/app/encrypt.go internal/app/decrypt.go internal/vault/crypto.go internal/vault/crypto_test.go
git commit -m "refactor: extract aes gcm crypto layer"
```

If `internal/vault/crypto.go` and `internal/vault/crypto_test.go` were deleted, `git add` still records the deletions.

---

### Task 4: Extract Local Keystore Layer

**Files:**
- Create: `internal/keystore/keystore.go`
- Create: `internal/keystore/keystore_test.go`
- Modify: `internal/app/encrypt.go`
- Modify: `internal/app/decrypt.go`
- Modify: `cmd/keys/gen/gen.go`
- Modify: `cmd/keys/add/add.go`
- Modify: `cmd/keys/list/list.go`
- Modify: `internal/vault/key.go`

**Interfaces:**
- Consumes: current key behavior from `internal/vault/key.go`.
- Produces:
  - `const KeyLength = 32`
  - `type KeyMapping struct { keys map[string]string }`
  - `type FileStore struct { home string }`
  - `func NewFileStore(home string) *FileStore`
  - `func NewDefaultFileStore() *FileStore`
  - `func GenerateKey() (string, error)`
  - `func (fs *FileStore) SaveKey(key string, repoPath string, keyName string) error`
  - `func (fs *FileStore) UseKeyForRepo(keyName string, repoPath string) error`
  - `func (fs *FileStore) KeyForRepo(repoPath string) (string, error)`
  - `func (fs *FileStore) Mapping() (*KeyMapping, error)`

- [ ] **Step 1: Write keystore tests**

Create `internal/keystore/keystore_test.go`:

```go
package keystore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const keystoreTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestGenerateKeyReturns64HexCharacters(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(key) != 64 {
		t.Fatalf("expected 64 hex characters, got %d", len(key))
	}
}

func TestFileStoreSaveKeyCreates0600KeyAndMapping(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(t.TempDir(), "repo")
	store := NewFileStore(home)

	if err := store.SaveKey(keystoreTestKey, repoPath, "repo-key"); err != nil {
		t.Fatalf("expected save key to succeed, got %v", err)
	}

	keyPath := filepath.Join(home, ".config", "urv", "keys", "repo-key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected key mode 0600, got %o", info.Mode().Perm())
	}

	got, err := store.KeyForRepo(repoPath)
	if err != nil {
		t.Fatalf("expected mapped key, got %v", err)
	}
	if got != keystoreTestKey {
		t.Fatalf("expected key %q, got %q", keystoreTestKey, got)
	}
}

func TestFileStoreReadsExistingMappingFormat(t *testing.T) {
	home := t.TempDir()
	repoPath := "/tmp/example-repo"
	writePath := filepath.Join(home, ".config", "urv")
	if err := os.MkdirAll(filepath.Join(writePath, "keys"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(writePath, "keys", "repo-key"), []byte(keystoreTestKey), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(writePath, "mapping.yaml"), []byte(repoPath+": repo-key\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewFileStore(home).KeyForRepo(repoPath)
	if err != nil {
		t.Fatalf("expected mapped key, got %v", err)
	}
	if got != keystoreTestKey {
		t.Fatalf("expected key %q, got %q", keystoreTestKey, got)
	}
}

func TestFileStoreMissingKeyForRepoReturnsContext(t *testing.T) {
	_, err := NewFileStore(t.TempDir()).KeyForRepo("/tmp/missing")
	if err == nil {
		t.Fatal("expected missing mapping error, got nil")
	}
	if !strings.Contains(err.Error(), "key for repo not found") {
		t.Fatalf("expected missing mapping error, got %v", err)
	}
}
```

- [ ] **Step 2: Run keystore tests to verify missing package failure**

Run: `go test ./internal/keystore`

Expected: FAIL because the keystore package implementation is missing.

- [ ] **Step 3: Implement keystore package**

Create `internal/keystore/keystore.go` by moving key logic from `internal/vault/key.go`. Use this shape:

```go
package keystore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mustafmst/universal-repo-vault/internal/files"
	"go.yaml.in/yaml/v3"
)

const KeyVariable = "URV_KEY_NAME"
const KeyLength = 32

type KeyMapping struct {
	keys map[string]string
}

type FileStore struct {
	home string
}

func NewDefaultFileStore() *FileStore {
	return NewFileStore(os.Getenv("HOME"))
}

func NewFileStore(home string) *FileStore {
	return &FileStore{home: home}
}

func (fs *FileStore) keysDir() string {
	return filepath.Join(fs.home, ".config", "urv", "keys")
}

func (fs *FileStore) mappingPath() string {
	return filepath.Join(fs.home, ".config", "urv", "mapping.yaml")
}

func (fs *FileStore) keyPath(keyName string) string {
	return filepath.Join(fs.keysDir(), keyName)
}
```

Move and adapt `KeyMapping.List`, `KeyMapping.String`, `KeyMapping.Get`, `KeyMapping.Add`, and `KeyMapping.Save`. `Save` must call `fs.Mapping().Save()` through a receiver that knows the mapping path, or `FileStore.SaveMapping(mapping *KeyMapping) error`. Keep the YAML data as a map of repo path to key name.

Implement `GenerateKey`:

```go
func GenerateKey() (string, error) {
	keyBytes := make([]byte, KeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(keyBytes), nil
}
```

Implement key-file writes with mode `0600`:

```go
f, err := os.OpenFile(keyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
if err != nil {
	return err
}
defer f.Close()
```

Implement `KeyForRepo` using `os.ReadFile` and validate the read length:

```go
raw, err := os.ReadFile(keyFile)
if err != nil {
	return "", err
}
key := strings.TrimSpace(string(raw))
if len(key) != 2*KeyLength {
	return "", fmt.Errorf("reading key from: %s, expected key len: %d, read: %d", keyFile, 2*KeyLength, len(key))
}
return key, nil
```

- [ ] **Step 4: Update app and key commands to use keystore**

In `internal/app/encrypt.go` and `internal/app/decrypt.go`, import:

```go
"github.com/mustafmst/universal-repo-vault/internal/keystore"
```

Replace:

```go
key, err := vault.GetKeyForRepo(repoPath)
```

with:

```go
key, err := keystore.NewDefaultFileStore().KeyForRepo(repoPath)
```

In key command packages, replace imports and calls:

```go
keystore.GenerateKey()
keystore.NewDefaultFileStore().SaveKey(key, repoPath, keyName)
keystore.NewDefaultFileStore().UseKeyForRepo(keyName, repoPath)
keystore.NewDefaultFileStore().Mapping()
```

- [ ] **Step 5: Remove or wrap old vault key file**

Preferred final state: delete `internal/vault/key.go`.

If dependent command code cannot be moved cleanly in one edit, temporarily replace it with wrappers around `internal/keystore`, then delete wrappers before this task is committed.

- [ ] **Step 6: Run tests**

Run: `gofmt -w internal/keystore/keystore.go internal/keystore/keystore_test.go internal/app/encrypt.go internal/app/decrypt.go cmd/keys/gen/gen.go cmd/keys/add/add.go cmd/keys/list/list.go`

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 7: Commit Task 4**

```bash
git add internal/keystore internal/app/encrypt.go internal/app/decrypt.go cmd/keys/gen/gen.go cmd/keys/add/add.go cmd/keys/list/list.go internal/vault/key.go
git commit -m "refactor: extract local keystore"
```

---

### Task 5: Simplify Vault Metadata Layer

**Files:**
- Modify: `internal/vault/vault.go`
- Modify: `internal/vault/vault_test.go`
- Modify: `internal/app/encrypt.go`
- Modify: `internal/app/decrypt.go`

**Interfaces:**
- Consumes: `archive.Archiver`, `crypto.Cipher`, `keystore.FileStore`.
- Produces: `internal/vault` limited to vault YAML compatibility and metadata validation.

- [ ] **Step 1: Add vault save compatibility tests**

Add this test to `internal/vault/vault_test.go`:

```go
func TestSaveToFileWritesCompatibleVaultYAML(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, VaultFileName)
	v := NewVaultFromData([]byte("secret"), map[string]string{".env": "abc123"})

	if err := v.SaveToFile(vaultPath); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	loaded, err := NewVaultFromFilePath(vaultPath)
	if err != nil {
		t.Fatalf("expected load to succeed, got %v", err)
	}
	if loaded.Version != VaultVersion {
		t.Fatalf("expected version %q, got %q", VaultVersion, loaded.Version)
	}
	if loaded.Algo != VaultAlgo {
		t.Fatalf("expected algo %q, got %q", VaultAlgo, loaded.Algo)
	}
	if loaded.Hashes[".env"] != "abc123" {
		t.Fatalf("unexpected hashes: %#v", loaded.Hashes)
	}
	data, err := loaded.GetByteData()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "secret" {
		t.Fatalf("expected decoded data, got %q", string(data))
	}
}
```

- [ ] **Step 2: Clean vault imports and save path handling**

In `internal/vault/vault.go`, keep only imports needed for YAML metadata:

```go
import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)
```

Replace `SaveToFile` with:

```go
func (v *Vault) SaveToFile(filePath string) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling vault to yaml: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o664); err != nil {
		return fmt.Errorf("writing vault data to file %s: %w", filePath, err)
	}
	return nil
}
```

Remove the pre-create block that ignores the `os.Create` error.

- [ ] **Step 3: Verify no archive, crypto, or keystore responsibilities remain in vault**

Run:

```bash
rg "zip|Aes|KeyMapping|GetKeyForRepo|SaveKey|CreateZip|UnpackZip" internal/vault
```

Expected: no matches except old deleted files in git history are irrelevant because `rg` searches the working tree.

- [ ] **Step 4: Run tests**

Run: `gofmt -w internal/vault/vault.go internal/vault/vault_test.go internal/app/encrypt.go internal/app/decrypt.go`

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit Task 5**

```bash
git add internal/vault/vault.go internal/vault/vault_test.go internal/app/encrypt.go internal/app/decrypt.go
git commit -m "refactor: limit vault package to metadata"
```

---

### Task 6: Normalize Config Loading And Selection

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/app/encrypt.go`

**Interfaces:**
- Consumes: existing `.urv.yaml` fields.
- Produces:
  - `func Default() *Config`
  - `func Load(repoPath string) (*Config, error)`
  - `func (c *Config) ArchiverName() string`
  - `func (c *Config) CipherName() string`

- [ ] **Step 1: Add config tests**

Add tests to `internal/config/config_test.go`:

```go
func TestLoadKeepsExistingFieldsIncludingCypher(t *testing.T) {
	dir := t.TempDir()
	body := []byte("secretfiles:\n  - .env\npatterns:\n  - \"*.secret.*\"\narchiver: zip\ncypher: aes-gcm\n")
	if err := os.WriteFile(filepath.Join(dir, ".urv.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("expected load to succeed, got %v", err)
	}
	if cfg.ArchiverName() != "zip" {
		t.Fatalf("expected zip archiver, got %q", cfg.ArchiverName())
	}
	if cfg.CipherName() != "aes-gcm" {
		t.Fatalf("expected aes-gcm cipher, got %q", cfg.CipherName())
	}
}

func TestConfigDefaultsArchiverAndCypherWhenEmpty(t *testing.T) {
	cfg := &Config{}
	if cfg.ArchiverName() != "zip" {
		t.Fatalf("expected default archiver zip, got %q", cfg.ArchiverName())
	}
	if cfg.CipherName() != "aes-gcm" {
		t.Fatalf("expected default cipher aes-gcm, got %q", cfg.CipherName())
	}
}
```

- [ ] **Step 2: Run config tests to verify missing methods failure**

Run: `go test ./internal/config`

Expected: FAIL because `ArchiverName` and `CipherName` are undefined.

- [ ] **Step 3: Implement config defaults and selectors**

In `internal/config/config.go`, add:

```go
const (
	defaultArchiver = "zip"
	defaultCypher   = "aes-gcm"
)

func Default() *Config {
	return &Config{
		SecretFiles: []string{".env"},
		Patterns:    []string{"*.secret.*"},
		Archiver:    defaultArchiver,
		Cypher:      defaultCypher,
	}
}

func (c *Config) ArchiverName() string {
	if c.Archiver == "" {
		return defaultArchiver
	}
	return c.Archiver
}

func (c *Config) CipherName() string {
	if c.Cypher == "" {
		return defaultCypher
	}
	return c.Cypher
}
```

Replace `defaultConfig()` usage with `Default()`. Keep `ConfigProvider` if tests still cover it, but make it call the same YAML unmarshal/default path as `Load` where practical.

- [ ] **Step 4: Use config-selected cipher in encrypt**

In `internal/app/encrypt.go`, replace:

```go
cipher, err := urvcrypto.NewCipher(vault.VaultAlgo, key)
```

with:

```go
cipher, err := urvcrypto.NewCipher(cfg.CipherName(), key)
```

When creating the new vault, keep the YAML `algo` field as `aes-gcm` through `vault.NewVaultFromData` for current compatibility. Because only `aes-gcm` is supported, this still matches the selected cipher.

- [ ] **Step 5: Run tests**

Run: `gofmt -w internal/config/config.go internal/config/config_test.go internal/app/encrypt.go`

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit Task 6**

```bash
git add internal/config/config.go internal/config/config_test.go internal/app/encrypt.go
git commit -m "refactor: normalize config defaults"
```

---

### Task 7: Compose App With Explicit Dependencies

**Files:**
- Modify: `internal/app/encrypt.go`
- Modify: `internal/app/decrypt.go`
- Modify: `internal/app/app_test.go`

**Interfaces:**
- Consumes: `archive.Archiver`, `crypto.NewCipher`, `keystore.FileStore`, `config.Config`.
- Produces:
  - `type Services struct { Archiver archive.Archiver; KeyStore *keystore.FileStore }`
  - `func DefaultServices() Services`
  - `func EncryptRepoWithServices(repoPath string, services Services) (*EncryptResult, error)`
  - `func DecryptRepoWithServices(repoPath string, services Services) error`

- [ ] **Step 1: Add service-level app test**

Add this test to `internal/app/app_test.go` after updating imports for `archive` and `keystore`:

```go
func TestEncryptDecryptRepoWithServicesRoundTrip(t *testing.T) {
	repoPath := t.TempDir()
	homePath := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repoPath, ".urv.yaml"), "secretfiles:\n  - .env\n", 0o644)
	writeFile(t, filepath.Join(repoPath, ".env"), "API_KEY=two\n", 0o600)

	store := keystore.NewFileStore(homePath)
	if err := store.SaveKey(appTestKey, repoPath, "repo-key"); err != nil {
		t.Fatal(err)
	}

	services := Services{
		Archiver: archive.NewZipArchiver(),
		KeyStore: store,
	}

	if _, err := EncryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected encrypt to succeed, got %v", err)
	}
	if err := os.Remove(filepath.Join(repoPath, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := DecryptRepoWithServices(repoPath, services); err != nil {
		t.Fatalf("expected decrypt to succeed, got %v", err)
	}
	got, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "API_KEY=two\n" {
		t.Fatalf("unexpected env contents: %q", string(got))
	}
}
```

- [ ] **Step 2: Run app tests to verify missing services failure**

Run: `go test ./internal/app`

Expected: FAIL because `Services` and `EncryptRepoWithServices` are undefined.

- [ ] **Step 3: Implement Services**

At the top of `internal/app/encrypt.go` or in a new `internal/app/services.go`, add:

```go
type Services struct {
	Archiver archive.Archiver
	KeyStore *keystore.FileStore
}

func DefaultServices() Services {
	return Services{
		Archiver: archive.NewZipArchiver(),
		KeyStore: keystore.NewDefaultFileStore(),
	}
}

func (s Services) withDefaults() Services {
	if s.Archiver == nil {
		s.Archiver = archive.NewZipArchiver()
	}
	if s.KeyStore == nil {
		s.KeyStore = keystore.NewDefaultFileStore()
	}
	return s
}
```

If this creates import clutter in `encrypt.go`, create `internal/app/services.go`.

- [ ] **Step 4: Refactor encrypt workflow into service variant**

Change `EncryptRepo`:

```go
func EncryptRepo(repoPath string) (*EncryptResult, error) {
	return EncryptRepoWithServices(repoPath, DefaultServices())
}
```

Add:

```go
func EncryptRepoWithServices(repoPath string, services Services) (*EncryptResult, error) {
	services = services.withDefaults()
	cfg, err := config.Load(repoPath)
	if err != nil {
		return nil, err
	}

	key, err := services.KeyStore.KeyForRepo(repoPath)
	if err != nil {
		return nil, err
	}

	foundFiles, err := files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)
	if err != nil {
		return nil, fmt.Errorf("listing files for encryption failed: %w", err)
	}

	hashes, err := files.NewFileHashCollection(repoPath, foundFiles)
	if err != nil {
		return nil, fmt.Errorf("hash state creation failed: %w", err)
	}

	vaultPath := filepath.Join(repoPath, vault.VaultFileName)
	oldVault, err := vault.NewVaultFromFilePath(vaultPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading old vault: %w", err)
	}

	oldHashes := map[string]string{}
	if oldVault != nil {
		oldHashes = oldVault.Hashes
	}

	lockPath := filepath.Join(repoPath, files.LockFileName)
	oldLockfile, err := files.OpenLockFile(repoPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading old lockfile: %w", err)
	}
	if len(oldHashes) == 0 && err == nil {
		oldHashes, err = files.ParseLockFileBody(oldLockfile)
		if err != nil {
			return nil, fmt.Errorf("parsing old lockfile: %w", err)
		}
	}

	if oldVault != nil && files.HashesEqual(hashes.Hashes, oldHashes) {
		oldVault.Version = vault.VaultVersion
		oldVault.Algo = vault.VaultAlgo
		oldVault.Hashes = hashes.Hashes
		if err := oldVault.SaveToFile(vaultPath); err != nil {
			return nil, err
		}
		if err := removeOldLockFile(lockPath); err != nil {
			return nil, err
		}
		return &EncryptResult{Encrypted: false}, nil
	}

	data, err := services.Archiver.Pack(repoPath, foundFiles)
	if err != nil {
		return nil, fmt.Errorf("creating secret archive: %w", err)
	}

	cipher, err := urvcrypto.NewCipher(cfg.CipherName(), key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	encryptedData, err := cipher.Encrypt(data)
	if err != nil {
		return nil, fmt.Errorf("encryption error: %w", err)
	}

	v := vault.NewVaultFromData(encryptedData, hashes.Hashes)
	if err = v.SaveToFile(vaultPath); err != nil {
		return nil, err
	}
	if err := removeOldLockFile(lockPath); err != nil {
		return nil, err
	}
	return &EncryptResult{Encrypted: true}, nil
}
```

- [ ] **Step 5: Refactor decrypt workflow into service variant**

Change `DecryptRepo`:

```go
func DecryptRepo(repoPath string) error {
	return DecryptRepoWithServices(repoPath, DefaultServices())
}
```

Add:

```go
func DecryptRepoWithServices(repoPath string, services Services) error {
	services = services.withDefaults()
	key, err := services.KeyStore.KeyForRepo(repoPath)
	if err != nil {
		return err
	}

	v, err := vault.NewVaultFromFilePath(filepath.Join(repoPath, vault.VaultFileName))
	if err != nil {
		return err
	}
	if err := v.ValidateForDecrypt(); err != nil {
		return err
	}

	vaultData, err := v.GetByteData()
	if err != nil {
		return err
	}

	cipher, err := urvcrypto.NewCipher(v.Algo, key)
	if err != nil {
		return err
	}
	decryptedArch, err := cipher.Decrypt(vaultData)
	if err != nil {
		return err
	}

	return services.Archiver.Unpack(repoPath, decryptedArch, true)
}
```

- [ ] **Step 6: Run tests**

Run: `gofmt -w internal/app/encrypt.go internal/app/decrypt.go internal/app/app_test.go internal/app/services.go`

If `services.go` was not created, omit it from the command.

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 7: Commit Task 7**

```bash
git add internal/app/encrypt.go internal/app/decrypt.go internal/app/app_test.go internal/app/services.go
git commit -m "refactor: compose app workflows with services"
```

If `services.go` was not created, omit it from `git add`.

---

### Task 8: Remove Stale Names, Wrappers, And Dead Responsibilities

**Files:**
- Modify: `internal/files/hash.go`
- Modify: `internal/files/hash_test.go`
- Delete or ensure absent: `internal/vault/crypto.go`
- Delete or ensure absent: `internal/vault/crypto_test.go`
- Delete or ensure absent: `internal/vault/key.go`

**Interfaces:**
- Consumes: all new package boundaries from previous tasks.
- Produces: no remaining key, crypto, or archive behavior in `internal/vault`; corrected hash collection naming.

- [ ] **Step 1: Rename misspelled hash collection type**

In `internal/files/hash.go`, rename:

```go
type FileHasheCollection struct {
	Hashes map[string]string
}
```

to:

```go
type FileHashCollection struct {
	Hashes map[string]string
}
```

Update:

```go
func NewFileHashCollection(basePath string, files []string) (*FileHashCollection, error)
```

and constructor allocation:

```go
res := &FileHashCollection{
	Hashes: map[string]string{},
}
```

- [ ] **Step 2: Verify no old vault responsibility references remain**

Run:

```bash
rg "vault\\.(AesGcm|GetKeyForRepo|SaveKey|GenNewKey|NewKeyMapping|CreateZipVaultData|UnpackZipVaultData)" .
```

Expected: no matches.

- [ ] **Step 3: Delete obsolete wrapper files if they still exist**

If these files still exist only as wrappers or migrated code, delete them:

```bash
git rm internal/vault/crypto.go internal/vault/crypto_test.go internal/vault/key.go
```

If a file is already gone, do not recreate it.

- [ ] **Step 4: Run package boundary checks**

Run:

```bash
rg "archive/zip|crypto/aes|crypto/cipher|\\.config.*urv|mapping.yaml|keys/" internal/vault internal/app cmd
```

Expected:

- `archive/zip`, `crypto/aes`, and `crypto/cipher` appear only in `internal/archive` or `internal/crypto`, not in `internal/vault`, `internal/app`, or `cmd`.
- `.config`, `mapping.yaml`, and `keys/` appear only in `internal/keystore` and docs/tests.

- [ ] **Step 5: Run tests**

Run: `gofmt -w internal/files/hash.go internal/files/hash_test.go`

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit Task 8**

```bash
git add internal/files/hash.go internal/files/hash_test.go internal/vault/crypto.go internal/vault/crypto_test.go internal/vault/key.go
git commit -m "refactor: remove stale vault responsibilities"
```

If deleted files are already staged by `git rm`, this commit records the deletions.

---

### Task 9: Update Documentation And Final Verification

**Files:**
- Modify: `README.md`
- Modify: `doc/review.md`
- Modify: `doc/designs/solid-refactor.md`

**Interfaces:**
- Consumes: final package layout after Tasks 1-8.
- Produces: documentation aligned with refactored repository.

- [ ] **Step 1: Update README project structure**

In `README.md`, replace the `internal/vault` bullet:

```md
- `internal/vault/` handles key storage, zip archive creation, AES-GCM encryption, and vault YAML files.
```

with:

```md
- `internal/app/` coordinates encrypt and decrypt workflows.
- `internal/archive/` packs and unpacks vault archive data. The current implementation uses ZIP.
- `internal/crypto/` encrypts and decrypts archive bytes. The current implementation uses AES-GCM.
- `internal/keystore/` manages local key files and repository-to-key mappings.
- `internal/vault/` reads, validates, and writes `.urv.vault.yaml`.
```

Keep the existing `internal/files`, `internal/repo`, and `internal/config` bullets.

- [ ] **Step 2: Update review findings**

In `doc/review.md`, mark these findings as resolved if the implementation completed them:

```md
9. ~**P2: Vault metadata is not validated**~
10. ~**P2: File discovery can silently produce wrong vault contents**~
11. ~**P3: `CheckGitignore` is broken and currently unsafe to use**~
12. ~**P3: Tests miss the highest-risk behavior**~
```

For finding 12, add a short note after the item:

```md
   Added package and workflow coverage for archive, crypto, keystore, file discovery, vault metadata, and app encrypt/decrypt behavior.
```

- [ ] **Step 3: Update SOLID refactor note**

Replace `doc/designs/solid-refactor.md` with:

```md
# Layered refactor

The rough SOLID refactor idea has been formalized in:

- `docs/superpowers/specs/2026-07-31-layered-refactor-design.md`
- `docs/superpowers/plans/2026-07-31-layered-refactor.md`

The accepted direction is a layered refactor with compatibility adapters:

- `internal/app` coordinates workflows.
- `internal/archive` owns ZIP archive packing and unpacking.
- `internal/crypto` owns AES-GCM encryption and decryption.
- `internal/keystore` owns local keys and repo mappings.
- `internal/vault` owns `.urv.vault.yaml` metadata compatibility.

Existing `.urv.yaml`, `.urv.vault.yaml`, local key files, and mapping files remain valid.
```

- [ ] **Step 4: Run final verification**

Run:

```bash
gofmt -w internal cmd
go test ./...
go vet ./...
```

Expected: all commands PASS.

- [ ] **Step 5: Inspect final diff**

Run:

```bash
git status --short
git diff --stat
```

Expected: only intended refactor, tests, and docs files changed.

- [ ] **Step 6: Commit Task 9**

```bash
git add README.md doc/review.md doc/designs/solid-refactor.md
git commit -m "docs: update layered refactor notes"
```

---

## Final Completion Checklist

- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `gofmt -w internal cmd` has been run.
- [ ] Existing `.urv.yaml` files still load.
- [ ] Existing `.urv.vault.yaml` files still decrypt when the matching local key exists.
- [ ] Existing `~/.config/urv/mapping.yaml` format still maps repo paths to key names.
- [ ] New key files are created with `0600`.
- [ ] ZIP archive entries remain repository-relative paths.
- [ ] `internal/vault` no longer imports ZIP, AES, or key-store behavior.
- [ ] Documentation matches the final package layout.

## Self-Review Notes

- Spec coverage: every goal and compatibility rule maps to Tasks 1-9.
- Placeholder scan: no placeholder tasks remain; each task includes concrete files, signatures, code snippets, commands, and expected outcomes.
- Type consistency: `archive.Archiver`, `crypto.Cipher`, `keystore.FileStore`, `app.Services`, `EncryptRepoWithServices`, and `DecryptRepoWithServices` are defined before later tasks consume them.
