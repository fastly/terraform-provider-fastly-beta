---
page_title: "fastly_service_resource_link Resource - fastly"
subcategory: ""
description: |-
  Links a shared resource (such as a KV Store or Config Store) to a Fastly service version, making it accessible from Compute code. Writes directly to the specified writable service version.
---

# fastly_service_resource_link (Resource)

Links a shared resource (such as a KV Store or Config Store) to a Fastly service version, making it accessible from Compute code. Writes directly to the specified writable service version.

## Example Usage

```terraform
resource "fastly_service_compute" "app" {
  name    = "example-compute-service"
  comment = "Managed by Terraform"
}

resource "fastly_kvstore" "store" {
  name = "example-kv-store"
}

# Makes the KV Store available to Wasm code under the alias "store".
resource "fastly_service_resource_link" "store" {
  service_id  = fastly_service_compute.app.id
  version     = 1
  name        = "store"
  resource_id = fastly_kvstore.store.id
}
```

## Schema

### Required

- `name` (String) The name the service will use to open the linked resource from Compute code (e.g. a KV Store or Config Store SDK lookup). This is an alias and does not need to match the name of the underlying resource.
- `resource_id` (String) The ID of the shared resource to link (e.g. the ID of a KV Store or Config Store).
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Read-Only

- `id` (String) Terraform resource identifier.
- `link_id` (String) An alphanumeric string identifying this resource link.

## Import

`fastly_service_resource_link` has a stable Framework identity of
`service_id + resource_id`. Unlike most versioned resources, `name` is not part
of the identity because a link's alias can be renamed in place, while pointing
it at a different `resource_id` replaces the link. The `version` argument is
not part of the identity either, since explicit resources can move the same
link from one service version to another.

For import-from-scratch with the Terraform CLI, include the service version and
the link's alias name in the import ID:

```shell
terraform import fastly_service_resource_link.store SERVICE_ID/VERSION/NAME
```

Example:

```shell
terraform import fastly_service_resource_link.store SU1Z0isxPaozGVKXdv0eY/3/store
```

You can also use Terraform's identity-based import flow with the stable identity
fields. The resource configuration must still provide the `version` argument
because the provider needs a service version to read the link from Fastly:

```terraform
resource "fastly_service_resource_link" "store" {
  service_id  = "SU1Z0isxPaozGVKXdv0eY"
  version     = 3
  name        = "store"
  resource_id = "7d5b2e1f9a3c4e8b"
}

import {
  to = fastly_service_resource_link.store

  identity = {
    service_id  = "SU1Z0isxPaozGVKXdv0eY"
    resource_id = "7d5b2e1f9a3c4e8b"
  }
}
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version. To move a link to a newer draft version, clone a version that
already contains the link and update `version`; the provider adopts the cloned
link in place.
