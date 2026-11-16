# Compute service migration end-to-end test

Exercises `fastly-tf-migrate` and the provider's `MoveState` against a live
legacy `fastly_service_compute` (package, domain and backend).

## Flow

1. The migrator refuses `activate = false` and writes nothing.
2. Apply a legacy `fastly_service_compute` with the published legacy provider,
   then `apply -refresh-only` (persists schema v0 state).
3. `fastly-tf-migrate analyze`, `migrate --dry-run`, `migrate --write`.
4. Switch to the locally built provider (via `dev_overrides` for `fastly/fastly`)
   and plan: no create or delete (so no replace), and the service's
   `previous_address` is `fastly_service_compute.demo`.
5. Apply, then require a no-op `plan -detailed-exitcode`.
6. The Fastly service ID must be unchanged.
7. Destroy. On failure an emergency destroy and a name-prefix API sweep run.

## Running

```bash
export FASTLY_API_TOKEN=...
./scripts/test-migration-compute/run.sh
```

Requires `terraform`, `go` and `jq`. Set `LEGACY_PROVIDER_VERSION` to change the
legacy provider version (default `9.5.0`) and `KEEP_WORK_DIR=1` to keep the
work directory.
