---
page_title: "Fastly Provider (beta)"
description: |-
  Manage Fastly CDN and Compute services, and the domains, storage, TLS, and Next-Gen WAF resources around them.
---

# Fastly Provider (beta)

This provider is a ground-up rewrite of Fastly's Terraform provider on
HashiCorp's Plugin Framework. It manages Fastly CDN and Compute services,
along with the supporting domains, storage, TLS certificates, and Next-Gen WAF
configuration. During the beta program it's distributed as
`fastly/fastly-beta`. When the beta ends, it will ship as a new major version
of `fastly/fastly`.

## Guides

This provider isn't a drop-in replacement for `fastly/fastly`. Resource names
and HCL syntax have changed, so an existing configuration needs translation.
The navigation also lists every resource in the provider, including some that
aren't ready for testing. Read these first:

- [Beta Testing Guide](guides/beta_testing.md): how to use this provider
  during the beta testing period, and how to send us feedback.
- [HCL Syntax Changes](guides/hcl_syntax_changes.md): what changed from the
  legacy provider, and how to translate a configuration.

-> **Note:** The `-beta` suffix shown in this provider's name and navigation refers only to its Registry distribution during the beta program. Resource and data source type names are unaffected and keep their standard `fastly_` prefix (e.g. `fastly_service_cdn_auto`), with no `-beta` in the name.

## Example Usage

```terraform
# Terraform 0.13+ requires providers to be declared in a "required_providers" block
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly-beta"
      version = ">= 0.2.2"
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
