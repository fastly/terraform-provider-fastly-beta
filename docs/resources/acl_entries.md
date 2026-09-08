---
page_title: "fastly_acl_entries Resource - fastly"
subcategory: ""
description: |-
  Manages CIDR-based allow/block entries within a Fastly ACL.
---

# fastly_acl_entries (Resource)

Manages CIDR-based allow/block entries within a Fastly ACL.

Each CIDR prefix declared in `entries` is owned by this Terraform resource.
Terraform creates missing managed prefixes, updates managed actions that drift,
and deletes a managed prefix when it is removed from the `entries` map.

ACL entries that are not declared in this resource are left unchanged. This
allows a Fastly ACL to contain Terraform-managed entries alongside entries
managed through the Fastly API, CLI, control panel, or another system.

## Example Usage

```terraform
resource "fastly_acl" "example" {
  name = "my_acl"
}

resource "fastly_acl_entries" "example" {
  acl_id = fastly_acl.example.id

  entries = {
    "192.0.2.0/24"    = "ALLOW"
    "198.51.100.0/24" = "BLOCK"
  }
}
```

Removing a prefix from `entries` deletes only that Terraform-managed prefix.
Unrelated ACL entries that were never managed by this resource remain unchanged.

## Schema

### Required

- `acl_id` (String) The ID of the ACL that the entries belong to.
- `entries` (Map of String) The ACL entries managed by Terraform, where keys are CIDR prefixes and values are actions (`ALLOW` or `BLOCK`). Entries not declared in this map are left unchanged.

### Read-Only

- `id` (String) Terraform resource identifier. Format: `acl_id/entries`.

## Import

Fastly ACL entries can be imported using the format `<acl_id>/entries`:

```shell
terraform import fastly_acl_entries.example <acl_id>/entries
```

Import adopts all entries currently present in the ACL into this resource's
Terraform state. Before applying configuration after an import, ensure that the
`entries` map contains every imported prefix you want Terraform to continue
managing. Removing an imported prefix from `entries` tells Terraform to delete
that prefix.
