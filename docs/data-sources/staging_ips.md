---
page_title: "fastly_staging_ips Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve the staging IP addresses assigned to a Fastly service version's domains.
---

# fastly_staging_ips (Data Source)

Use this data source to retrieve the staging IP addresses assigned to a Fastly service version's domains.

## Example Usage

```terraform
data "fastly_staging_ips" "example" {
  service_id      = fastly_service_cdn_auto.example.id
  service_version = fastly_service_cdn_auto.example.active_version
}

output "staging_ips" {
  value = data.fastly_staging_ips.example.domains
}
```

## Schema

### Required

- `service_id` (String) Alphanumeric string identifying the service.
- `service_version` (Number) Integer identifying a service version.

### Read-Only

- `domains` (Attributes Set) List of domains with their staging IP addresses. (see [below for nested schema](#nestedatt--domains))
- `id` (String) Terraform data source identifier.

<a id="nestedatt--domains"></a>
### Nested Schema for `domains`

Read-Only:

- `name` (String) The domain name.
- `staging_ip` (String) The staging IP address for the domain.
