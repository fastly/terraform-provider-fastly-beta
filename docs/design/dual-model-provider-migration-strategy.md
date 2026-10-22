# Dual-Model Provider Migration Strategy

## Summary

Fastly should provide an **official open-source migration CLI** for existing Terraform users moving from the legacy provider to the new dual-model provider.

A deterministic CLI should be the primary migration path rather than AI-assisted migration. Cloudflare and Microsoft both provide official migration tooling for significant Terraform provider transitions, establishing a strong precedent for vendors to provide supported tooling when customers must migrate existing Terraform configuration and state.

Fastly should follow the same approach. We cannot assume that all users are comfortable using AI for infrastructure migrations, have access to approved AI tooling, or are permitted by their organization's security and compliance policies to provide Terraform configuration to an AI system. A CLI also gives users a deterministic, reviewable, scriptable, and CI-compatible migration path that Fastly can document, test, version, and support.

AI-assisted migration could still be evaluated later as an optional experience built around the same migration rules, but it should not be required for a successful provider migration.

The migration CLI will only support the **Automatic (`_auto`) family** because it most closely preserves the legacy provider workflow and is expected to be the migration path for most existing users.

The CLI should:

- transform existing Fastly HCL to the `_auto` resource model
- preserve variables, expressions, modules, references, and code structure where possible
- use Terraform and Terraform Plugin Framework migration mechanisms for state transitions
- never directly rewrite raw Terraform state
- provide a preview/dry-run before migration
- validate the result with Terraform

Recommended binary:

```text
fastly-tf-migrate
```

The migration CLI should live in this repository and be built and distributed as a separate binary. Keeping the CLI and provider together allows migration rules and provider changes to remain synchronized and land in the same pull request.

---

## Background

The dual-model provider introduces two ways to manage Fastly services:

- **Automatic (`_auto`) family** — preserves provider-managed cloning, validation, and activation.
- **Explicit family** — exposes service configuration as first-class resources and leaves service-version lifecycle to the caller.

The migration CLI will support only:

```text
fastly_service_vcl     → fastly_service_cdn_auto
fastly_service_compute → fastly_service_compute_auto
```

The Explicit family is out of scope for this tool because migration requires breaking one legacy service resource into many independent Terraform resources, making both HCL and state migration fundamentally different.

There is one split-resource case that the `_auto` migration must handle: legacy `product_enablement` configuration is nested inside `fastly_service_vcl` and `fastly_service_compute`, while the dual-model provider represents each product as its own versionless `fastly_service_product_*` resource. Handling this targeted split is part of migrating the legacy service to `_auto`; it does not extend the CLI to the Explicit resource family.

---

## Why `terraform import` and `terraform query` are not enough

Terraform already provides ways to adopt existing Fastly resources and generate new configuration for both resource families.

For the Automatic (`_auto`) family, an existing service can be declared with an `import` block:

```hcl
import {
  to = fastly_service_cdn_auto.example
  id = "<service-id>"
}
```

Terraform can then generate a corresponding resource configuration:

```bash
terraform plan -generate-config-out=generated.tf
```

For the Explicit family, `terraform query` can discover existing Fastly resources and generate configuration:

```bash
terraform query -generate-config-out=generated.tf
```

Both workflows are useful for bringing previously unmanaged Fastly resources under Terraform management. However, they generate a new representation of the live infrastructure rather than migrate the user's existing Terraform configuration.

Generated configuration does not preserve the existing user's:

- variables and locals
- expressions
- modules
- resource labels
- `for_each` / `count`
- file layout
- references
- coding style

For existing Fastly Terraform users, the existing HCL should therefore remain the source of truth. The migration CLI should transform that configuration in place rather than replace it with newly generated HCL.

---

## Related migration approaches

### Cloudflare

Cloudflare provides the official `tf-migrate` CLI for its ground-up v5 provider rewrite.

Its two-phase migration support for resources that split into multiple target resources is particularly relevant to Fastly's legacy `product_enablement` block, which must become standalone product resources in the dual-model provider.

Relevant ideas for Fastly include:

- deterministic HCL transformation
- dry-run support
- migration warnings
- Terraform-native `moved`, `removed`, and `import` workflows
- provider-native state migration instead of direct raw-state rewriting
- a two-phase migration model for resources that must be split into multiple target resources

### Microsoft

Microsoft provides dedicated migration tooling such as `aztfmigrate` for migrations between AzureRM and AzAPI.

Relevant ideas include:

- an explicit migration planning step
- deterministic HCL transformation
- unsupported-case reporting
- Terraform-based state transitions
- post-migration `terraform plan` validation

These approaches support providing an official Fastly migration CLI rather than requiring customers to manually rewrite a large Terraform codebase or depend on AI tooling for a critical state migration.

---

## Proposed Fastly strategy

The migration CLI should own **configuration migration**, while Terraform and the new provider own **state migration**.

The CLI should not parse and rewrite `.tfstate` directly.

Instead, Fastly should use the migration mechanisms provided by Terraform Core and the Terraform Plugin Framework, including:

- `ResourceWithUpgradeState` where an existing resource state schema needs conversion
- `ResourceWithMoveState` where state moves between different resource types
- Terraform `moved` blocks
- Terraform `removed` blocks
- Terraform `import` blocks

Because the legacy resource types are renamed, `UpgradeState` alone is not sufficient. The exact combination of `MoveState`, state upgrade logic, and Terraform migration blocks should be proven before the CLI implementation is finalized.

### Terraform-native examples

A `moved` block records that an existing resource address now maps to a new address:

```hcl
moved {
  from = fastly_service_vcl.example
  to   = fastly_service_cdn_auto.example
}
```

For a move between different resource types such as this, the target provider must support the required state conversion, for example through Framework `ResourceWithMoveState`.

A `removed` block can release the old resource from Terraform state without destroying the Fastly service:

```hcl
removed {
  from = fastly_service_vcl.example

  lifecycle {
    destroy = false
  }
}
```

An `import` block can then associate an existing Fastly service with the new `_auto` resource:

```hcl
import {
  to = fastly_service_cdn_auto.example
  id = "<service-id>"
}
```

`removed` + `import` should be a fallback where a direct state move cannot be implemented safely.

Terraform remains responsible for backend locking, state serialization, remote state writes, and committing the migration.

### Product enablement

Product enablement cannot be migrated with the service's `moved` block alone. The legacy `product_enablement` block is nested state and has no independent Terraform resource address, while the target model contains one or more standalone `fastly_service_product_*` resources.

The migration CLI should therefore support a phased migration for services that manage product enablement:

1. migrate the legacy service to its `_auto` resource while removing the nested product-enablement data from service state without disabling or reconfiguring the live products
2. generate standalone product resource blocks from the existing HCL and import the corresponding service-scoped product state into those resources

The proof-of-concept must validate the exact sequencing and confirm that product enablement is not disabled and re-enabled as a side effect of migration. If Terraform can safely perform both parts in one operation, the CLI may collapse the phases; otherwise it should use an explicit two-phase workflow.

---

## Initial migration workflow

The first CLI should provide a small, predictable workflow:

```bash
fastly-tf-migrate plan
fastly-tf-migrate migrate --dry-run
fastly-tf-migrate migrate
```

At a high level it should:

1. identify legacy Fastly resources and supported `_auto` mappings
2. back up or require a backup of the current Terraform state
3. transform supported Fastly HCL while preserving expressions and references
4. add or coordinate the required Terraform-native migration mechanism, including phased product-enablement migration when needed
5. run or instruct the user to run Terraform validation
6. require a final `terraform plan` with no unexpected create, destroy, or replace actions

The CLI should never directly modify raw state and should not silently run a destructive Terraform operation.

---

## First engineering step

Before building the full migration CLI, Fastly should prove the state migration path with the two service moves and the product-enablement split:

```text
fastly_service_vcl                     -> fastly_service_cdn_auto
fastly_service_compute                 -> fastly_service_compute_auto
legacy service with product_enablement -> _auto service + fastly_service_product_* resources
```

The proof-of-concept should determine:

- whether Framework `ResourceWithMoveState` can safely handle each resource rename
- whether `ResourceWithUpgradeState` is also required for schema/type differences
- whether `moved` blocks are sufficient when paired with provider move-state support
- when `removed` + `import` is required instead
- whether the resulting configuration and state produce a no-op Terraform plan
- whether nested product-enablement state can be separated from the service without changing the live product configuration
- whether the generated standalone product resources can import the existing product state and converge without enabling, disabling, or reconfiguring products

---

## Decision requested

The team should agree on the following before implementation begins:

1. Fastly will provide an official migration CLI.
2. The CLI will support only legacy -> `_auto` migration.
3. The CLI will transform existing HCL rather than regenerate configuration.
4. The CLI will not directly rewrite raw Terraform state.
5. Terraform Plugin Framework and Terraform-native migration mechanisms will be the preferred state-migration path.
6. A focused state-migration proof-of-concept will be completed before the CLI architecture is finalized.
7. Legacy `product_enablement` will be migrated as a targeted split into standalone versionless product resources as part of the `_auto` migration.
8. Explicit-family migration is out of scope for this CLI and would require a separate migration strategy.
