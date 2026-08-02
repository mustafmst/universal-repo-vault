# Security Release Readiness Review

## Context

This review covers the current `main` branch state of `universal-repo-vault` after the `urv status` feature landed at commit `7920092`. The target users are personal homelab users and small teams that want to keep encrypted secret material in a public or private Git repository.

URV stores selected plaintext repository files in `.urv.vault.yaml` using a locally stored key. The intended public-hosting posture is:

- Commit `.urv.yaml`.
- Commit `.urv.vault.yaml`.
- Do not commit local key files.
- Do not commit real plaintext secret files.
- Use `urv status` before committing to detect stale vaults and broken setup.

## Verdict

URV is suitable for continued personal experimentation and private repository use, but it is **not yet release-ready as a recommended tool for public-hosted personal or small-team repositories**.

The main cryptographic primitive is reasonable for this stage: AES-GCM is used with random nonces, generated keys are 32 random bytes, and vault decrypt rejects tampered ciphertext. The highest remaining risks are practical workflow risks around public Git hosting, destructive decrypt behavior, key validation consistency, and metadata disclosure.

## Threat Model

This review considers:

- A public repository observer who can read `.urv.yaml`, `.urv.vault.yaml`, file names, file hashes, docs, and history.
- A local user or process that can read files under the same OS account.
- A user accidentally committing plaintext secrets to Git.
- A small team manually sharing keys outside URV.
- A malicious or corrupted `.urv.vault.yaml` committed to the repository.
- Interrupted writes, partial files, or accidental local edits during normal CLI use.

This review does not cover:

- Host compromise where the attacker can read the local URV key file.
- Malicious changes to the URV binary itself.
- Hardware-backed key storage.
- Strong multi-recipient/team key exchange.
- Formal cryptographic proof of the vault format.

## What Is Sound

- AES-GCM is used as an authenticated encryption mode in `internal/crypto/crypto.go`.
- Encryption uses a random nonce from `crypto/rand` for each encrypt operation.
- Decrypt rejects tampered ciphertext through GCM authentication.
- Generated keys use 32 random bytes and are hex encoded.
- New key files are created with `0600` permissions in `internal/keystore/keystore.go`.
- Archive unpacking rejects unsafe ZIP paths using `filepath.IsLocal`.
- `urv status` now detects many unsafe config states before hashing files.
- `.git` is skipped during configured file discovery.
- The current tests cover crypto round trips, random nonce behavior, tamper rejection, unsafe archive paths, key permissions, status file states, malformed vaults, and unsafe status selections.

## Findings

### Blocker: Decrypt Overwrites Files By Default

Evidence: `internal/app/decrypt.go:43` calls `services.Archiver.Unpack(repoPath, decryptedArch, true)`.

`urv decrypt` currently overwrites existing working-tree files without a dry run, prompt, backup, or default no-overwrite mode. This is a release blocker for public-hosted personal/small-team use because decrypt is a normal onboarding and recovery operation. A user can lose local changes by running the expected command.

Minimum fix:

- Add `urv decrypt --dry-run`.
- Add `urv decrypt --no-overwrite`.
- Consider making no-overwrite the default before a public release.
- Add tests for create, overwrite, no-overwrite failure, nested paths, and dry-run output.

### Blocker: Public Hosting Workflow Does Not Prevent Plaintext Secret Commits

Evidence: `.gitignore` only ignores `.env`; `README.md` tells users not to commit plaintext secrets, but there is no enforced Git staging check.

URV protects files only when users remember to run the right commands. A public repo observer can read any accidentally committed plaintext secret immediately. `urv status` helps, but it does not inspect the Git index and does not fail as a pre-commit gate.

Minimum fix:

- Add `urv check` with a non-zero exit code for unsafe states.
- Detect configured plaintext files staged for commit.
- Document a pre-commit hook that runs `urv status` or `urv check`.
- Report configured plaintext files that are not ignored by Git.

### High: Vault Metadata Leaks File Names And Plaintext Hashes

Evidence: `.urv.vault.yaml` contains `hashes` keyed by repository-relative file paths.

The encrypted payload hides file contents, but the vault file exposes which files are protected and a SHA-256 hash of each plaintext file. This allows a public observer to learn names such as `.env`, Kubernetes secret paths, or Ansible variable paths. For low-entropy or common example files, hashes may also enable offline guessing.

This may be acceptable for personal homelab use if documented clearly, but it is not private metadata.

Minimum fix:

- Document that `.urv.vault.yaml` exposes protected file paths and content hashes.
- For a later vault format, consider storing encrypted metadata or keyed hashes.
- Warn users not to encode sensitive environment names, hostnames, or service names in secret file paths if publishing publicly.

### High: Key Validation Is Inconsistent Between Health Checks And Real Use

Evidence: `internal/keystore/keystore.go:217-254` validates hex encoding in `HealthForRepo`, while `KeyForRepo` at `internal/keystore/keystore.go:191-214` only checks key length before returning the key.

`urv status` can identify non-hex key files, but encryption and decryption still rely on later cipher construction to reject them. This is not a direct confidentiality failure, but inconsistent validation weakens diagnostics and makes the key path easier to misuse.

Minimum fix:

- Extract one shared key parsing/validation helper.
- Use it from both `KeyForRepo` and `HealthForRepo`.
- Keep returning only the hex string from `KeyForRepo` unless the crypto boundary changes.
- Add tests proving `KeyForRepo` rejects non-hex 64-character keys.

### High: Small-Team Key Sharing Is Manual And Under-Specified

Evidence: README says “add or map the same key” on another machine, but does not define a safe transfer workflow.

Small teams need to move the symmetric key out-of-band. URV currently provides no key export warning, key fingerprint, recipient model, rotation command, or documented secure transfer process. In practice, users may paste keys into chat, email, shell history, or issue trackers.

Minimum fix:

- Document a safe manual transfer workflow and explicitly warn against public channels.
- Add `urv keys current` or fingerprint output so users can verify they mapped the expected key without printing the key.
- Add key rotation as a later milestone.

### Medium: Vault Writes Are Not Atomic

Evidence: `internal/vault/vault.go:42-49` writes `.urv.vault.yaml` directly with `os.WriteFile`.

An interrupted encrypt can leave a partial or corrupt vault file. This is not primarily a confidentiality problem, but it affects availability and can create confusing public commits if the broken vault is committed.

Minimum fix:

- Write vault data to a temporary file in the same directory.
- `fsync` where practical.
- Rename atomically over `.urv.vault.yaml`.
- Preserve current file format.

### Medium: Vault File Permissions Are Broad For Local Multi-User Machines

Evidence: `internal/vault/vault.go:47` writes vault files with `0664`.

The vault is designed to be committed, so broad read access is acceptable for the encrypted file itself. However, on a local shared machine, group-writable vault files can be modified by another local user in the same group. AES-GCM prevents undetected ciphertext tampering from decrypting successfully, but a local user can still cause denial of service or misleading Git diffs.

Minimum fix:

- Prefer `0644` for repository vault files unless there is a clear group-write requirement.
- Consider documenting local filesystem assumptions.

### Medium: ZIP Unpack Does Not Limit Decompressed Size Or File Count

Evidence: `internal/archive/archive.go:65-78` iterates all ZIP entries and copies contents without size limits.

A malicious vault that decrypts successfully can contain a large archive and exhaust disk space during decrypt. Because successful decryption requires the repository key, this mostly matters for compromised team workflows or accidental re-encryption of a bad archive.

Minimum fix:

- Add sane limits for file count and total decompressed bytes.
- Report clear errors when limits are exceeded.
- Keep limits configurable later only if users actually need it.

### Medium: Public Repo Includes Dummy Plaintext Secret Examples

Evidence: tracked files include `example-files/example.secret.yaml`, `example-files/example.secret.json`, and `example-files/example.env` contains a dummy `EXAMPLE_SECRET`.

These are obviously dummy examples, not real credentials. Still, public scanners and humans may flag them as secrets, and users may copy the pattern into real repositories.

Minimum fix:

- Rename values to unmistakable dummy strings such as `example-not-a-real-secret`.
- Add a short README note that example plaintext files are intentionally fake and should not be copied with real values.
- Consider moving plaintext examples under a directory name that makes their purpose explicit.

### Low: Toolchain And Dependency Posture Is Not Documented

Evidence: `go.mod` targets `go 1.26.1`, and README repeats this target.

The target Go version is unusual relative to current stable Go releases. For release readiness, users should know whether this is intentional or a local toolchain artifact. Dependencies are limited to Cobra/Viper and transitive packages, but no dependency audit process is documented.

Minimum fix:

- Confirm the intended Go version.
- Add a release checklist entry for `go test ./...`, `go vet ./...`, and dependency review.
- Consider using a current stable Go version if `1.26.1` is not intentional.

### Low: Key Mapping Reveals Local Repository Paths

Evidence: `~/.config/urv/mapping.yaml` maps absolute repo paths to key names.

This file is local machine state and should not be committed, but if copied into logs or issue reports it reveals local paths and project names.

Minimum fix:

- Document that `mapping.yaml` is local sensitive metadata.
- Avoid printing full mapping paths in future commands unless requested.

## Public Hosting Checklist

Before recommending URV for public-hosted repositories:

- `.urv.yaml` contains only intentional file names and patterns.
- `.urv.vault.yaml` contains no unexpected file paths in `hashes`.
- Real plaintext secret files are not tracked.
- Real plaintext secret files are ignored or caught by a pre-commit check.
- `urv status` reports `safe` before commit.
- The local key file is not inside the repository.
- The local key was transferred only through a private channel.
- The user understands that vault metadata is public.
- `urv decrypt` cannot overwrite local files unexpectedly, or the user has explicitly opted into overwrite behavior.

## Minimum Release Gate

URV should not be described as release-ready for public-hosted personal or small-team use until these items are complete:

1. Add safe decrypt controls and tests.
2. Add a scriptable `urv check` command or equivalent pre-commit gate.
3. Document vault metadata disclosure.
4. Unify key validation between `KeyForRepo` and `HealthForRepo`.
5. Document safe small-team key transfer and key verification.
6. Make vault writes atomic.
7. Clarify dummy example secrets and public-hosting guidance in README.

## Recommended Next Milestone

The next implementation milestone should be **public-hosting guardrails**:

- `urv decrypt --dry-run`
- `urv decrypt --no-overwrite`
- `urv check`
- README public-hosting warning section
- shared key validation helper

This milestone directly addresses the two blockers and the most important high-severity workflow risks without introducing a large team-key architecture.
