---
page_title: "fastly_tls_private_key_ids Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to get the IDs of all available TLS private keys.
---

# fastly_tls_private_key_ids (Data Source)

Use this data source to get the IDs of all available TLS private keys.

## Example Usage

```terraform
data "fastly_tls_private_key_ids" "example" {}

output "private_key_ids" {
  value = data.fastly_tls_private_key_ids.example.ids
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `ids` (Set of String) List of IDs of the TLS private keys.
