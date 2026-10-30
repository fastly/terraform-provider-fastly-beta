terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly-beta"
    }
  }
}

provider "fastly" {
  # API token set via FASTLY_API_TOKEN environment variable
}

resource "fastly_integration" "slack" {
  name = "audit-log-alerts"
  type = "slack"

  authentication = {
    webhook = "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"
  }
}

# Notify on account-level audit events. `scope_ids` must be empty for the
# `account` scope type.
resource "fastly_audit_log_event_mapping" "account" {
  name            = "account audit events"
  description     = "Send address changes to Slack"
  scope_type      = "account"
  event_types     = ["address.create", "address.update", "address.delete"]
  integration_ids = [fastly_integration.slack.id]
}

resource "fastly_service_cdn_auto" "example" {
  name          = "example-service"
  force_destroy = true

  domain {
    name = "example.com"
  }

  backend {
    name    = "example-backend"
    address = "example.org"
  }
}

# Scope a mapping to specific services of a given type.
resource "fastly_audit_log_event_mapping" "service" {
  name            = "example service backend changes"
  scope_type      = "vcl"
  scope_ids       = [fastly_service_cdn_auto.example.id]
  event_types     = ["backend.delete"]
  integration_ids = [fastly_integration.slack.id]
}

# Look up an existing mapping by ID...
data "fastly_audit_log_event_mapping" "by_id" {
  id = fastly_audit_log_event_mapping.account.id
}

# ...or by filters, which must match exactly one mapping.
data "fastly_audit_log_event_mapping" "by_name" {
  name       = "example service backend changes"
  scope_type = "vcl"
  depends_on = [fastly_audit_log_event_mapping.service]
}
