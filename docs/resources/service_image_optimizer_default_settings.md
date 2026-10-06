---
page_title: "fastly_service_image_optimizer_default_settings Resource - fastly"
subcategory: ""
description: |-
  Fastly service Image Optimizer default settings resource. Writes directly to the specified writable service version.
---

# fastly_service_image_optimizer_default_settings (Resource)

Fastly service Image Optimizer default settings resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages the Image Optimizer default settings on the configured service version.
It does not clone, activate, or stage service versions. Image Optimizer is only
supported on CDN services.

Image Optimizer must be enabled on the service before these settings can be
written. Enabling the product is versionless, so add a `depends_on` on
`fastly_service_product_image_optimizer` to enable it in the same `apply`.

Every write sets all Image Optimizer default settings, replacing any values
previously changed in the UI or API. Omitted attributes are set to their
defaults.

## Example Usage

```terraform
resource "fastly_service_product_image_optimizer" "example" {
  service_id = fastly_service_cdn.example.id
}

resource "fastly_service_image_optimizer_default_settings" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1

  resize_filter = "bicubic"
  webp          = true
  webp_quality  = 80
  jpeg_type     = "progressive"
  jpeg_quality  = 90

  depends_on = [fastly_service_product_image_optimizer.example]
}
```

## Schema

### Required

- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `allow_video` (Boolean) Enables GIF to MP4 transformations on this service. Default `false`.
- `jpeg_quality` (Number) The default quality to use with JPEG output. This can be overridden with the `quality` parameter on specific image optimizer requests. Default `85`.
- `jpeg_type` (String) The default type of JPEG output to use. This can be overridden with `format=bjpeg` and `format=pjpeg` on specific image optimizer requests. Valid values are `auto`, `baseline` and `progressive`. Default `auto`.
- `resize_filter` (String) The type of filter to use while resizing an image. Valid values are `lanczos3`, `lanczos2`, `bicubic`, `bilinear` and `nearest`. Default `lanczos3`.
- `upscale` (Boolean) Whether or not we should allow output images to render at sizes larger than input. Default `false`.
- `webp` (Boolean) Controls whether or not to default to WebP output when the client supports it. This is equivalent to adding `auto=webp` to all image optimizer requests. Default `false`.
- `webp_quality` (Number) The default quality to use with WebP output. This can be overridden with the second option in the `quality` URL parameter on specific image optimizer requests. Default `85`.

### Read-Only

- `id` (String) Terraform resource identifier.

## Import

Import Image Optimizer default settings using the service ID and version:

```shell
terraform import fastly_service_image_optimizer_default_settings.example SERVICE_ID/VERSION
```

Example:

```shell
terraform import fastly_service_image_optimizer_default_settings.example SU1Z0isxPaozGVKXdv0eY/3
```

## Destroy behavior

Destroying this resource resets all Image Optimizer default settings on the
configured service version to their API defaults rather than deleting them. If
Image Optimizer is no longer enabled on the service, the reset is skipped.
