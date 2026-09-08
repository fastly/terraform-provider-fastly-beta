---
page_title: "fastly_tls_certificate_ids Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to get the IDs of available TLS certificates.
---

# fastly_tls_certificate_ids (Data Source)

Use this data source to get the IDs of available TLS certificates.

## Example Usage

```terraform
data "fastly_tls_certificate_ids" "example" {}

resource "fastly_tls_activation" "example" {
  certificate_id = data.fastly_tls_certificate_ids.example.ids[0]
  domain         = "example.com"
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `ids` (Set of String) List of IDs corresponding to Custom TLS certificates.
