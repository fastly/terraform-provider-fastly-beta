---
page_title: "fastly_service_cdn_acl_entries Resource - fastly"
subcategory: ""
description: |-
  Manages ACL entries for a Fastly service ACL. Terraform manages only the entries declared in the entry blocks and leaves other ACL entries unchanged.
---

# fastly_service_cdn_acl_entries (Resource)

Manages ACL entries for a Fastly service ACL. Terraform manages only the entries
declared in the `entry` blocks and leaves other ACL entries unchanged.

Each `entry` block declared here is owned by this Terraform resource. Terraform
creates missing managed entries, updates managed entries that drift, and deletes
a managed entry when its `entry` block is removed from configuration.

Entries that are not declared in this resource are left unchanged. This allows
an ACL to contain Terraform-managed entries alongside entries managed by other
systems, the Fastly API, the Fastly CLI, or the Fastly control panel.

This resource uses the Fastly batch operations API to efficiently handle large
numbers of entries.

## Example Usage

```terraform
resource "fastly_service_cdn" "example" {
  name = "example-service"

  domain {
    name = "example.com"
  }

  backend {
    address = "http-me.fastly.dev"
    name    = "backend"
  }
}

resource "fastly_service_cdn_acl" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "example_acl"
}

resource "fastly_service_cdn_acl_entries" "example" {
  service_id = fastly_service_cdn.example.id
  acl_id     = fastly_service_cdn_acl.example.acl_id

  entry {
    ip      = "192.0.2.1"
    subnet  = 32
    negated = false
    comment = "Single IP address"
  }

  entry {
    ip      = "198.51.100.0"
    subnet  = 24
    negated = false
    comment = "IP range"
  }

  entry {
    ip      = "203.0.113.10"
    subnet  = 32
    negated = true
    comment = "Negated entry - blocks this IP"
  }
}
```

Removing an `entry` block deletes that entry from the ACL:

```terraform
resource "fastly_service_cdn_acl_entries" "example" {
  service_id = fastly_service_cdn.example.id
  acl_id     = fastly_service_cdn_acl.example.acl_id

  entry {
    ip      = "192.0.2.1"
    subnet  = 32
    negated = false
    comment = "Single IP address"
  }
}
```

In this example Terraform deletes the previously managed `198.51.100.0/24` and
`203.0.113.10/32` entries, but does not delete unrelated ACL entries that were
never managed by this resource.

## Schema

### Required

- `acl_id` (String) The ID of the ACL that the items belong to.
- `service_id` (String) The ID of the Service that the ACL belongs to.

### Optional

- `entry` (Block Set) ACL entries owned by this resource. Entries not declared here are left unchanged. (see [below for nested schema](#nestedblock--entry))

### Read-Only

- `id` (String) Alphanumeric string identifying the resource. Format: `service_id/acl_id`.

<a id="nestedblock--entry"></a>
### Nested Schema for `entry`

Required:

- `ip` (String) An IP address that is the focus for the ACL.

Optional:

- `comment` (String) A personal freeform descriptive note.
- `negated` (Boolean) A boolean that will negate the match if true.
- `subnet` (Number) Number of bits for the subnet mask applied to the IP address (0-32 for IPv4, 0-128 for IPv6).

Read-Only:

- `id` (String) The unique ID of the entry.

## Import

The import ID format is `service_id/acl_id`.

```shell
terraform import fastly_service_cdn_acl_entries.example SERVICE_ID/ACL_ID
```

Example:

```shell
terraform import fastly_service_cdn_acl_entries.example SU1Z0isxPaozGVKXdv0eY/7Lsb7Y8w6St2eqwFgqzc
```

Import adopts every entry currently present in the ACL into this resource's
Terraform state. Before applying configuration after an import, ensure that
your `entry` blocks cover every imported entry you want Terraform to continue
managing. Run `terraform show` after importing to see the entries that were
read in, and copy them into your configuration before applying — removing an
imported entry from your configuration tells Terraform to delete it.
