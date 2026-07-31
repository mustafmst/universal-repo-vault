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
