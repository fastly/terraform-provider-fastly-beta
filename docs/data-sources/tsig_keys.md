---
page_title: "fastly_tsig_keys Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly TSIG keys.
---

# fastly_tsig_keys (Data Source)

Use this data source to retrieve a list of Fastly TSIG keys.

## Example Usage

```terraform
data "fastly_tsig_keys" "example" {}

output "fastly_tsig_keys_all" {
  value = data.fastly_tsig_keys.example.keys
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `keys` (Attributes Set) A list of TSIG keys. (see [below for nested schema](#nestedatt--keys))
- `total` (Number) The total number of TSIG keys returned.

<a id="nestedatt--keys"></a>
### Nested Schema for `keys`

Read-Only:

- `algorithm` (String) The algorithm of the TSIG key.
- `description` (String) A freeform descriptive note.
- `id` (String) TSIG Key Identifier (UUID).
- `name` (String) The name of the TSIG key.
