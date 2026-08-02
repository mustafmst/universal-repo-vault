# universal-repo-vault

`universal-repo-vault` (`urv`) is a small CLI tool for storing selected repository files in an encrypted vault. It is intended for files such as `.env` files, Kubernetes secret manifests, Ansible variables, or other local configuration files that should not be committed in plaintext.

## Project Background and Idea

This project started from a problem I encountered while working on my small homelab. I set up a few small PCs as a k3s cluster to host lightweight automations and services. The plan is to manage everything in a single repository, using Kustomize for Kubernetes deployments and Ansible for machine setup.

There are already a few solutions that solve parts of this problem, and combining some of them would probably be enough. However, they all feel like too much hassle for my simple setup, and combining several tools increases the chance of making mistakes.

That is why I wanted to experiment a little and build a simple solution for storing, encrypting, and decrypting data in a repository.

## Goal

URV should make it easy to keep secret files out of Git while still keeping an encrypted copy of them in the repository. The current implementation can:

- Initialize repository configuration.
- Select files explicitly or by filename pattern.
- Generate and store local encryption keys.
- Map a local key to a repository.
- Encrypt selected files into `.urv.vault.yaml`.
- Track encrypted file hashes in `.urv.vault.yaml`.
- Decrypt the vault back into the working tree.
- Inspect repository vault safety status without changing files.

## Project Structure

- `main.go` starts the CLI.
- `cmd/` contains Cobra command definitions.
- `cmd/initcmd/` implements `urv init`.
- `cmd/encrypt/` implements `urv encrypt`.
- `cmd/decrypt/` implements `urv decrypt`.
- `cmd/keys/` implements key management commands.
- `internal/app/` coordinates encrypt and decrypt workflows.
- `internal/archive/` packs and unpacks vault archive data. The current implementation uses ZIP.
- `internal/config/` loads and writes `.urv.yaml`.
- `internal/crypto/` encrypts and decrypts archive bytes. The current implementation uses AES-GCM.
- `internal/files/` finds configured files and computes file hashes.
- `internal/keystore/` manages local key files and repository-to-key mappings.
- `internal/repo/` detects the current Git repository.
- `internal/vault/` reads, validates, and writes `.urv.vault.yaml`.
- `example-files/` contains example files used by the sample configuration.

## Installation

This project is written in Go. Make sure Go is installed before building it. The module currently targets Go `1.26.1`.

Clone the repository:

```sh
git clone https://github.com/mustafmst/universal-repo-vault.git
cd universal-repo-vault
```

Build the CLI:

```sh
make build
```

The binary will be created at:

```sh
dist/urv
```

You can optionally install the built binary to `~/.local/bin/urv`:

```sh
make install
```

During development, you can also run the CLI directly:

```sh
go run ./main.go
```

## Basic Workflow

Run URV commands from inside the Git repository that you want to manage.

Initialize URV in the repository:

```sh
urv init
```

If you are using the local build without installing it, run:

```sh
./dist/urv init
```

This creates `.urv.yaml` if it does not already exist.

Edit `.urv.yaml` and list the files that should be stored in the encrypted vault:

```yaml
secretfiles:
  - .env
patterns:
  - "*.secret.*"
```

Generate a new local encryption key for the current repository:

```sh
urv keys gen
```

Encrypt the configured files into the vault:

```sh
urv encrypt
```

This creates or updates:

- `.urv.vault.yaml`

Check repository safety before committing:

```sh
urv status
```

`safe` means the configured files match the vault metadata. `needs encryption` means at least one configured file is new, changed, missing, or only present in the vault metadata. `broken setup` means URV could not validate required setup such as config, vault, or local key mapping.

Commit `.urv.yaml` and `.urv.vault.yaml` to Git. Do not commit the plaintext secret files unless that is intentional.

On another machine, add or map the same key, then decrypt the vault:

```sh
urv decrypt
```

## Configuration

The repository configuration is stored in `.urv.yaml`.

Example:

```yaml
secretfiles:
  - example-files/example.env
patterns:
  - "*.secret.*"
```

`secretfiles` contains exact repository-relative file paths.

`patterns` contains filename patterns matched against file names while walking the repository. The current implementation skips `.git` and matches patterns such as `*.secret.*` against the base file name.

## Commands

Initialize URV configuration in the current Git repository:

```sh
urv init
```

Encrypt configured files:

```sh
urv encrypt
```

Show whether the repository is safe, needs encryption, or has broken setup:

```sh
urv status
```

Decrypt the vault into the working tree:

```sh
urv decrypt
```

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

Manage keys:

```sh
urv keys gen
urv keys add
urv keys list
```

## Public Hosting Notes

URV is designed to commit `.urv.yaml` and `.urv.vault.yaml`, but those files are not fully private metadata. `.urv.vault.yaml` exposes protected repository-relative file paths and SHA-256 hashes of plaintext file contents. Do not use sensitive hostnames, service names, or environment names in secret file paths if that metadata should stay private.

Before pushing to a public repository:

```sh
urv status
urv check
```

`urv check` exits non-zero when the repository is not safe to commit. It fails whenever a configured plaintext file is present in the Git index, including files committed in `HEAD`, and must never report success in that state. It is intended for local scripts, CI, and pre-commit hooks.

The files under `example-files/` contain dummy values only. Do not copy real secrets into tracked example files.

## Key Management

URV stores keys locally under:

```sh
~/.config/urv/keys/
```

Repository-to-key mappings are stored in:

```sh
~/.config/urv/mapping.yaml
```

Generate a new key and map it to the current repository using the repository directory name as the key name:

```sh
urv keys gen
```

Generate a new key with a custom key name:

```sh
urv keys gen --name-override my-key
```

Import an existing key file and map it to the current repository:

```sh
urv keys add --file ./my-key
```

Use an already stored key for the current repository:

```sh
urv keys add --key my-key
```

List configured repository-to-key mappings:

```sh
urv keys list
```

Keys are local machine state. They are not written to `.urv.vault.yaml` and should not be committed to Git.

For small teams, transfer key files only through a private channel that is already trusted for secrets. Do not paste URV keys into public issues, pull requests, chat rooms, shell history, or commit messages. After importing or mapping a key, run `urv status` to verify that the repository can see a valid mapped key.

## Generated Files

URV uses these files in the repository:

- `.urv.yaml` stores repository configuration.
- `.urv.vault.yaml` stores encrypted vault data and hashes of files included in the last encryption run.

URV also uses these local user files:

- `~/.config/urv/keys/<key-name>` stores encryption keys.
- `~/.config/urv/mapping.yaml` maps repository paths to key names.

## Development

Build the binary:

```sh
make build
```

Run tests:

```sh
make test
```

Download and tidy dependencies:

```sh
make tidy
```

Format and vet the code:

```sh
make lint
```

## Current Limitations

- The current vault format uses zip archive data encrypted with AES-GCM.
- Key files are stored only on the local machine.
- `urv decrypt` writes files back into the working tree and can replace existing file contents.
- Git hooks are part of the broader project idea but are not implemented yet.
