---
page_title: "fastly_routing_config_domain_link Resource - fastly"
subcategory: ""
description: |-
  Links a versionless fastly_domain to a fastly_routing_config.
---

# fastly_routing_config_domain_link (Resource)

Links a versionless `fastly_domain` to a `fastly_routing_config`, independent of managing the
domain or routing config themselves.

## Example Usage

```terraform
resource "fastly_domain" "example" {
  fqdn = "www.example.com"
}

resource "fastly_routing_config" "example" {
  name = "example-routing-config"

  path {
    path = "/"

    rule {
      action_type  = "service"
      action_value = fastly_service_cdn.example.id
    }
  }
}

resource "fastly_routing_config_domain_link" "example" {
  domain_id         = fastly_domain.example.id
  routing_config_id = fastly_routing_config.example.routing_config_id
}
```

## Schema

### Required

- `domain_id` (String) The Domain Identifier of the versionless domain being linked (UUID).
- `routing_config_id` (String) The routing config identifier to link to the domain.

### Read-Only

- `id` (String) The ID of this resource (identical to `domain_id`).

## Import

Fastly Routing Config Domain Links can be imported using the linked Domain ID, e.g.

```shell
terraform import fastly_routing_config_domain_link.example <domain_id>
```
