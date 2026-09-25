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

`terraform query` is supported for these explicit/default resources:

- `fastly_service_cdn`
- `fastly_service_compute`
- `fastly_service_domain`
- `fastly_service_backend`
- `fastly_service_settings`
- `fastly_service_ratelimiter`
- `fastly_service_dictionary`
- `fastly_service_cache_setting`
- `fastly_service_gzip`
- `fastly_service_cdn_acl`
- `fastly_service_cdn_acl_entries`
- `fastly_service_condition`
- `fastly_service_custom_vcl`
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

list "fastly_service_cache_setting" "all" {
  provider = fastly
}

list "fastly_service_gzip" "all" {
  provider = fastly
}

list "fastly_service_cdn_acl" "all" {
  provider = fastly
}

list "fastly_service_cdn_acl_entries" "all" {
  provider = fastly
}

list "fastly_service_condition" "all" {
  provider = fastly
}

list "fastly_service_custom_vcl" "all" {
  provider = fastly
}

list "fastly_service_logging_bigquery" "all" {
  provider = fastly
}

list "fastly_service_logging_blobstorage" "all" {
  provider = fastly
}

list "fastly_service_logging_cloudfiles" "all" {
  provider = fastly
}

list "fastly_service_logging_datadog" "all" {
  provider = fastly
}

list "fastly_service_logging_digitalocean" "all" {
  provider = fastly
}

list "fastly_service_logging_elasticsearch" "all" {
  provider = fastly
}

list "fastly_service_logging_ftp" "all" {
  provider = fastly
}

list "fastly_service_logging_gcs" "all" {
  provider = fastly
}

list "fastly_service_logging_googlepubsub" "all" {
  provider = fastly
}

list "fastly_service_logging_grafanacloudlogs" "all" {
  provider = fastly
}

list "fastly_service_logging_heroku" "all" {
  provider = fastly
}

list "fastly_service_logging_honeycomb" "all" {
  provider = fastly
}

list "fastly_service_logging_https" "all" {
  provider = fastly
}

list "fastly_service_logging_kafka" "all" {
  provider = fastly
}

list "fastly_service_logging_kinesis" "all" {
  provider = fastly
}

list "fastly_service_logging_loggly" "all" {
  provider = fastly
}

list "fastly_service_logging_logshuttle" "all" {
  provider = fastly
}

list "fastly_service_logging_newrelic" "all" {
  provider = fastly
}

list "fastly_service_logging_newrelicotlp" "all" {
  provider = fastly
}

list "fastly_service_logging_openstack" "all" {
  provider = fastly
}

list "fastly_service_logging_papertrail" "all" {
  provider = fastly
}

list "fastly_service_logging_s3" "all" {
  provider = fastly
}

list "fastly_service_logging_scalyr" "all" {
  provider = fastly
}

list "fastly_service_logging_sftp" "all" {
  provider = fastly
}

list "fastly_service_logging_splunk" "all" {
  provider = fastly
}

list "fastly_service_logging_sumologic" "all" {
  provider = fastly
}

list "fastly_service_logging_syslog" "all" {
  provider = fastly
}
```

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

Example service output:

```hcl
resource "fastly_service_cdn" "my_service" {
  provider = fastly
  name     = "My Service"
}

import {
  to       = fastly_service_cdn.my_service
  provider = fastly
  identity = {
    service_id = "abc123"
  }
}
```

Example domain output:

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
    version    = 7
    name       = "www.example.com"
  }
}
```

Example backend output:

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
    version    = 7
    name       = "origin"
  }
}
```

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
