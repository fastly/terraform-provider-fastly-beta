terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
    }
  }
}

provider "fastly" {
  # API token set via FASTLY_API_TOKEN environment variable
}

resource "fastly_ngwaf_workspace" "staging" {
  name        = "Example Staging Workspace"
  description = "Managed by Terraform"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace" "production" {
  name        = "Example Production Workspace"
  description = "Managed by Terraform"
  mode        = "block"

  attack_signal_thresholds {}
}

# A request rule applied to two named workspaces. applies_to is a request body
# field, so adding or removing a workspace updates the rule in place rather
# than replacing it.
resource "fastly_ngwaf_request_rule" "block_ip" {
  applies_to = [
    fastly_ngwaf_workspace.staging.id,
    fastly_ngwaf_workspace.production.id,
  ]
  description = "Block a specific IP"
  enabled     = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}

# The wildcard form applies a rule to every workspace in the account.
resource "fastly_ngwaf_request_rule" "block_legacy_protocol" {
  applies_to      = ["*"]
  description     = "Block requests using an obsolete HTTP protocol version"
  enabled         = true
  group_operator  = "all"
  request_logging = "sampled"

  condition {
    field    = "protocol_version"
    operator = "equals"
    value    = "HTTP/1.0"
  }

  # An account-scoped block carries no redirect_url or response_code.
  action {
    type = "block"
  }
}

# Two actions on one rule: tag the request with a signal and block it. A
# request rule accepts up to two actions.
resource "fastly_ngwaf_request_rule" "block_suspect_agents" {
  applies_to  = ["*"]
  description = "Tag and block requests from a suspicious user agent"
  enabled     = true

  condition {
    field    = "user_agent"
    operator = "contains"
    value    = "curl"
  }

  action {
    type   = "add_signal"
    signal = "SUSPECTED-BOT"
  }

  action {
    type = "block"
  }
}

# A request rule combining a group_condition and a multival_condition: block
# requests under /admin that do not carry an internal auth header.
resource "fastly_ngwaf_request_rule" "block_admin_without_header" {
  applies_to  = [fastly_ngwaf_workspace.production.id]
  description = "Block unauthenticated admin traffic"
  enabled     = true

  group_condition {
    group_operator = "all"

    condition {
      field    = "path"
      operator = "like"
      value    = "/admin*"
    }

    multival_condition {
      field          = "request_header"
      operator       = "does_not_exist"
      group_operator = "all"

      condition {
        field    = "name"
        operator = "equals"
        value    = "X-Internal-Auth"
      }
    }
  }

  action {
    type = "block"
  }
}

# A signal rule excludes a signal from requests matching its conditions,
# suppressing a known false positive across every workspace.
resource "fastly_ngwaf_signal_rule" "exclude_xss_on_contact_form" {
  applies_to  = ["*"]
  description = "Exclude an XSS false positive on the contact form"
  enabled     = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/contact-form"
  }

  action {
    signal = "XSS"
  }
}

# Reads back the rules defined at account scope. Each entry reports its own
# scope and the workspaces it applies to.
data "fastly_ngwaf_rules" "all" {
  depends_on = [
    fastly_ngwaf_request_rule.block_ip,
    fastly_ngwaf_signal_rule.exclude_xss_on_contact_form,
  ]
}

output "account_rule_ids" {
  description = "IDs of the rules defined at account scope."
  value       = [for rule in data.fastly_ngwaf_rules.all.rules : rule.id]
}
