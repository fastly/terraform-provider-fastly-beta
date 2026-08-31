---
page_title: "fastly_dns_zone Resource - fastly"
subcategory: ""
description: |-
  Provides a Fastly DNS Zone.
---

# fastly_dns_zone (Resource)

Provides a Fastly DNS Zone.

This resource is versionless: it is not tied to a service version and is managed independently of any `fastly_service_cdn` or `fastly_service_compute` resource.

## Example Usage

Basic usage:

```terraform
resource "fastly_dns_zone" "example" {
  name        = "example.com."
  description = "Managed by Terraform"
}
```

With inbound zone transfer configuration:

```terraform
resource "fastly_dns_zone" "example" {
  name        = "example.com."
  description = "Managed by Terraform"

  xfr_config_inbound {
    inbound_tsig_key_id = "TSIG_KEY_ID"

    primaries {
      address     = "203.0.113.1"
      description = "Primary DNS server"
    }
  }
}
```

## Schema

### Required

- `name` (String) The domain name for your zone in FQDN format (e.g. `example.com.`). Must include a trailing period. The API provides no way to rename a zone in place, so changing this attribute will delete and recreate the resource.

### Optional

- `description` (String) A freeform descriptive note.
- `xfr_config_inbound` (Block List) All attributes associated with inbound zone transfers. (see [below for nested schema](#nestedblock--xfr_config_inbound))

### Read-Only

- `id` (String) Zone Identifier.

<a id="nestedblock--xfr_config_inbound"></a>
### Nested Schema for `xfr_config_inbound`

Optional:

- `inbound_tsig_key_id` (String) The ID of the TSIG key used to secure inbound zone transfers.
- `primaries` (Block List) An array of the primary DNS server objects associated with inbound zone transfers. (see [below for nested schema](#nestedblock--xfr_config_inbound--primaries))

<a id="nestedblock--xfr_config_inbound--primaries"></a>
### Nested Schema for `xfr_config_inbound.primaries`

Optional:

- `address` (String) An IPv4 address for the Primary DNS Server. IPv6 is not supported for DNS zone transfers.
- `description` (String) A description of the Primary DNS server.

## Import

Fastly DNS Zones can be imported using their Zone ID, e.g.

```shell
terraform import fastly_dns_zone.example <zone_id>
```
