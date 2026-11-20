provider "fastly" {
  alias = "secondary"
}

resource "fastly_ngwaf_workspace" "main" {
  name        = "main"
  description = "Main workspace"
  mode        = "block"

  attack_signal_thresholds {
    one_minute  = 100
    ten_minutes = 500
    one_hour    = 2000
  }
}

resource "fastly_ngwaf_workspace_signal" "login" {
  workspace_id = fastly_ngwaf_workspace.main.id
  name         = "login failures"
}

# Blocked addresses shared by every workspace.
resource "fastly_ngwaf_ip_list" "blocked" {
  count       = 2
  name        = "blocked-${count.index}"
  description = "Blocked addresses"
  entries     = ["10.0.0.1"]
}

resource "fastly_ngwaf_workspace_string_list" "admins" {
  provider     = fastly.secondary
  workspace_id = fastly_ngwaf_workspace.main.id
  name         = "admins"
  entries      = ["admin"]
}

resource "fastly_ngwaf_workspace_signal_rule" "exclude_xss" {
  workspace_id    = fastly_ngwaf_workspace.main.id
  description     = "Exclude a false positive"
  enabled         = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/contact"
  }

  action {
    signal = "XSS"
  }
}

resource "fastly_ngwaf_workspace_templated_signal_rule" "login" {
  workspace_id = fastly_ngwaf_workspace.main.id
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  action {
    signal = "LOGINATTEMPT"
  }
}

resource "fastly_ngwaf_workspace_rate_limit_rule" "limit" {
  workspace_id = fastly_ngwaf_workspace.main.id
  description  = "Rate limit logins"
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  rate_limit {
    duration  = 300
    interval  = 60
    signal    = fastly_ngwaf_workspace_signal.login.reference_id
    threshold = 100

    client_identifiers {
      type = "ip"
    }
  }

  action {
    type   = "log_request"
    signal = fastly_ngwaf_workspace_signal.login.reference_id
  }

  depends_on = [fastly_ngwaf_workspace_signal_rule.exclude_xss]
}

resource "fastly_ngwaf_workspace_virtual_patch" "struts" {
  workspace_id     = fastly_ngwaf_workspace.main.id
  virtual_patch_id = "CVE-2017-5638"
  mode           = "block"
  enabled          = true

  lifecycle {
    ignore_changes = [mode]
  }
}

resource "fastly_ngwaf_workspace_alert_slack_integration" "ops" {
  workspace_id = fastly_ngwaf_workspace.main.id
  description  = "Ops channel"
  authentication = {
    webhook      = var.slack_webhook
  }
}

resource "fastly_ngwaf_workspace_alert_jira_integration" "security" {
  workspace_id = fastly_ngwaf_workspace.main.id
  host         = "https://example.atlassian.net"
  username     = "terraform"
  project      = "SEC"
  authentication = {
    key          = var.jira_key # rotated quarterly
  }
}

variable "slack_webhook" {
  type      = string
  sensitive = true
}

variable "jira_key" {
  type      = string
  sensitive = true
}
