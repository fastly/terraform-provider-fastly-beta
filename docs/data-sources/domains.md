---
page_title: "fastly_domains Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly domains.
---

# fastly_domains (Data Source)

Use this data source to retrieve a list of Fastly domains.

## Example Usage

```terraform
data "fastly_domains" "example" {}

output "fastly_domains_all" {
  value = data.fastly_domains.example.domains
}

output "fastly_domains_filtered" {
  # Example: get the ID of the domain with fqdn "www.example.com"
  value = one([
    for domain in data.fastly_domains.example.domains :
    domain.id if domain.fqdn == "www.example.com"
  ])
}
```

## Schema

### Read-Only

- `domains` (Attributes Set) A domain represents the domain name through which visitors will retrieve content. There can be multiple domains for a service. (see [below for nested schema](#nestedatt--domains))
- `id` (String) Terraform data source identifier.
- `total` (Number) The total number of domains returned.

<a id="nestedatt--domains"></a>
### Nested Schema for `domains`

Read-Only:

- `fqdn` (String) The fully-qualified domain name for your domain.
- `id` (String) Domain Identifier (UUID).
- `service_id` (String) The `service_id` associated with your domain or `null` if there is no association.
