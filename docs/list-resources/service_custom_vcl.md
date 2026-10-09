---
page_title: "fastly_service_custom_vcl List Resource - fastly"
subcategory: ""
description: |-
  List all custom VCL files across all Fastly CDN services at their active version, or latest version when no active version exists.
---

# fastly_service_custom_vcl (List Resource)

List all custom VCL files across all Fastly CDN services at their active version,
or latest version when no active version exists.

## Schema

### Optional

- `service_id` (String) Optional Fastly service ID to restrict discovery to a single service. When omitted, all services accessible to the API token are considered.
