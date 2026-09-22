# Terraform query support

## Purpose

This document explains how to use `terraform query` with the dual-model Fastly
Terraform provider.

`terraform query` is supported for the **explicit/default first-class resource
family** only. It is intended for read-only discovery and generated
configuration.

It is not used for the automatic compatibility resource family.

---

## Supported resource family

`terraform query` is supported for every explicit/default resource that has a
corresponding list resource registered in `provider.go`'s `ListResources()`.
As of this writing, that is:

Top-level services:

- `fastly_service_cdn`
- `fastly_service_compute`

Versioned child resources:

- `fastly_service_backend`
- `fastly_service_domain`
- `fastly_service_dictionary`
- `fastly_service_settings`
- `fastly_service_ratelimiter`
- `fastly_service_cdn_acl`
- `fastly_service_cdn_acl_entries`
- `fastly_service_condition`
- `fastly_service_custom_vcl`

Logging endpoints (one per provider):

- `fastly_service_logging_bigquery`
- `fastly_service_logging_blobstorage`
- `fastly_service_logging_cloudfiles`
- `fastly_service_logging_datadog`
- `fastly_service_logging_digitalocean`
- `fastly_service_logging_elasticsearch`
- `fastly_service_logging_ftp`
- `fastly_service_logging_gcs`
- `fastly_service_logging_googlepubsub`
- `fastly_service_logging_grafanacloudlogs`
- `fastly_service_logging_heroku`
- `fastly_service_logging_honeycomb`
- `fastly_service_logging_https`
- `fastly_service_logging_kafka`
- `fastly_service_logging_kinesis`
- `fastly_service_logging_loggly`
- `fastly_service_logging_logshuttle`
- `fastly_service_logging_newrelic`
- `fastly_service_logging_newrelicotlp`
- `fastly_service_logging_openstack`
- `fastly_service_logging_papertrail`
- `fastly_service_logging_s3`
- `fastly_service_logging_scalyr`
- `fastly_service_logging_sftp`
- `fastly_service_logging_splunk`
- `fastly_service_logging_sumologic`
- `fastly_service_logging_syslog`

These resources are first-class Terraform resources, so they can be discovered
independently and generated as separate resource blocks.

This list grows as new explicit/default resources gain a `list.go` (see
"Adding a new resource" in the top-level `CLAUDE.md`). If a resource you
expect to query is missing, check whether its package has a `list.go` and
whether it's registered in `ListResources()` before assuming query doesn't
support it.

Example explicit/default configuration:

```hcl
resource "fastly_service_cdn" "example" {
  name = "example"
}

resource "fastly_service_domain" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "www.example.com"
}

resource "fastly_service_backend" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "origin"
  address    = "origin.example.com"
  port       = 443
}
```

---

## Automatic compatibility resources use import instead

`terraform query` is not supported for automatic compatibility resources:

- `fastly_service_cdn_auto`
- `fastly_service_compute_auto`

Automatic compatibility resources use nested configuration:

```hcl
resource "fastly_service_cdn_auto" "example" {
  backend {
    name    = "origin"
    address = "origin.example.com"
    port    = 443
  }
}
```

Because automatic compatibility resources own nested configuration as one
aggregate resource, generated configuration for this resource family should come
from Terraform import, not from query:

```bash
terraform import fastly_service_cdn_auto.example <service_id>
```

---

## Version selection

`terraform query` is read-only. It must not clone, activate, stage, or otherwise
mutate Fastly service versions.

For each Fastly service, query selects the version to read from using this order:

1. If the service has an active version, read from the active version.
2. If the service has no active version, read from the latest service version.

A Fastly service is expected to have at least one version because service
creation creates version `1`.

Generated explicit/default resources include the version number that was read.

If the generated version is active or locked, the generated configuration is
still useful for discovery and import. Before making changes with the
explicit/default resource family, clone or select a writable version and update
the generated resources to target that writable version.

### Query flow

```text
terraform query for explicit/default resources
  |
  v
inspect service S
  |
  | active version exists?
  |---- yes ---> read first-class versioned resources from active version
  |
  |---- no ----> read first-class versioned resources from latest version
  v
generate first-class resources with version pinned to the version read
  |
  v
read-only discovery; no version lifecycle operations
```

---

## Basic usage

Create a normal Terraform provider configuration.

```hcl
# main.tf
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly-beta"
    }
  }
}

provider "fastly" {}
```

Create a query configuration file:

```hcl
# fastly.tfquery.hcl
list "fastly_service_cdn" "all" {
  provider = fastly
}

list "fastly_service_compute" "all" {
  provider = fastly
}

list "fastly_service_domain" "all" {
  provider = fastly
}

list "fastly_service_backend" "all" {
  provider = fastly
}

list "fastly_service_settings" "all" {
  provider = fastly
}

list "fastly_service_ratelimiter" "all" {
  provider = fastly
}

list "fastly_service_dictionary" "all" {
  provider = fastly
}

list "fastly_service_logging_s3" "all" {
  provider = fastly
}
```

This is a representative subset; add a `list` block for any resource named
under "Supported resource family" above, following the same pattern.

The `provider` argument is required in every `list` block. It tells Terraform
which provider configuration to use.

Then run:

```bash
export FASTLY_API_TOKEN=<token>

terraform query
```

To generate Terraform configuration:

```bash
terraform query -generate-config-out=generated.tf
```

To produce machine-readable output:

```bash
terraform query -json
```

If `generated.tf` already exists, remove it before running
`terraform query -generate-config-out=generated.tf` again.

---

## Local development usage

When testing a locally built provider with development overrides, first build the
provider and export the generated Terraform CLI config file:

```bash
make build
export TF_CLI_CONFIG_FILE=/path/to/repo/bin/developer_overrides.tfrc
export FASTLY_API_TOKEN=<token>
```

Then run query from an example or test configuration directory:

```bash
terraform query
terraform query -generate-config-out=generated.tf
```

---

## Generated configuration

`terraform query -generate-config-out=generated.tf` produces Terraform resource
blocks and matching import blocks.

Every resource in the supported family implements Terraform's
[resource identity](https://developer.hashicorp.com/terraform/plugin/framework/resources/identity)
(`resource.ResourceWithIdentity`), so generated import blocks use an
`identity` block rather than a plain `id` string. Identity deliberately
excludes `version`: a resource identity can't safely contain a mutable field,
since Terraform treats an identity change as a different resource and forces
a replace. Import by identity is version-less by design — it always resolves
to the active version, or the latest version when no active version exists,
the same way query itself picks one (see "Version selection" below).

Example service output (identity is just the service ID):

```hcl
resource "fastly_service_cdn" "my_service" {
  provider = fastly
  name     = "My Service"
}

import {
  to       = fastly_service_cdn.my_service
  provider = fastly
  identity = {
    id = "abc123"
  }
}
```

Example domain output (identity is `service_id` + `name`):

```hcl
resource "fastly_service_domain" "www_example_com" {
  provider   = fastly
  service_id = "abc123"
  version    = 7
  name       = "www.example.com"
}

import {
  to       = fastly_service_domain.www_example_com
  provider = fastly
  identity = {
    service_id = "abc123"
    name       = "www.example.com"
  }
}
```

Example backend output (same `service_id` + `name` shape as domain):

```hcl
resource "fastly_service_backend" "origin" {
  provider   = fastly
  service_id = "abc123"
  version    = 7
  name       = "origin"
  address    = "origin.example.com"
  port       = 443
}

import {
  to       = fastly_service_backend.origin
  provider = fastly
  identity = {
    service_id = "abc123"
    name       = "origin"
  }
}
```

Most versioned child resources follow the `service_id` + `name` shape above.
Two exceptions:

- `fastly_service_settings` has no name component (it's a singleton per
  service version), so its identity is `service_id` alone.
- `fastly_service_cdn_acl_entries` isn't pinned to a version at all (ACL
  entries are read/written against the ACL directly, not a service version),
  so its identity is `service_id` + `acl_id`.

The legacy composite-string `terraform import <resource> service_id/version/name`
format (and `service_id/version`, `service_id/acl_id` for the two exceptions
above) still works for manual imports — it's what each resource's
`ImportState` falls back to when identity isn't present on the import block.
Unlike identity-based import, that path *is* version-pinned: it reads exactly
the version named in the string. Prefer identity-based import (what query
generates) unless you specifically need to pin an import to a non-active,
non-latest version.

After generating configuration:

1. review the generated resources
2. copy the generated blocks into the desired Terraform configuration
3. run `terraform apply` to import the discovered resources into state
4. remove the import blocks after a successful import, or keep them as a
   historical record

---

## Editing generated explicit/default resources

Generated explicit/default resources are pinned to the version that query read.

If that version is active or locked, do not edit and apply those resources
directly. First clone or select a writable version, then update the generated
`version` arguments to target that writable version.

For example:

```hcl
resource "fastly_service_domain" "www_example_com" {
  service_id = fastly_service_cdn.my_service.id
  version    = var.service_version
  name       = "www.example.com"
}
```

The explicit/default resource family is intended for caller-managed lifecycle
workflows. Terraform writes to the version you specify; it does not choose,
clone, or activate versions during normal resource CRUD.

---

## Summary

```text
*_auto service resources
  -> automatic compatibility nested resource family
  -> generated configuration through terraform import
  -> automatic clone and activation during CRUD

clean first-class resources
  -> explicit/default resource family
  -> generated configuration through terraform query
  -> query reads active version, or latest version if no active version exists
  -> query never clones or activates
  -> lifecycle is caller-managed
```
