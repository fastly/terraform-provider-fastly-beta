---
page_title: "fastly_tls_subscription_ids Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to get the IDs of available TLS subscriptions.
---

# fastly_tls_subscription_ids (Data Source)

Use this data source to get the IDs of available TLS subscriptions.

## Example Usage

```terraform
data "fastly_tls_subscription_ids" "example" {}

output "subscription_ids" {
  value = data.fastly_tls_subscription_ids.example.ids
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `ids` (Set of String) IDs of available TLS subscriptions.
