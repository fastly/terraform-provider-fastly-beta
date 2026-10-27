---
page_title: "fastly_routing_config_auto Resource - fastly"
subcategory: ""
description: |-
  Provides a Fastly Routing Config with automatic versioning.
---

# fastly_routing_config_auto (Resource)

Routing configs let you route requests to different services (or other destinations) based on
URL path, using conditional rules. This resource manages a routing config's `path` and `rule`
configuration using **automatic versioning**: every apply reconciles the routing config's draft
paths and rules against this resource's configuration and, if anything changed, activates the
result as the new active version. The underlying draft/version workflow exposed by the API is
never surfaced to Terraform - there's no `version` or `activate` argument for you to manage.

A routing config isn't useful on its own; it must be linked to one or more domains via
`fastly_routing_config_domain_link` before it will route traffic.

## Example Usage

```terraform
resource "fastly_routing_config_auto" "example" {
  name = "example-routing-config"

  path {
    path = "/api/*"

    rule {
      action_type  = "service"
      action_value = fastly_service_cdn.example.id
    }
  }

  path {
    path = "/"

    rule {
      action_type  = "service"
      action_value = fastly_service_cdn.example.id

      condition {
        type     = "header"
        key      = "X-Beta"
        operator = "equals"
        value    = "true"
      }
    }

    rule {
      action_type  = "service"
      action_value = fastly_service_cdn.example.id
    }
  }
}
```

## Schema

### Required

- `name` (String) The user-defined name of the routing config (max 255 characters). Can be created, but not updated.

### Optional

- `path` (Block List) A URL path pattern and the rules used to route matching requests. At least one `path` block is required. (see [below for nested schema](#nestedblock--path))

### Read-Only

- `activated_at` (String) The date and time the routing config's active version was most recently activated, in ISO 8601 format.
- `id` (String) Alphanumeric string identifying the routing config.

<a id="nestedblock--path"></a>
### Nested Schema for `path`

Required:

- `path` (String) The URL path pattern (max 2048 characters, starts with `/`).

Optional:

- `rule` (Block List) A conditional routing rule for this path; each `path` requires at least one, including a default rule. Rules are evaluated in the order listed and the first match wins, so order matters. A rule with no `condition` blocks is the default (catch-all) rule for the path. (see [below for nested schema](#nestedblock--path--rule))

Read-Only:

- `path_id` (String) Alphanumeric string identifying the path.

<a id="nestedblock--path--rule"></a>
### Nested Schema for `path.rule`

Required:

- `action_type` (String) The action type. Currently only `service` is supported.
- `action_value` (String) The destination for the action (e.g. a service ID when `action_type` is `service`).

Optional:

- `condition` (Block List) A matching criterion evaluated against the incoming request. Omit to make this the default (catch-all) rule for the path. (see [below for nested schema](#nestedblock--path--rule--condition))

Read-Only:

- `is_default` (Boolean) Whether this is the catch-all rule for its path (i.e. it has no conditions).
- `rule_id` (String) The rule identifier.

<a id="nestedblock--path--rule--condition"></a>
### Nested Schema for `path.rule.condition`

Required:

- `operator` (String) The comparison operator used to evaluate `value` against the request. One of `equals`, `starts_with`, `ends_with`, or `contains`.
- `type` (String) The condition category. Currently only `header` is supported.
- `value` (String) The value compared against the request using `operator`.

Optional:

- `key` (String) The header name to match against. Required when `type` is `header`.

## Import

Routing configs can be imported using their ID, e.g.

```shell
terraform import fastly_routing_config_auto.example <routing_config_id>
```
