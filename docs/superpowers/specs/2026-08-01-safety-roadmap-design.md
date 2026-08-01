# Safety-First URV Roadmap Design

## Context

`universal-repo-vault` is a small Go/Cobra CLI for encrypting selected repository files into `.urv.vault.yaml` and decrypting them back into the working tree. The current repository has already been refactored into clearer layers:

- `cmd/*` defines Cobra commands.
- `internal/app` coordinates encrypt and decrypt workflows.
- `internal/archive` owns ZIP packing and unpacking.
- `internal/config` owns `.urv.yaml`.
- `internal/crypto` owns AES-GCM encryption.
- `internal/files` owns configured file discovery and hashing.
- `internal/keystore` owns local keys and repository mappings.
- `internal/repo` owns Git repository detection and `.gitignore` helpers.
- `internal/vault` owns `.urv.vault.yaml` metadata and data encoding.

The next feature work should serve a single-user homelab repository first. URV should stay simple, local-first, and hard to misuse. Small-team features can come later unless they also improve the single-user safety workflow.

## Goals

- Help the user answer "is this repository safe to commit?" before changing files.
- Make decrypt safer by exposing overwrite behavior before it happens.
- Catch configuration mistakes that would leave files unprotected.
- Improve key and vault diagnostics without exposing secret values.
- Keep the CLI small and predictable.
- Reuse the existing package boundaries created by the layered refactor.

## Non-Goals

- No hosted service, daemon, agent, sync system, or remote secret backend.
- No multi-user recipient model in the first safety roadmap.
- No breaking change to `.urv.yaml`, `.urv.vault.yaml`, key files, or mapping files.
- No migration away from ZIP or AES-GCM.
- No automatic deletion of plaintext secret files as part of the initial roadmap.

## Ranking Method

Features are ranked by:

1. Safety impact: how much the feature reduces accidental plaintext commits, stale vaults, destructive decrypts, or broken key workflows.
2. Fit with current architecture: how naturally it can be added through existing packages.
3. Implementation effort: smaller features with high safety value rank earlier.
4. Homelab usefulness: single-user workflows rank above team-sharing workflows.

## Ranked Features

### 1. `urv status`

`urv status` reports the current safety state of the repository without changing files.

It should show:

- Whether `.urv.yaml` exists and can be parsed.
- Whether `.urv.vault.yaml` exists and can be parsed.
- Whether a key is mapped for the current repository.
- Whether the mapped key file exists and has a valid length.
- Which configured files are present.
- Which configured explicit files are missing.
- Whether configured file hashes match the vault hashes.
- Which configured files are new, changed, unchanged, or missing from the vault.
- Whether configured patterns are valid.

Priority: highest.
Meaning: very high.
Effort: medium.

This is the first feature because it gives the user a clear answer before committing or decrypting. It also creates a shared internal status model that later features can reuse.

### 2. `urv check`

`urv check` is a stricter safety gate intended for scripts and pre-commit hooks. It should reuse the same internal status model as `urv status`, but return a non-zero exit code when the repository is unsafe.

The first version should fail when:

- `.urv.yaml` is missing or invalid.
- Configured patterns are invalid.
- The vault is missing.
- The key mapping is missing or broken.
- Any configured file has changed since the last vault encryption.
- Any configured file is missing from the vault.
- Any explicit configured file is missing from the working tree.

It may also warn, but not fail, when a pattern matches no files. A later version can add Git index checks for staged plaintext files.

Priority: high.
Meaning: very high.
Effort: medium.

This turns URV from a manual workflow into a safety rail. It should be suitable for `pre-commit`, CI, or a local shell alias.

### 3. Safe Decrypt Controls

`urv decrypt` should expose safer overwrite behavior.

Proposed flags:

- `--dry-run`: report which files would be written, created, or overwritten.
- `--no-overwrite`: fail if decrypt would replace an existing file.
- `--backup`: before overwriting, copy the existing file to a deterministic backup path.

The default can remain overwrite-compatible for now to avoid breaking existing behavior, but status output should make the overwrite behavior explicit.

Priority: high.
Meaning: high.
Effort: low to medium.

Current decrypt behavior is useful but destructive. These controls reduce the chance of losing local edits when restoring secrets.

### 4. Config Validation and Preview

URV should validate `.urv.yaml` more explicitly and show what the config protects.

Validation should detect:

- Invalid glob patterns.
- Unsupported `archiver` values.
- Unsupported `cypher` values.
- Explicit file paths that are absolute or escape the repository.
- Explicit files that do not exist.
- Patterns that match no files.
- Accidental inclusion of `.urv.vault.yaml`, `.urv.yaml`, `.git`, or local key files.

The first implementation should include this validation inside the shared status model used by `urv status` and future `urv check`. A separate `urv config validate` command can be added later only if the combined status output becomes too broad.

Priority: medium-high.
Meaning: high.
Effort: low to medium.

Configuration mistakes directly affect which files are protected. Validation is small but important safety work.

### 5. Git Ignore and Plaintext Safety Assistant

URV should help ensure plaintext secret files are not accidentally committed.

The first version should extend `urv status` to report configured plaintext files that are not ignored by Git. A later version can add `urv protect` or extend `urv init` to update `.gitignore` for configured exact files.

This feature should be careful with patterns. It should avoid blindly adding broad patterns to `.gitignore` unless the user explicitly asks.

Priority: medium.
Meaning: high.
Effort: medium.

The most severe user-facing failure is committing plaintext secrets. This feature addresses that risk while keeping the user in control.

### 6. Key Health Commands

Key management should provide better diagnostics without exposing key material.

Possible commands:

- `urv keys current`: show the key name mapped to the current repository and whether the file exists.
- `urv keys remove`: remove a key mapping, and optionally remove an unused local key file.
- Improved `urv keys list`: support a concise table and deterministic ordering.

Priority: medium.
Meaning: medium.
Effort: low.

Broken mappings are confusing, but they are less likely to cause plaintext leaks than stale vault or config problems.

### 7. Key Rotation

URV should eventually support re-encrypting the vault with a new or existing local key.

Possible command:

```sh
urv keys rotate --name <new-key-name>
```

The command should decrypt the current vault with the existing mapped key, encrypt it with the new key, update the repo mapping only after the new vault is written, and leave the old key file untouched unless explicitly requested.

Priority: medium-low.
Meaning: medium-high.
Effort: medium.

Key rotation matters, but it is less frequent in a single-user homelab than checking repository safety.

### 8. Vault Metadata Inspection

`urv vault info` should show non-secret vault metadata.

It should show:

- Vault version.
- Algorithm.
- Encrypted file count.
- File paths present in vault hashes.
- Whether encrypted data is present and valid hex.

Priority: low-medium.
Meaning: medium.
Effort: low.

This overlaps with `urv status`, so it should wait unless users need a narrower vault-only command.

## First Milestone Design

The first implementation milestone should be `urv status`.

### Status Model

Add an internal application-level status model in `internal/app` that can be used by both human-readable and machine-readable commands later.

The model should include:

- Config state.
- Vault state.
- Key mapping state.
- Configured file discovery state.
- Per-file hash comparison state.
- Warnings.
- Errors that make the repository unsafe.

The model should avoid printing directly. Command packages should format output.

### File States

Each configured file should have one of these states:

- `unchanged`: file exists and hash matches the vault.
- `changed`: file exists and hash differs from the vault.
- `new`: file exists but is not in the vault.
- `missing`: file is explicitly configured but missing from the working tree.
- `vault-only`: file is recorded in the vault but not currently discovered from config.

### Command Output

The human output should be concise and readable. It should start with an overall state such as `safe`, `needs encryption`, or `broken setup`, followed by grouped details.

The initial command does not need JSON output. JSON can be added later when `urv check` or automation users need it.

### Exit Codes

`urv status` should return success when it can inspect the repository, even if it reports warnings or changed files. It should return an error only when inspection itself fails unexpectedly.

`urv check`, when added later, should use the same model but fail on unsafe states.

## Architecture

The roadmap should preserve existing package responsibilities.

`cmd/*` remains thin. New commands should parse flags, call `internal/app`, and format output.

`internal/app` owns safety workflows and status aggregation. It can call config, vault, keystore, files, and repo packages, but it should not implement low-level YAML, ZIP, crypto, or Git file parsing directly.

`internal/files` should continue owning configured file discovery and hashing. It can grow small helpers for classifying file hash state when those helpers are independent of CLI presentation.

`internal/vault` should continue owning vault file parsing, validation, and metadata access.

`internal/keystore` should expose key health without returning secret values unless encryption or decryption needs the key itself.

`internal/repo` should own repository detection and Git-related checks such as ignore status.

## Testing

The first milestone should include focused tests for the status model and command output.

Test cases:

- No `.urv.yaml`.
- Invalid `.urv.yaml`.
- No `.urv.vault.yaml`.
- No key mapping.
- Key mapping exists but key file is missing.
- Key file has invalid length.
- Configured files are unchanged.
- Configured files changed after encryption.
- Configured file exists but is not in vault.
- Explicit configured file is missing.
- Vault contains a path no longer matched by config.
- Invalid glob pattern.

Verification commands:

```sh
go test ./...
go vet ./...
gofmt -w .
```

## Implementation Order

1. Add status model tests in `internal/app`.
2. Implement status aggregation in `internal/app`.
3. Add small package helpers only where needed for clean boundaries.
4. Add `cmd/status`.
5. Add root command wiring.
6. Add command output tests.
7. Update README with the new status workflow.

## Open Decisions

No open decisions are blocking the roadmap. The first implementation should start with `urv status`; later features should be revisited after the status model reveals which safety states are most useful in practice.
