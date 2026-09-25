---
page_title: "fastly_dictionaries Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly dictionaries for a service version.
---

# fastly_dictionaries (Data Source)

Use this data source to retrieve a list of Fastly dictionaries for a service version, including
each dictionary's ID - useful when importing `fastly_service_dictionary_items` without direct
access to the Fastly API or control panel.

## Example Usage

```terraform
data "fastly_dictionaries" "example" {
  service_id      = fastly_service_cdn_auto.example.id
  service_version = fastly_service_cdn_auto.example.active_version
}

output "dictionary_ids_by_name" {
  value = {
    for dictionary in data.fastly_dictionaries.example.dictionaries :
    dictionary.name => dictionary.id
  }
}
```

## Schema

### Required

- `service_id` (String) Fastly service ID.
- `service_version` (Number) Fastly service version to read dictionaries from.

### Read-Only

- `dictionaries` (Attributes Set) List of all dictionaries for the configured service version. (see [below for nested schema](#nestedatt--dictionaries))
- `id` (String) Terraform data source identifier.

<a id="nestedatt--dictionaries"></a>
### Nested Schema for `dictionaries`

Read-Only:

- `id` (String) Alphanumeric string identifying the dictionary.
- `name` (String) Name of the dictionary.
- `write_only` (Boolean) Whether items in the dictionary are readable or not.
