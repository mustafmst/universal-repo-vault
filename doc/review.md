# Code Review Findings

1. ~**P0: Decrypt can write files outside the repository**~
   `internal/vault/vault.go:118-121`

   `UnpackZipVaultData` trusts `zf.Name` from the vault and joins it directly with `basePath`. A crafted vault entry like `../../.ssh/authorized_keys` or an absolute path can escape the repo and overwrite arbitrary files during `urv decrypt`.

   Change needed: reject absolute paths, `..` path traversal, and paths that do not remain under `basePath` after cleaning/evaluating.

2. ~**P0: Encrypt can leave `.urv.lock` updated while vault encryption failed**~
   `cmd/encrypt/encrypt.go:43-74`

   The command writes `.urv.lock` before zip creation, encryption, and vault write complete. If any later step fails, the lockfile says the plaintext files are already encrypted, and the next run can skip encryption because hashes match.

   Change needed: build archive, encrypt, and write vault successfully before replacing `.urv.lock`, ideally with temp files and atomic rename.

3. ~**P0: Key files are created with default filesystem permissions**~
   `internal/vault/key.go:113`

   `os.Create` uses `0666` before umask, commonly resulting in `0644`. Encryption keys may become readable by other local users.

   Change needed: create key files with `0600`, and consider enforcing/chmodding existing key files.

4. ~**P1: Several CLI errors are hidden or terminate outside Cobra error handling**~
   `cmd/keys/gen/gen.go:15-18`, `cmd/keys/add/add.go:19-23`, `cmd/keys/add/add.go:31-34`, `cmd/encrypt/encrypt.go:64-69`, `cmd/initcmd/init.go:21-27`

   Some commands return `nil` after failure, so the process exits successfully even though nothing worked. Other paths call `log.Fatalf` inside command handlers, bypassing `RunE` error handling and any cleanup.

   Change needed: return errors from commands instead of logging success/fatal exits.

5. ~**P1: Encrypt ignores critical errors before using results**~
   `cmd/encrypt/encrypt.go:40-50`

   `ListAllConfiguredFiles`, `NewFileHashCollection`, and `GetHexHash` errors are ignored or overwritten. If file walking or hashing fails, the command can panic on `hashes.GetLockfileBody()` or proceed with incomplete state.

   Change needed: check every returned error before using the result.

6. ~**P1: Decrypt does not create parent directories and can leak file handles**~
   `internal/vault/vault.go:121-131`

   `os.Create(fullPath)` fails if the zip contains nested paths whose directories do not exist. The opened destination file and zip entry reader are also not closed in the loop.

   Change needed: create parent directories with safe permissions and close both file handles per entry.

7. ~**P2: Existing files are not handled safely during decrypt**~
   `internal/vault/vault.go:121-126`

   `os.Create` already truncates existing files, so the `os.ErrExist` branch is effectively dead. If it did run, `os.Open` opens read-only, so `io.Copy` would fail.

   Change needed: explicitly decide overwrite behavior, use `os.OpenFile` with correct flags, and consider a `--force` or backup strategy.

8. ~**P2: `ConfigProvider.Get` cannot deserialize config correctly**~
   `internal/config/config.go:52`

   `yaml.Unmarshal(rawData, c)` passes a non-pointer struct. This will fail if `ConfigProvider.Get` is used, returning defaults plus an error instead of loaded config.

   Change needed: pass `&c`. Also add a test for `ConfigProvider`.

9. **P2: Vault metadata is not validated**
   `internal/vault/vault.go:27-29`, `cmd/decrypt/decrypt.go:25-30`

   `GetByteData` ignores invalid hex, and decrypt never checks `v.Algo`. A corrupt or unsupported vault gives misleading AES errors or decrypts empty data.

   Change needed: make hex decode return an error and reject unsupported `algo` values.

10. **P2: File discovery can silently produce wrong vault contents**
    `internal/files/files.go:33-42`

    Invalid glob patterns are ignored, and files can be added twice if they match both explicit `secretfiles` and `patterns`.

    Change needed: return pattern errors and de-duplicate discovered paths before hashing/zipping.

11. **P3: `CheckGitignore` is broken and currently unsafe to use**
    `internal/repo/checks.go:78-85`

    It opens `.gitignore` read-only and then writes to it, which will fail. It also does not append a newline or check for an existing `.urvtemp` entry.

    Change needed: open with append/write flags and avoid duplicate entries, or remove the function if unused.

12. **P3: Tests miss the highest-risk behavior**

    Current tests cover AES and some config/repo basics, but not encrypt/decrypt workflow, lockfile consistency, key permissions, zip traversal, duplicate file discovery, or command error exits.

    Change needed: add tests around the CLI workflow and vault unpacking before refactoring further.
