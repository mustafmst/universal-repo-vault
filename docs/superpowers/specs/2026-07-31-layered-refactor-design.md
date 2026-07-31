# Layered Refactor Design

## Context

`universal-repo-vault` is a Go/Cobra CLI that encrypts configured repository files into `.urv.vault.yaml` and decrypts them back into the working tree. The current code already has a useful command/application split, but several responsibilities are still bundled together:

- `internal/vault` handles vault YAML, ZIP archive creation/extraction, AES-GCM metadata, and some filesystem behavior.
- `internal/vault/key.go` handles local key files and repo-to-key mapping.
- `internal/config` exposes both `Load` and `ConfigProvider`.
- `internal/files` handles discovery, hashing, lockfile parsing, and generic file writes.

The refactor should improve internal boundaries and prepare the project for future archivers and ciphers while preserving current behavior and all existing storage formats.

## Goals

- Keep the current CLI behavior and command names.
- Keep existing `.urv.yaml` files valid.
- Keep existing `.urv.vault.yaml` files valid.
- Keep existing local keys under `~/.config/urv/keys/<key-name>` valid.
- Keep existing repo mapping file `~/.config/urv/mapping.yaml` valid.
- Keep ZIP payload entries as repository-relative file paths.
- Make archiving and encryption replaceable through small internal interfaces.
- Fix cleanup issues discovered during repository analysis without changing user-facing formats.
- Add workflow-level tests so behavior is protected while code moves.

## Non-Goals

- No breaking vault format migration.
- No new supported archiver beyond ZIP in the first refactor.
- No new supported cipher beyond AES-GCM in the first refactor.
- No new CLI command structure.
- No removal of old `.urv.lock` read compatibility during this refactor.

## Architecture

The project should move toward explicit layers while keeping package count modest.

`cmd/*` remains thin. Commands resolve the current repository, call `internal/app`, and print user-facing status messages. Command handlers should return errors through Cobra instead of exiting directly.

`internal/app` owns workflows such as `EncryptRepo`, `DecryptRepo`, and key operations. It coordinates config loading, key lookup, file discovery, archiving, encryption, and vault persistence. It should not implement ZIP, AES, YAML storage, or local key-file details directly.

`internal/config` owns `.urv.yaml` compatibility. It should expose one clear loading API and default handling. The existing fields remain valid, including `secretfiles`, `patterns`, `archiver`, and `cypher`.

`internal/vault` owns `.urv.vault.yaml` compatibility. It stores and validates `version`, `algo`, `hashes`, and hex-encoded `data`. It should not create ZIP archives or encrypt data.

`internal/archive` is a new package with an `Archiver` interface and ZIP implementation. The ZIP implementation must produce and consume the same repository-relative entry layout used today.

`internal/crypto` is a new package with a `Cipher` interface and AES-GCM implementation. AES-GCM remains selected by the existing vault/config name `aes-gcm`.

`internal/keystore` is a new package for local key files and repo mapping. It preserves the current paths and YAML mapping format.

`internal/files` remains focused on repository file discovery, hashing, lockfile parsing, and small file utilities that are not specific to vaults or keys.

`internal/repo` remains focused on repository detection and `.gitignore` checks.

## Interfaces

The exact names can be adjusted during implementation, but the design should stay close to these boundaries:

```go
type Archiver interface {
    Pack(basePath string, relPaths []string) ([]byte, error)
    Unpack(basePath string, data []byte, overwrite bool) error
}

type Cipher interface {
    Encrypt(data []byte) ([]byte, error)
    Decrypt(data []byte) ([]byte, error)
    Name() string
}

type KeyStore interface {
    GenerateKey() (string, error)
    SaveKey(key string, repoPath string, keyName string) error
    UseKeyForRepo(keyName string, repoPath string) error
    KeyForRepo(repoPath string) (string, error)
    Mapping() (*KeyMapping, error)
}
```

The implementation can start with concrete constructors such as `archive.NewZipArchiver`, `crypto.NewAESGCMCipher`, and `keystore.NewFileStore`. Factories for config-selected archivers and ciphers should support only known values at first:

- `zip`
- `aes-gcm`

Unknown values should return explicit unsupported-type errors.

## Encrypt Flow

1. `cmd/encrypt` resolves the current repository path and calls `app.EncryptRepo`.
2. `app` loads `.urv.yaml` through `config`.
3. `app` asks `keystore` for the repo key.
4. `files` discovers configured files and computes hashes.
5. `vault` loads `.urv.vault.yaml` if it exists.
6. If hashes are unchanged, `app` preserves the existing vault data, refreshes current metadata if needed, removes old `.urv.lock`, and returns an unchanged result.
7. If files changed, `archive.ZipArchiver` packs the repository-relative paths.
8. `crypto.AESGCMCipher` encrypts the ZIP bytes.
9. `vault` writes `.urv.vault.yaml` with current fields: `version`, `algo`, `hashes`, and `data`.
10. `app` removes old `.urv.lock` if present.

## Decrypt Flow

1. `cmd/decrypt` resolves the current repository path and calls `app.DecryptRepo`.
2. `app` loads the key through `keystore`.
3. `vault` loads and validates `.urv.vault.yaml`.
4. `crypto` is selected from the vault `algo` value.
5. `vault` decodes the existing hex payload.
6. `crypto` decrypts the payload.
7. `archive.ZipArchiver` unpacks files safely into the repository.

The first refactor can keep the current overwrite behavior during decrypt. The archive layer should accept an `overwrite` argument so a future CLI flag can be added without changing the archive boundary.

## Key Flow

The `keys gen`, `keys add`, and `keys list` commands should continue to behave as they do today. Local key persistence moves behind `internal/keystore`, but these compatibility rules are fixed:

- Generated keys remain 32 random bytes encoded as 64 hex characters.
- Key files remain stored at `~/.config/urv/keys/<key-name>`.
- Repo mappings remain stored at `~/.config/urv/mapping.yaml`.
- Mapping YAML remains a map of repo path to key name.
- New key files are created with `0600` permissions.

## Error Handling

- Invalid glob patterns return errors instead of being ignored.
- File discovery returns de-duplicated, deterministic repository-relative paths.
- Key files are created with `0600`.
- All opened files, archive readers, and archive writers are closed.
- Decrypt rejects unsafe archive paths before writing.
- Unsupported vault version or algorithm fails before decrypting.
- Unsupported configured archiver or cipher fails before archiving/encrypting.
- Workflow functions wrap lower-level errors with enough context for CLI users and tests.
- Command handlers do not call `log.Fatal`, `panic`, or silently return success after failure.

## Compatibility Adapters

Old `.urv.lock` support remains as a read-only migration path for unchanged-hash detection. New encrypt runs should only write `.urv.vault.yaml`; they may remove `.urv.lock` after a successful vault write.

Existing vault YAML is the source of truth for encrypted data. The refactor may rename internal types and functions, but it must not rename YAML fields or change encoded data semantics.

The current config field name `cypher` should remain accepted to preserve compatibility. A later breaking-change discussion can decide whether to add a correctly spelled `cipher` alias.

## Testing

The refactor should be test-first around behavior.

Package-level coverage:

- `archive`: ZIP round-trip, nested paths, unsafe path rejection, repeated unpack writes.
- `crypto`: AES-GCM round-trip, random nonce behavior, invalid key length, invalid ciphertext.
- `vault`: YAML compatibility, invalid hex, unsupported version/algo, hash preservation.
- `keystore`: key generation length, `0600` file mode, mapping read/write compatibility, missing key errors.
- `files`: invalid glob errors, path de-duplication, deterministic ordering, `.git` skip behavior.
- `app`: full encrypt/decrypt workflow using temp repo and temp home dirs, unchanged-hash fast path, missing config/key/vault errors.
- `cmd`: focused Cobra tests for command success and failure messages where existing command patterns support it.

Verification commands:

```sh
go test ./...
go vet ./...
gofmt -w .
```

## Implementation Order

1. Add characterization tests around current encrypt/decrypt/key/file discovery behavior.
2. Extract `internal/archive` from ZIP functions while keeping behavior unchanged.
3. Extract `internal/crypto` from AES-GCM functions while keeping behavior unchanged.
4. Extract `internal/keystore` from key and mapping logic while preserving local file formats.
5. Simplify `internal/vault` to vault YAML and metadata concerns.
6. Update `internal/app` to compose the new packages.
7. Clean names and stale helpers after tests prove behavior is preserved.
8. Update docs to reflect the new package layout.

## Open Decisions

No open design decisions remain for this spec. The implementation should preserve current behavior and storage formats, with only internal package boundaries changing.
