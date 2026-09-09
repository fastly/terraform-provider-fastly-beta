---
page_title: "fastly_object_storage_access_keys Resource - fastly"
subcategory: ""
description: |-
  Provides an Object Storage Access Key, used to manage resources in various clouds. Access keys are versionless and independent of any service-version lifecycle. The resource is immutable: any change to description, permission, or buckets destroys and recreates it.
---

# fastly_object_storage_access_keys (Resource)

Provides an Object Storage Access Key, used to manage resources in various clouds. Access keys are versionless and independent of any service-version lifecycle. The resource is immutable: any change to `description`, `permission`, or `buckets` destroys and recreates it.

`authentication.secret_key` is only returned by the create response, so it is never populated by a `terraform import` or by any later read; it is only ever present in state immediately after the resource is created.

## Example Usage

```terraform
resource "fastly_object_storage_access_keys" "example" {
  description = "access key for my application"
  permission  = "read-write-objects"
  buckets     = ["bucket1", "bucket2"]
}
```

## Schema

### Required

- `description` (String) The description of the access key. Access keys cannot be updated, so changing this attribute destroys and recreates the resource.
- `permission` (String) The permissions of the access key. Access keys cannot be updated, so changing this attribute destroys and recreates the resource.

### Optional

- `buckets` (List of String) The buckets the access key will be associated with. Access keys cannot be updated, so changing this attribute destroys and recreates the resource.

### Read-Only

- `access_key_id` (String) ID for the object storage access key.
- `authentication` (Attributes) Sensitive credential material for the access key. (see [below for nested schema](#nestedatt--authentication))
- `id` (String) Same value as `access_key_id`.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Read-Only:

- `secret_key` (String, Sensitive) Secret key for the object storage access key. Only returned at creation time, so it is not populated after a `terraform import`.

## Import

An Object Storage Access Key can be imported using its access key ID, e.g.

```shell
terraform import fastly_object_storage_access_keys.example xxxxxxxx
```

`authentication.secret_key` is not returned on read and so is not populated by import; it remains null in state unless the resource is later replaced.
