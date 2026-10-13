---
page_title: "fastly_service_cdn_acl_entries List Resource - fastly"
subcategory: ""
description: |-
  List ACL entries for all ACLs across all Fastly CDN services at their active version, or latest version when no active version exists.
---

# fastly_service_cdn_acl_entries (List Resource)

List ACL entries for all ACLs across all Fastly CDN services at their active version, or latest version when no active version exists.

## Schema

### Optional

- `service_id` (String) Optional Fastly service ID to restrict discovery to a single service. When omitted, all services accessible to the API token are considered.
