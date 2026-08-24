---
page_title: "fastly_configstores Data Source - fastly"
subcategory: ""
description: |-
  Retrieves the Fastly Config Stores available to the account.
---

# fastly_configstores (Data Source)

Use this data source to retrieve the Fastly Config Stores available to the
current account.

The `stores` attribute is a set because the Fastly API does not guarantee the
order in which Config Stores are returned.

## Example Usage

```terraform
data "fastly_configstores" "example" {}

output "config_stores" {
  value = data.fastly_configstores.example.stores
}
```

Find a Config Store by name:

```terraform
data "fastly_configstores" "example" {}

locals {
  config_store_id = one([
    for store in data.fastly_configstores.example.stores :
    store.id if store.name == "my_config_store"
  ])
}
```

## Schema

### Read-Only

- `id` (String) Stable Terraform data source identifier derived from the returned Config Store IDs.
- `stores` (Attributes Set) The Config Stores available to the account. Set semantics are used because the Fastly API does not guarantee list ordering. (see [below for nested schema](#nestedatt--stores))

<a id="nestedatt--stores"></a>
### Nested Schema for `stores`

Read-Only:

- `id` (String) An alphanumeric string identifying the Config Store.
- `name` (String) The name of the Config Store.
