---
page_title: "fastly Provider"
description: |-
  Manage Fastly CDN and Compute services, and the domains, storage, TLS and Next-Gen WAF resources around them.
---

# fastly Provider

The Fastly provider manages Fastly CDN and Compute services, along with the
domains, storage, TLS certificates and Next-Gen WAF configuration that support
them.

This is a ground-up rewrite of the Fastly Terraform provider, built on
HashiCorp's Plugin Framework. It is under active development and distributed
during the beta program as `fastly/fastly-beta`. The **Automatic** resource
family described below is ready to test today; telling us how it holds up
against a configuration you already run is what the beta is for.

-> **Note:** The `-beta` suffix shown in this provider's name and navigation refers only to its Registry distribution during the beta program. Resource and data source type names are unaffected and keep their standard `fastly_` prefix (e.g. `fastly_service_cdn_auto`), with no `-beta` in the name.

## Guides

- [Beta Testing Guide](guides/beta_testing.md) — how to try this provider
  against a configuration you already run, and what to send back.
- [HCL Syntax Changes](guides/hcl_syntax_changes.md) — what changed from the
  legacy provider, and how to translate a configuration.

## Resource families

This provider offers two ways to manage a Fastly service. Resources from both
appear in the navigation, so it is worth knowing which is which.

The **Automatic** family — `fastly_service_cdn_auto` and
`fastly_service_compute_auto` — retains the legacy provider's [default
activation behavior](https://registry.terraform.io/providers/fastly/fastly/latest/docs/resources/service_vcl#activation-and-staging).
Service configuration lives in nested blocks, and the provider clones,
validates and activates a service version for you during `terraform apply`.
**This is the family to use today.**

The **Explicit** family replaces those nested blocks with first-class
resources, and hands version lifecycle operations back to you. It is **still in
development and not ready for use.** Its resources are visible in the
navigation — `fastly_service_backend`, `fastly_service_domain`,
`fastly_service_logging_*` and others — but each one maps to a nested block or
an `_auto` resource that does the same job today. The
[README](https://github.com/fastly/terraform-provider-fastly-beta#explicitdefault-resources-still-under-development)
lists the mapping.

Everything else is **versionless**: `fastly_domain`, `fastly_acl`, config and
KV stores, TLS, and Next-Gen WAF resources are not tied to a service version,
and work the same way with either family.

## Example Usage

```terraform
# Terraform 0.13+ requires providers to be declared in a "required_providers" block
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly-beta"
      version = ">= 0.1.3"
    }
  }
}

# Configure the Fastly Provider
provider "fastly" {
  api_token = "test"
}

# Create a Service
resource "fastly_service_cdn_auto" "myservice" {
  name = "myawesometestservice"

  backend {
    name    = "backend"
    address = "backend.example.com"
  }
}

# Domains are versionless and attach to the service by ID
resource "fastly_domain" "myservice" {
  fqdn       = "www.example.com"
  service_id = fastly_service_cdn_auto.myservice.id
}
```

## Schema

### Optional

- `api_token` (String, Sensitive) The Fastly API token. Can also be set via the FASTLY_API_TOKEN environment variable.
