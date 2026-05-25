# universal-repo-vault

## Project Background and Idea

This project started from a problem I encountered while working on my small homelab. I set up a few small PCs as a k3s cluster to host lightweight automations and services. The plan is to manage everything in a single repository, using Kustomize for Kubernetes deployments and Ansible for machine setup.
There are already a few solutions that solve parts of this problem, and combining some of them would probably be enough. However, they all feel like too much hassle for my simple setup, and combining several tools increases the chance of making mistakes.
That is why I wanted to experiment a little and build a simple solution for storing, encrypting, and decrypting data in a repository.

## Goal

I want to create a simple CLI tool that can initialize its configuration, create a state lock file, update `.gitignore`, and eventually add Git hooks. After initialization, the tool should collect the contents of specified files in a repository, store them in an encrypted archive, and decrypt them on demand.

## Installation

This project is written in Go. Make sure you have Go installed before building it. The module currently targets Go `1.26.1`.

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

You can also run the project directly during development:

```sh
go run ./main.go
```

## Usage

Run commands from inside the Git repository that you want to manage.

Initialize URV in the repository:

```sh
./dist/urv init
```

This creates `.urv.yaml` if it does not already exist and updates repository state needed by the tool.

Edit `.urv.yaml` and list files or filename patterns that should be stored in the encrypted vault:

```yaml
secretfiles:
  - .env
patterns:
  - "*.secret.*"
```

Set the key name that URV should use:

```sh
export URV_KEY_NAME=my-repo-key
```

Generate a new encryption key:

```sh
./dist/urv keys gen
```

The key is saved under `~/.config/urv/keys/` using the name from `URV_KEY_NAME`.

Encrypt configured files into the vault:

```sh
./dist/urv encrypt
```

Decrypt files from the vault:

```sh
./dist/urv decrypt
```

## Generated Files

URV currently uses these files:

- `.urv.yaml` for repository configuration.
- `.urv.lock` for file state information generated during encryption.
- `.urv.vault.yaml` for encrypted vault data.
- `~/.config/urv/keys/<key-name>` for locally stored encryption keys.

## Authorship Note

All code in this project was written by hand. AI was used only to help generate and improve this documentation because writing clear documentation is not my strongest skill.
