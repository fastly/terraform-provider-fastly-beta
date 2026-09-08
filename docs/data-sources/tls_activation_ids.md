---
page_title: "fastly_tls_activation_ids Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to get the IDs of available TLS activations, optionally filtered by certificate.
---

# fastly_tls_activation_ids (Data Source)

Use this data source to get the IDs of available TLS activations, optionally filtered by certificate.

## Example Usage

```terraform
data "fastly_tls_activation_ids" "example" {
  certificate_id = fastly_tls_certificate.example.id
}

output "activation_ids" {
  value = data.fastly_tls_activation_ids.example.ids
}
```

## Schema

### Optional

- `certificate_id` (String) ID of TLS certificate used to filter activations.

### Read-Only

- `id` (String) Terraform data source identifier.
- `ids` (Set of String) List of IDs of the TLS Activations.
