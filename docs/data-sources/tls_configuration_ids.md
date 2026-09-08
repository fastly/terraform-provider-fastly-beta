---
page_title: "fastly_tls_configuration_ids Data Source - fastly"
subcategory: ""
description: |-
  Get IDs of available TLS configurations.
---

# fastly_tls_configuration_ids (Data Source)

Use this data source to get the IDs of available TLS configurations for use with other resources.

## Example Usage

```terraform
data "fastly_tls_configuration_ids" "example" {}

output "tls_configuration_ids" {
  value = data.fastly_tls_configuration_ids.example.ids
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `ids` (Set of String) List of IDs corresponding to available TLS configurations.
