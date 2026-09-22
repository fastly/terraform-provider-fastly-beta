# Fastly Terraform Provider - Beta

This repository contains the source code for the `fastly-beta`
Terraform provider, a new implementation of the existing `fastly`
Terraform provider. The provider is built on the HashiCorp Terraform
plugin framework unlike the previous provider which is built on the
Terraform Plugin SDK v2.

Fastly expects to deliver the first release of this new provider in
September of 2026, and expects to deliver the first non-beta release
before the end of 2026.

## Design overview

This provider offers a **dual-model design** with two separate resource
families:

- an **automatic compatibility resource family** for users who want
  current-provider-style nested resources with automatic clone and activation
  behavior
- an **explicit/default resource family** for users who want first-class
  versioned resources and explicit version lifecycle operations

The user chooses the model through the resource type.

The explicit/default resource family uses the clean resource names. The automatic
compatibility resource family uses the `_auto` suffix.

## Resource families

### Automatic compatibility family

```hcl
resource "fastly_service_cdn_auto" "example" {
  backend {
    name    = "origin"
    address = "origin.example.com"
    port    = 443
  }
}

resource "fastly_domain" "example" {
  fqdn       = "www.example.com"
  service_id = fastly_service_cdn_auto.example.id
}
```

The automatic compatibility family owns nested configuration and performs
automatic version lifecycle handling.

Compatibility service resources:

- `fastly_service_cdn_auto`
- `fastly_service_compute_auto`

### Explicit/default resources still under development

Some resources in this provider can't be used with the automatic compatibility family: they
belong to the explicit/default family, which is still under development and **not ready for
testing**. Use their automatic family equivalents instead.

Two of them are service resources, replaced one for one:

| Explicit/default resource | Use instead |
| --- | --- |
| `fastly_service_cdn` | `fastly_service_cdn_auto` |
| `fastly_service_compute` | `fastly_service_compute_auto` |

The rest are configured on those two resources as nested blocks:

| Explicit/default resource | Use instead | Available on |
| --- | --- | --- |
| `fastly_service_backend` | `backend` block | both |
| `fastly_service_cdn_acl` | `acl` block | CDN |
| `fastly_service_condition` | `condition` block | CDN |
| `fastly_service_custom_vcl` | `custom_vcl` block | CDN |
| `fastly_service_domain` | `domain` block \* | both |
| `fastly_service_dynamic_vcl_snippet` | `dynamic_snippet` block | CDN |
| `fastly_service_logging_*` | `logging_*` blocks | both |
| `fastly_service_ratelimiter` | `rate_limiter` block | CDN |
| `fastly_service_resource_link` | `resource_link` block | Compute |
| `fastly_service_vcl_snippet` | `snippet` block | CDN |

\* Both `fastly_service_domain` and the `domain` block configure classic domains, which are only
available on accounts created before September 16, 2025. The versionless `fastly_domain` resource
is recommended for all accounts, and works with either service resource.

Every other resource is available for testing today.

### Explicit/default family

```hcl
resource "fastly_service_cdn" "example" {
  name = "example"
}

resource "fastly_service_domain" "example" {
  service_id = fastly_service_cdn.example.id
  version    = var.service_version
  name       = "www.example.com"
}
```

The explicit/default family uses first-class resources. Version cloning,
activation, and staging are handled explicitly by the caller or workflow
automation.

Explicit/default service resources:

- `fastly_service_cdn`
- `fastly_service_compute`

Shared versioned resources include:

- `fastly_service_domain`
- `fastly_service_backend`
- `fastly_service_dictionary`

## Design documents

For the full design, see:

- [Dual-Model Provider Design](docs/dual-model-provider-design.md)
- [Terraform Query Support](docs/terraform-query.md)

## Examples

The examples compare the two resource families by managing the same basic Fastly
service configuration:

- `examples/orchestration-cdn-auto`
- `examples/orchestration-explicit-actions`
- `examples/orchestration-explicit-cli`
- `examples/orchestration-explicit-latest-cli`
- `examples/compute-explicit-package`
- `examples/terraform-query-import`

## Important design rule

A Fastly service should be managed through **one resource family only**.

Do not manage the same Fastly service with both:

- an automatic compatibility service resource such as `fastly_service_cdn_auto`
  or `fastly_service_compute_auto`
- explicit/default resources such as `fastly_service_cdn`,
  `fastly_service_compute`, `fastly_service_domain`, or
  `fastly_service_backend`
