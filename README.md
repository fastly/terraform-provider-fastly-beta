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
  domain {
    name = "www.example.com"
  }

  backend {
    name    = "origin"
    address = "origin.example.com"
    port    = 443
  }
}
```

The automatic compatibility family owns nested configuration and performs
automatic version lifecycle handling.

Compatibility service resources:

- `fastly_service_cdn_auto`
- `fastly_service_compute_auto`

### Explicit/default resources still under development

All explicit/default resources are still under active development and are **not ready for
use**. Use the automatic compatibility family (`fastly_service_cdn_auto` /
`fastly_service_compute_auto`) instead until these reach parity:

- `fastly_service_cdn`
- `fastly_service_compute`
- `fastly_service_domain`
- `fastly_service_backend`
- `fastly_service_vcl`
- `fastly_service_vcl_snippet`
- `fastly_service_dynamic_vcl_snippet`
- `fastly_service_dynamic_snippet_content`
- `fastly_service_condition`
- `fastly_service_cdn_acl`
- `fastly_service_cdn_acl_entries`
- `fastly_service_resource_link`
- `fastly_service_dictionary_items`
- `fastly_service_logging_bigquery`
- `fastly_service_logging_blobstorage`
- `fastly_service_logging_datadog`
- `fastly_service_logging_gcs`
- `fastly_service_logging_https`
- `fastly_service_logging_newrelic`
- `fastly_service_logging_newrelicotlp`
- `fastly_service_logging_s3`
- `fastly_service_logging_splunk`
- `fastly_service_logging_sumologic`
- `fastly_service_logging_syslog`

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
