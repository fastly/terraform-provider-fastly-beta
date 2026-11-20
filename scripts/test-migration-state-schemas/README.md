# Legacy service state schema compatibility

This suite covers additive schema changes without an SDKv2 service schema
version bump. It does **not** implement new HCL migration features.

## Candidate release matrix

| SDKv2 release | VCL (`schema_version = 1`) | Compute (`schema_version = 0`) |
| --- | --- | --- |
| 9.1.0 | Yes | Not claimed |
| 9.5.0 | Yes | Yes |
| 9.8.0 | Yes | Yes |
| 10.2.0 | Yes | Yes |
| 10.3.0 | Yes | Yes |

The original fixtures are retained. Current and intermediate fixtures are
reconstructed from the historical snapshots and source changes. The
published-schema suite must independently confirm those types; they are not
claimed to be fresh exports. The lockfile version range is not a substitute
for verifying the persisted state.

## 1. Credential-free regression tests

```sh
go test -count=1 -v ./internal/provider -run '^TestLegacyServiceMoveState'
make test-unit
make test-migrator
```

The protocol tests invoke the real Framework MoveResourceState RPC with
synthetic JSON state using each historical shape. They test populated and empty
nested blocks, absent/null fields, service ID and lifecycle flags, product
warnings, package local inputs and private state, invalid JSON shapes, and
unsupported IAM or source schema versions. CLI tests ensure IAM and FOS
configurations are rejected without changing HCL or state prerequisites.

## 2. Independently compare the published schemas

```sh
make test-migration-schemas
```

Requires the repository Go toolchain, Terraform, jq and registry connectivity,
but no Fastly token. Exports the exact releases listed above in isolated
temporary directories, verifies the selected provider versions, compares
every type and nesting mode, and runs the move RPC using those exported
shapes. The script does not create infrastructure.

## 3. Live state move verification

Use only an isolated Fastly test account (`FASTLY_API_TOKEN`). The live tests
create and destroy real services and require Terraform supporting cross-type
moves (1.8+), Go, jq, registry connectivity, and account permissions to create
and manage the appropriate service types. VCL also needs curl and cksum.
Compute needs package upload permission.

```sh
for version in 9.1.0 9.5.0 9.8.0 10.2.0 10.3.0; do
  LEGACY_PROVIDER_VERSION="$version" make test-migration-vcl || exit 1
done

for version in 9.5.0 9.8.0 10.2.0 10.3.0; do
  LEGACY_PROVIDER_VERSION="$version" ./scripts/test-migration-state-schemas/run-compute.sh || exit 1
done
```

The VCL suite uses the existing CLI-driven workflow with exact source
version selection. The Compute suite uses manually authored target HCL and
a `moved` block, so it does not depend on or duplicate CDTOOL-1867. Both
exercise real legacy state, reject infrastructure-changing migration
actions before apply, check identity and active version, audit service API
traffic, and require a final no-op. Compute also checks the filename and
package hash before and after apply.

### NGWAF

Every renamed or split legacy NGWAF resource type is unchanged at schema
version 0 from 9.1.0 through 10.3.0, so one 10.3.0 fixture per type in
`internal/ngwafmovestate/testdata` covers the matrix. The published-schema
suite above compares all 16 against each release.

```sh
for version in 9.1.0 9.5.0 9.8.0 10.2.0 10.3.0; do
  LEGACY_PROVIDER_VERSION="$version" ./scripts/test-migration-state-schemas/run-ngwaf.sh || exit 1
done
```

The NGWAF suite needs an account entitled to Next-Gen WAF. It creates one
workspace and one object for each of the 28 target types, including
account-wide lists, rules and a signal, then moves them with hand-written
target HCL and `moved` blocks (CDTOOL-1873 owns the CLI workflow). It
requires 28 state-only moves, no mutating NGWAF API requests, unchanged
object and workspace IDs, and a final no-op. Set `NGWAF_VIRTUAL_PATCH_ID`
if `CVE-2017-5638` isn't available to the test account.

Start with 10.3.0 for each service type, then run the older releases. Retained
work directories contain private Terraform state, plan and log files; do not
upload them unredacted. Cleanup attempts to destroy only test-owned resources.

## Scope and merge gate

Configured VCL backend IAM is rejected until target parity is implemented.
FOS HCL migration remains blocked; decoding populated FOS state is tested
separately. No new Compute CLI compatibility is enabled. Earlier Compute
releases are not claimed by this matrix. All unit, published-schema and live
matrix checks must pass before advertising the matrix as verified.
