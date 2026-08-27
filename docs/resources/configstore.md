---
page_title: "fastly_configstore Resource - fastly"
subcategory: ""
description: |-
  Provides a versionless Config Store for key-value data used by Compute services.
---

# fastly_configstore (Resource)

Provides a Config Store, a versionless container that stores key-value data which
can be read by Fastly Compute services during request processing.

Config Stores are account-level resources. They are not tied to a service
version, and updates to a Config Store are immediately visible to every linked
Compute service version.

Config Stores are available to Compute services only; they cannot be linked to
CDN (VCL-based) services. To make a Config Store available to Compute code, link
it from the Compute service with `resource_link`.

This resource manages the Config Store container itself. Manage key-value
items separately with the `fastly_configstore_items` resource.

## Example Usage

### Complete Compute Auto usage

```terraform
resource "fastly_configstore" "example" {
  name = "my_shared_configuration"
}

data "fastly_configstores" "all" {
  depends_on = [fastly_configstore.example]
}

locals {
  config_store_id = one([
    for store in data.fastly_configstores.all.stores :
    store.id if store.name == fastly_configstore.example.name
  ])
}

data "fastly_package_hash" "example" {
  filename = "package.tar.gz"
}

resource "fastly_service_compute_auto" "example" {
  name = "my_compute_service"

  domain {
    name = "www.example.com"
  }

  package {
    filename         = "package.tar.gz"
    source_code_hash = data.fastly_package_hash.example.hash
  }

  resource_link {
    name        = "config_store_link"
    resource_id = fastly_configstore.example.id
  }
}
```

The `data.fastly_configstores` block is optional in the example above; it shows
how to discover Config Stores by name. When linking a store created in the same
configuration, prefer `fastly_configstore.example.id` directly.

### Explicit/versioned Compute usage

For explicit/versioned Compute services, create the link with the standalone
`fastly_service_resource_link` resource:

```terraform
resource "fastly_configstore" "example" {
  name = "my_shared_configuration"
}

data "fastly_package_hash" "example" {
  filename = "package.tar.gz"
}

resource "fastly_service_compute" "example" {
  name = "my_compute_service"

  domain {
    name = "www.example.com"
  }

  package {
    filename         = "package.tar.gz"
    source_code_hash = data.fastly_package_hash.example.hash
  }
}

resource "fastly_service_resource_link" "config_store_link" {
  service_id  = fastly_service_compute.example.id
  version     = 1
  name        = "config_store_link"
  resource_id = fastly_configstore.example.id
}
```

The Config Store name is mutable. Renaming the store updates the existing Fastly
resource and does not change its ID. Resource-link names are independent and are
not renamed automatically when the Config Store name changes.

-> **Note:** A Config Store must be unlinked from every service before it can be
deleted. Delete the `resource_link` first, apply that change, and then delete the
Config Store. Deleting a Config Store also deletes the key-value pairs contained
in that store.

## Schema

### Required

- `name` (String) A unique name identifying the Config Store. The name can be updated in place without replacing the store.

### Read-Only

- `id` (String) An alphanumeric string identifying the Config Store.

## Import

Fastly Config Stores can be imported using their Config Store ID:

```shell
terraform import fastly_configstore.example <config_store_id>
```
