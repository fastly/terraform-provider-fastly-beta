---
page_title: "fastly_services Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly services in your account.
---

# fastly_services (Data Source)

Use this data source to retrieve a list of [Fastly services](https://developer.fastly.com/reference/api/services/service/) in your account.

## Example Usage

```terraform
data "fastly_services" "example" {}

output "fastly_services_all" {
  value = data.fastly_services.example.details
}

output "fastly_services_filtered" {
  # Example: get the ID of the service named "Example Service"
  value = one([
    for svc in data.fastly_services.example.details :
    svc.id if svc.name == "Example Service"
  ])
}
```

## Schema

### Read-Only

- `details` (Attributes Set) A detailed list of Fastly services in your account. This is limited to the services the API token can read. (see [below for nested schema](#nestedatt--details))
- `id` (String) Terraform data source identifier.
- `ids` (Set of String) A list of service IDs in your account. This is limited to the services the API token can read.

<a id="nestedatt--details"></a>
### Nested Schema for `details`

Read-Only:

- `comment` (String) A freeform descriptive note.
- `created_at` (String) Date and time in ISO 8601 format.
- `customer_id` (String) Alphanumeric string identifying the customer.
- `id` (String) Alphanumeric string identifying the service.
- `name` (String) The name of the service.
- `type` (String) The type of this service. One of `vcl`, `wasm`.
- `updated_at` (String) Date and time in ISO 8601 format.
- `version` (Number) The currently activated version.
