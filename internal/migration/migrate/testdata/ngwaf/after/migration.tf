terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = ">= 20.0.0"
    }
  }
}

moved {
  from = fastly_ngwaf_account_list.blocked
  to   = fastly_ngwaf_ip_list.blocked
}

moved {
  from = fastly_ngwaf_alert_jira_integration.security
  to   = fastly_ngwaf_workspace_alert_jira_integration.security
}

moved {
  from = fastly_ngwaf_alert_slack_integration.ops
  to   = fastly_ngwaf_workspace_alert_slack_integration.ops
}

moved {
  from = fastly_ngwaf_virtual_patches.struts
  to   = fastly_ngwaf_workspace_virtual_patch.struts
}

moved {
  from = fastly_ngwaf_workspace_list.admins
  to   = fastly_ngwaf_workspace_string_list.admins
}

moved {
  from = fastly_ngwaf_workspace_rule.exclude_xss
  to   = fastly_ngwaf_workspace_signal_rule.exclude_xss
}

moved {
  from = fastly_ngwaf_workspace_rule.limit
  to   = fastly_ngwaf_workspace_rate_limit_rule.limit
}

moved {
  from = fastly_ngwaf_workspace_rule.login
  to   = fastly_ngwaf_workspace_templated_signal_rule.login
}
