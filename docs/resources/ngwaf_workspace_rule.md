---
page_title: "fastly_ngwaf_workspace_rule Resource - fastly"
subcategory: ""
description: |-
  Manages a Fastly Next-Gen WAF rule scoped to a single workspace.
---

# fastly_ngwaf_workspace_rule (Resource)

Manages a Fastly Next-Gen WAF rule scoped to a single workspace.

Workspace-scoped rules support all four rule types (`request`, `signal`,
`rate_limit`, and `templated_signal`) and the full workspace action set.

## Example Usage

```terraform
resource "fastly_ngwaf_workspace" "example" {
  name        = "Example Workspace"
  description = "Managed by Terraform"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_rule" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
  type         = "request"
  description  = "Block a specific IP"
  enabled      = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}
```

### Grouped and multi-value conditions

```terraform
resource "fastly_ngwaf_workspace_rule" "grouped_example" {
  workspace_id = fastly_ngwaf_workspace.example.id
  type         = "request"
  description  = "Block admin paths hit without the expected header"
  enabled      = true

  group_condition {
    group_operator = "all"

    condition {
      field    = "path"
      operator = "contains"
      value    = "/admin"
    }

    multival_condition {
      field          = "request_header"
      operator       = "does_not_exist"
      group_operator = "any"

      condition {
        field    = "name"
        operator = "equals"
        value    = "X-Internal-Token"
      }
    }
  }

  action {
    type = "block"
  }
}
```

### Standalone multi-value condition

`multival_condition` can also be declared at the rule's top level, independent
of any `group_condition` - useful when the multival check is the rule's only
condition.

```terraform
resource "fastly_ngwaf_workspace_rule" "debug_query_param" {
  workspace_id = fastly_ngwaf_workspace.example.id
  type         = "request"
  description  = "Block requests carrying a debug query parameter"
  enabled      = true

  multival_condition {
    field          = "query_parameter"
    operator       = "exists"
    group_operator = "any"

    condition {
      field    = "name"
      operator = "equals"
      value    = "debug"
    }
  }

  action {
    type = "block"
  }
}
```

### Rate limit rule

```terraform
resource "fastly_ngwaf_workspace_rule" "rate_limit_example" {
  workspace_id = fastly_ngwaf_workspace.example.id
  type         = "rate_limit"
  description  = "Rate limit login attempts by IP"
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  rate_limit {
    duration  = 300
    interval  = 60
    signal    = "site.excessive-login-attempts"
    threshold = 100

    client_identifiers {
      type = "ip"
    }

    client_identifiers {
      type = "request_cookie"
      name = "session_id"
    }
  }

  action {
    type   = "log_request"
    signal = "site.excessive-login-attempts"
  }
}
```

### Signal exclusion rule

```terraform
resource "fastly_ngwaf_workspace_rule" "exclude_xss_signal" {
  workspace_id = fastly_ngwaf_workspace.example.id
  type         = "signal"
  description  = "Exclude XSS signal to address a false positive"
  enabled      = true

  condition {
    field    = "path"
    operator = "like"
    value    = "/contact-form"
  }

  action {
    type   = "exclude_signal"
    signal = "XSS"
  }
}
```

### Templated signal rule

`templated_signal` is the one action type whose name collides with a rule
`type` value: it's the sole action a `templated_signal`-type rule may
declare. `description` must be an empty string.

```terraform
resource "fastly_ngwaf_workspace_rule" "login_attempt_signal" {
  workspace_id = fastly_ngwaf_workspace.example.id
  type         = "templated_signal"
  description  = ""
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  action {
    type   = "templated_signal"
    signal = "LOGINATTEMPT"
  }
}
```

## Action types by rule type

A rule's `type` determines which `action.type` values are valid, and how
many actions the rule may declare:

| Rule `type`        | Valid action types                                                                      | Actions |
| ------------------- | ----------------------------------------------------------------------------------------- | ------- |
| `request`           | `allow`, `block`, `add_signal`, `browser_challenge`, `verify_token`, `dynamic_challenge`, `deception` | 1-2     |
| `signal`            | `exclude_signal`                                                                           | exactly 1 |
| `rate_limit`        | `block_signal`, `log_request`, `browser_challenge`, `dynamic_challenge`, `deception`        | exactly 1 |
| `templated_signal`  | `templated_signal`                                                                          | exactly 1 |

An action's `type` also determines which of its other fields are required:
`add_signal`, `exclude_signal`, `block_signal`, `log_request`, and
`templated_signal` all require `signal`; `browser_challenge` requires
`allow_interactive`; `deception` requires `deception_type` (one of
`invalid_login_response`, `vulnerable_application_response`, or `ato`).
`redirect_url`/`response_code` are optional companions to `block` and
`block_signal`.

## Schema

### Required

- `description` (String) The description of the rule. Must be an empty string for `templated_signal` rules.
- `enabled` (Boolean) Whether the rule is currently enabled.
- `type` (String) The type of the rule. One of `request`, `signal`, `rate_limit`, or `templated_signal`.
- `workspace_id` (String) The ID of the workspace this rule belongs to.

### Optional

- `action` (Block List) List of actions to perform when the rule matches. At least one is required. (see [below for nested schema](#nestedblock--action))
- `condition` (Block List) Flat list of individual conditions. Each must include `field`, `operator`, and `value`. (see [below for nested schema](#nestedblock--condition))
- `group_condition` (Block List) List of grouped conditions with nested logic. Each group must define a `group_operator` and at least one `condition` or `multival_condition`. (see [below for nested schema](#nestedblock--group_condition))
- `group_operator` (String) Logical operator applied across the rule's top-level condition, group_condition, and multival_condition entries. One of `any` or `all`. Defaults to `all`.
- `multival_condition` (Block List) List of multival conditions with nested logic. Each multival must define a `field`, `operator`, and `group_operator`, and at least one condition. (see [below for nested schema](#nestedblock--multival_condition))
- `rate_limit` (Block List) Configuration specific to `rate_limit`-type rules. (see [below for nested schema](#nestedblock--rate_limit))
- `request_logging` (String) Logging behavior for matching requests. Only valid for `request`-type rules. One of `sampled` or `none`. Defaults to `sampled` when set on a `request` rule.

### Read-Only

- `id` (String) The rule identifier generated by Fastly.

<a id="nestedblock--action"></a>
### Nested Schema for `action`

Required:

- `type` (String) The action type. Valid values depend on the rule's `type`: `request` rules allow `allow`, `block`, `add_signal`, `browser_challenge`, `verify_token`, `dynamic_challenge`, or `deception` (1-2 actions); `signal` rules require exactly `exclude_signal`; `rate_limit` rules allow exactly one of `block_signal`, `log_request`, `browser_challenge`, `dynamic_challenge`, or `deception`; `templated_signal` rules require exactly `templated_signal`.

Optional:

- `allow_interactive` (Boolean) Specifies if interaction is allowed. Required when `type = browser_challenge`.
- `deception_type` (String) Specifies the type of deception. Required when `type = deception`. One of `invalid_login_response`, `vulnerable_application_response`, or `ato`.
- `redirect_url` (String) Redirect target. Optional companion to `type = block` or `type = block_signal`.
- `response_code` (Number) Response code used with `type = block` or `type = block_signal`.
- `signal` (String) Signal name. Required by `add_signal`, `exclude_signal`, `block_signal`, `log_request`, and `templated_signal`; optional on `browser_challenge`, `dynamic_challenge`, and `deception`.


<a id="nestedblock--condition"></a>
### Nested Schema for `condition`

Required:

- `field` (String) Field to inspect. One of `agent_name`, `country`, `domain`, `ip`, `ip_remote`, `ja3_fingerprint`, `ja4_fingerprint`, `key_name`, `method`, `parameter_name`, `parameter_value`, `path`, `protocol_version`, `response_code`, `scheme`, or `user_agent`.
- `operator` (String) Operator to apply. One of `equals`, `does_not_equal`, `contains`, `does_not_contain`, `like`, `not_like`, `in_list`, `not_in_list`, `matches`, `does_not_match`, `greater_equal`, or `lesser_equal`.
- `value` (String) The value to test the field against.


<a id="nestedblock--group_condition"></a>
### Nested Schema for `group_condition`

Required:

- `group_operator` (String) Logical operator for the group. One of `any` or `all`.

Optional:

- `condition` (Block List) A list of nested conditions in this group. (see [below for nested schema](#nestedblock--group_condition--condition))
- `multival_condition` (Block List) List of nested multival conditions in this group. Each multival must define a `field`, `operator`, and `group_operator`, and at least one condition. (see [below for nested schema](#nestedblock--group_condition--multival_condition))

<a id="nestedblock--group_condition--condition"></a>
### Nested Schema for `group_condition.condition`

Required:

- `field` (String) Field to inspect. One of `agent_name`, `country`, `domain`, `ip`, `ip_remote`, `ja3_fingerprint`, `ja4_fingerprint`, `key_name`, `method`, `parameter_name`, `parameter_value`, `path`, `protocol_version`, `response_code`, `scheme`, or `user_agent`.
- `operator` (String) Operator to apply. One of `equals`, `does_not_equal`, `contains`, `does_not_contain`, `like`, `not_like`, `in_list`, `not_in_list`, `matches`, `does_not_match`, `greater_equal`, or `lesser_equal`.
- `value` (String) The value to test the field against.


<a id="nestedblock--group_condition--multival_condition"></a>
### Nested Schema for `group_condition.multival_condition`

Required:

- `field` (String) Field to inspect. One of `post_parameter`, `query_parameter`, `request_cookie`, `request_header`, `response_header`, or `signal`.
- `group_operator` (String) Logical operator used to evaluate the nested conditions. One of `any` or `all`.
- `operator` (String) Whether the nested conditions check for existence or non-existence of matching field values. One of `exists` or `does_not_exist`.

Optional:

- `condition` (Block List) Nested conditions evaluated against the multival field. At least one is required. (see [below for nested schema](#nestedblock--group_condition--multival_condition--condition))

<a id="nestedblock--group_condition--multival_condition--condition"></a>
### Nested Schema for `group_condition.multival_condition.condition`

Required:

- `field` (String) Field to inspect. One of `name`, `value`, `value_string`, `value_int`, `value_ip`, `signal_id`, `parameter_name`, or `parameter_value`.
- `operator` (String) Operator to apply. One of `equals`, `does_not_equal`, `contains`, `does_not_contain`, `like`, `not_like`, `in_list`, `not_in_list`, `matches`, `does_not_match`, `greater_equal`, or `lesser_equal`.
- `value` (String) The value to test the field against.




<a id="nestedblock--multival_condition"></a>
### Nested Schema for `multival_condition`

Required:

- `field` (String) Field to inspect. One of `post_parameter`, `query_parameter`, `request_cookie`, `request_header`, `response_header`, or `signal`.
- `group_operator` (String) Logical operator used to evaluate the nested conditions. One of `any` or `all`.
- `operator` (String) Whether the nested conditions check for existence or non-existence of matching field values. One of `exists` or `does_not_exist`.

Optional:

- `condition` (Block List) Nested conditions evaluated against the multival field. At least one is required. (see [below for nested schema](#nestedblock--multival_condition--condition))

<a id="nestedblock--multival_condition--condition"></a>
### Nested Schema for `multival_condition.condition`

Required:

- `field` (String) Field to inspect. One of `name`, `value`, `value_string`, `value_int`, `value_ip`, `signal_id`, `parameter_name`, or `parameter_value`.
- `operator` (String) Operator to apply. One of `equals`, `does_not_equal`, `contains`, `does_not_contain`, `like`, `not_like`, `in_list`, `not_in_list`, `matches`, `does_not_match`, `greater_equal`, or `lesser_equal`.
- `value` (String) The value to test the field against.



<a id="nestedblock--rate_limit"></a>
### Nested Schema for `rate_limit`

Required:

- `duration` (Number) Duration in seconds for the rate limit. Minimum `300`, maximum `86400`.
- `interval` (Number) Time interval for the rate limit in seconds. One of `60`, `600`, or `3600`.
- `signal` (String) Reference ID of the custom signal this rule uses to count requests.
- `threshold` (Number) Rate limit threshold. Minimum `1`, maximum `100000`.

Optional:

- `client_identifiers` (Block Set) List of client identifiers used for rate limiting. Must contain 1 or 2 entries. (see [below for nested schema](#nestedblock--rate_limit--client_identifiers))

<a id="nestedblock--rate_limit--client_identifiers"></a>
### Nested Schema for `rate_limit.client_identifiers`

Required:

- `type` (String) Type of the Client Identifier. One of `ip`, `post_parameter`, `request_cookie`, `request_header`, or `signal_payload`.

Optional:

- `key` (String) Key for the Client Identifier. Only valid when `type` is `request_header`; excluded otherwise.
- `name` (String) Name for the Client Identifier. Required when `type` is `request_header`, `request_cookie`, or `post_parameter`; excluded when `type` is `ip` or `signal_payload`.
- `signal` (String) Signal for the Client Identifier. Required when `type` is `signal_payload`; excluded otherwise.

## Import

Next-Gen WAF workspace rules are imported using a combination of the
workspace ID and the rule ID, separated by a `/`:

```shell
terraform import fastly_ngwaf_workspace_rule.example WORKSPACE_ID/RULE_ID
```
