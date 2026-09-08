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

resource "fastly_ngwaf_workspace" "example" {
  name        = "Example Workspace"
  description = "Managed by Terraform"
  mode        = "block"

  attack_signal_thresholds {}
}

# A simple request rule: block a specific IP.
resource "fastly_ngwaf_workspace_request_rule" "block_ip" {
  workspace_id = fastly_ngwaf_workspace.example.id
  description  = "Block a specific IP"
  enabled      = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}

# A request rule combining a group_condition and a multival_condition:
# block requests under /admin that don't carry an internal auth header.
resource "fastly_ngwaf_workspace_request_rule" "block_admin_without_header" {
  workspace_id = fastly_ngwaf_workspace.example.id
  description  = "Block admin paths hit without the expected header"
  enabled      = true

  group_condition {
    group_operator = "all"

    condition {
      field    = "path"
      operator = "contains"
      value    = "/admin"
    }

    multival_condition {
      field          = "request_header"
      operator       = "does_not_exist"
      group_operator = "any"

      condition {
        field    = "name"
        operator = "equals"
        value    = "X-Internal-Token"
      }
    }
  }

  action {
    type = "block"
  }
}

# A request rule using a standalone multival_condition, declared at the
# rule's top level rather than nested inside a group_condition.
resource "fastly_ngwaf_workspace_request_rule" "debug_query_param" {
  workspace_id = fastly_ngwaf_workspace.example.id
  description  = "Block requests carrying a debug query parameter"
  enabled      = true

  multival_condition {
    field          = "query_parameter"
    operator       = "exists"
    group_operator = "any"

    condition {
      field    = "name"
      operator = "equals"
      value    = "debug"
    }
  }

  action {
    type = "block"
  }
}

# A signal rule: exclude a signal that's producing a false positive on a
# known-safe path. The action carries only the signal to exclude.
resource "fastly_ngwaf_workspace_signal_rule" "exclude_xss_signal" {
  workspace_id = fastly_ngwaf_workspace.example.id
  description  = "Exclude XSS signal to address a false positive"
  enabled      = true

  condition {
    field    = "path"
    operator = "like"
    value    = "/contact-form"
  }

  action {
    signal = "XSS"
  }
}

# A rate limit rule: block a client once it exceeds a request threshold.
# `rate_limit` references a custom signal by its reference ID; this repo has
# no fastly_ngwaf_workspace_signal resource yet, so the reference below is a
# placeholder for a signal created out of band. client_identifiers combines
# two identifiers here (ip + request_cookie) to rate limit per session
# within an IP.
resource "fastly_ngwaf_workspace_rate_limit_rule" "ip_rate_limit" {
  workspace_id = fastly_ngwaf_workspace.example.id
  description  = "Rate limit requests from a single IP"
  enabled      = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "1.2.3.4"
  }

  rate_limit {
    signal    = "site.demo" # replace with a real custom signal's reference ID
    threshold = 100
    interval  = 60
    duration  = 300

    client_identifiers {
      type = "ip"
    }

    client_identifiers {
      type = "request_cookie"
      name = "session_id"
    }
  }

  action {
    type   = "block_signal"
    signal = "SUSPECTED-BOT"
  }
}

# A templated signal rule: attaches a Fastly-defined signal to matching
# requests, here derived from another signal rather than from request
# attributes alone - tag a request INVITE-FAILURE when it already carries
# INVITE-ATTEMPT and came back 404. Every attribute forces replacement, and
# the resource takes no description.
resource "fastly_ngwaf_workspace_templated_signal_rule" "invite_failure" {
  workspace_id = fastly_ngwaf_workspace.example.id
  enabled      = true

  condition {
    field    = "response_code"
    operator = "equals"
    value    = "404"
  }

  multival_condition {
    field          = "signal"
    operator       = "exists"
    group_operator = "all"

    condition {
      field    = "signal_id"
      operator = "equals"
      value    = "INVITE-ATTEMPT"
    }
  }

  action {
    signal = "INVITE-FAILURE"
  }
}

data "fastly_ngwaf_workspace_rules" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
}

output "ngwaf_workspace_rules" {
  value = data.fastly_ngwaf_workspace_rules.example.rules
}
