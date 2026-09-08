---
page_title: "fastly_dns_zones Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly DNS zones.
---

# fastly_dns_zones (Data Source)

Use this data source to retrieve a list of Fastly DNS zones.

## Example Usage

```terraform
data "fastly_dns_zones" "example" {}

output "fastly_dns_zones_all" {
  value = data.fastly_dns_zones.example.zones
}

output "fastly_dns_zones_filtered" {
  # Example: get the ID of the zone named "example.com."
  value = one([
    for zone in data.fastly_dns_zones.example.zones :
    zone.id if zone.name == "example.com."
  ])
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `total` (Number) The total number of DNS zones returned.
- `zones` (Attributes Set) A list of DNS zones. (see [below for nested schema](#nestedatt--zones))

<a id="nestedatt--zones"></a>
### Nested Schema for `zones`

Read-Only:

- `description` (String) A freeform descriptive note.
- `id` (String) Zone Identifier.
- `name` (String) The domain name for the zone.
