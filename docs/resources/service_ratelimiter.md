---
page_title: "fastly_service_ratelimiter Resource - fastly"
subcategory: ""
description: |-
  Fastly service rate limiter resource. Writes directly to the specified writable service version.
---

# fastly_service_ratelimiter (Resource)

Fastly service rate limiter resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a rate limiter on the configured service version. It does not clone,
activate, or stage service versions. Rate limiters are only supported on VCL
services.

## Example Usage

```terraform
resource "fastly_service_ratelimiter" "login_throttle" {
  service_id           = fastly_service_cdn.example.id
  version              = 1
  name                 = "login_throttle"
  action               = "response"
  client_key           = ["req.http.Fastly-Client-IP"]
  http_methods         = ["POST"]
  penalty_box_duration = 10
  rps_limit            = 20
  window_size          = 10

  response = {
    content      = "Too many requests"
    content_type = "text/plain"
    status       = 429
  }
}
```

## Schema

### Required

- `action` (String) The action to take when a rate limiter violation is detected. One of `log_only`, `response`, or `response_object`.
- `client_key` (List of String) VCL variables used to generate a counter key to identify a client. Example: `["req.http.Fastly-Client-IP"]`.
- `http_methods` (List of String) HTTP methods to apply rate limiting to. Each method must be uppercase. Example: `["POST", "PUT", "PATCH", "DELETE"]`.
- `name` (String) A unique human readable name for the rate limiting rule.
- `penalty_box_duration` (Number) Length of time in minutes that the rate limiter is in effect after the initial violation is detected.
- `rps_limit` (Number) Upper limit of requests per second allowed by the rate limiter.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.
- `window_size` (Number) Number of seconds during which the RPS limit must be exceeded in order to trigger a violation. One of `1`, `10`, `60`.

### Optional

- `feature_revision` (Number) Revision number of the rate limiting feature implementation. Defaults to the most recent revision.
- `logger_type` (String) Name of the type of logging endpoint to be used when `action` is `log_only`. One of `azureblob`, `bigquery`, `cloudfiles`, `datadog`, `digitalocean`, `elasticsearch`, `ftp`, `gcs`, `googleanalytics`, `heroku`, `honeycomb`, `http`, `https`, `kafka`, `kinesis`, `logentries`, `loggly`, `logshuttle`, `newrelic`, `openstack`, `papertrail`, `pubsub`, `s3`, `scalyr`, `sftp`, `splunk`, `stackdriver`, `sumologic`, `syslog`.
- `response` (Attributes) Custom response to be sent when the rate limit is exceeded. Required if `action` is `response`. (see [below for nested schema](#nestedatt--response))
- `response_object_name` (String) Name of existing response object. Required if `action` is `response_object`.
- `uri_dictionary_name` (String) The name of an Edge Dictionary containing URIs as keys. If not defined or null, all origin URIs will be rate limited.

### Read-Only

- `id` (String) Terraform resource identifier.
- `rate_limiter_id` (String) Alphanumeric string identifying the rate limiter.

<a id="nestedatt--response"></a>
### Nested Schema for `response`

Required:

- `content` (String) HTTP response body data.
- `content_type` (String) HTTP Content-Type (e.g. `application/json`).
- `status` (Number) HTTP response status code (e.g. `429`).

## Import

Import a rate limiter using the service ID, version, and rate limiter name:

```shell
terraform import fastly_service_ratelimiter.login_throttle SERVICE_ID/VERSION/RATE_LIMITER_NAME
```

Example:

```shell
terraform import fastly_service_ratelimiter.login_throttle SU1Z0isxPaozGVKXdv0eY/3/login_throttle
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.

## Clearing response, response_object_name, or uri_dictionary_name

The Fastly API's update endpoint silently keeps the previous value in place if
`uri_dictionary_name`, `response_object_name`, or `response` is simply omitted
from a request. When any of these fields is removed from configuration, the
provider deletes and recreates the rate limiter on the same service version
instead of issuing an update, so the change actually takes effect.
