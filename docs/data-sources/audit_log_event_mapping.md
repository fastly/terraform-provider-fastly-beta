---
page_title: "fastly_audit_log_event_mapping Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to get the details of an Audit Log Event Mapping, by ID or by other lookup criteria.
---

# fastly_audit_log_event_mapping (Data Source)

Use this data source to get the details of an Audit Log Event Mapping, by ID or by other lookup
criteria.

The filters are applied using an **AND** boolean operator, so depending on the combination of
filters, they may become mutually exclusive. `id` must not be specified in combination with any of
the others.

If more or less than a single match is returned by the search, the read fails. Ensure that your
search is specific enough to return a single result.

## Example Usage

```terraform
data "fastly_audit_log_event_mapping" "by_id" {
  id = "abc123"
}

data "fastly_audit_log_event_mapping" "by_name" {
  name       = "example service backend changes"
  scope_type = "vcl"
}
```

## Schema

### Optional

- `id` (String) Unique ID of the mapping. Conflicts with all the other filters.
- `integration_id` (String) Filters results to mappings that reference the given integration ID.
- `mapping_status` (String) Filters results by mapping status: `active` or `inactive`.
- `name` (String) Filters results to mappings whose name contains the given string (case-insensitive).
- `scope_id` (String) Filters results to mappings that apply to the given service or workspace ID.
- `scope_type` (String) Filters results to the given scope type: `account`, `vcl`, `wasm`, or `ngwaf`.

### Read-Only

- `created_at` (String) Timestamp (RFC3339) when the mapping was created.
- `description` (String) Description of the mapping.
- `event_types` (Set of String) The audit event types that trigger a notification.
- `integration_ids` (Set of String) The IDs of the integrations that receive notifications.
- `scope_ids` (Set of String) The specific service or workspace IDs the mapping is scoped to.
- `updated_at` (String) Timestamp (RFC3339) when the mapping was last updated.
