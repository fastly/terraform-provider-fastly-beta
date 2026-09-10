---
page_title: "fastly_user_service_authorization Resource - fastly"
subcategory: ""
description: |-
  Grants a user permissions on a service. User service authorizations are versionless and independent of any service-version lifecycle.
---

# fastly_user_service_authorization (Resource)

Grants a user permissions on a service. User service authorizations are versionless and independent of any service-version lifecycle.

`user_id` must be an existing Fastly user account ID; there is no Terraform resource for managing users, so it must come from another source (e.g. the Fastly control panel or API).

## Example Usage

```terraform
resource "fastly_service_cdn" "example" {
  name    = "my_vcl_service"
  version = 1

  force_destroy = true
}

resource "fastly_user_service_authorization" "example" {
  service_id = fastly_service_cdn.example.id
  user_id    = "4whrmweg9bskt3knuqk4bs"
  permission = "purge_all"
}
```

## Schema

### Required

- `service_id` (String) The ID of the service to grant permissions for.
- `user_id` (String) The ID of the user being given access to the service.

### Optional

- `permission` (String) The permissions to grant the user. Can be `full`, `read_only`, `purge_select` or `purge_all`. Default: `full`.

### Read-Only

- `id` (String) The ID of this user service authorization.

## Import

A Fastly User Service Authorization can be imported using its ID, e.g.

```shell
terraform import fastly_user_service_authorization.example xxxxxxxxxxxxxxxxxxxx
```
