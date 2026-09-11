---
page_title: "fastly_tls_domain Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to look up activations, certificates, and subscriptions associated with a TLS domain.
---

# fastly_tls_domain (Data Source)

Use this data source to look up activations, certificates, and subscriptions associated with a TLS domain.

If more or less than a single domain matches the search, the read fails. Ensure that your search is specific enough to return a single result.

## Example Usage

```terraform
data "fastly_tls_domain" "example" {
  domain = "example.com"
}

output "certificate_ids" {
  value = data.fastly_tls_domain.example.tls_certificate_ids
}
```

## Schema

### Required

- `domain` (String) Domain name to look up activations, certificates and subscriptions for.

### Read-Only

- `id` (String) Terraform data source identifier. Mirrors domain.
- `tls_activation_ids` (Set of String) IDs of the activations associated with the domain.
- `tls_certificate_ids` (Set of String) IDs of the certificates associated with the domain.
- `tls_subscription_ids` (Set of String) IDs of the subscriptions associated with the domain.
