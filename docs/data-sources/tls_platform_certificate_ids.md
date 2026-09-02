---
page_title: "fastly_tls_platform_certificate_ids Data Source - fastly"
subcategory: ""
description: |-
  Get IDs of available Platform TLS certificates.
---

# fastly_tls_platform_certificate_ids (Data Source)

Use this data source to get the IDs of available Platform TLS certificates for use with other
resources, notably `fastly_tls_platform_certificate`.

## Example Usage

```terraform
data "fastly_tls_platform_certificate_ids" "example" {}

data "fastly_tls_platform_certificate" "example" {
  id = data.fastly_tls_platform_certificate_ids.example.ids[0]
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `ids` (Set of String) IDs of every Platform TLS certificate.
