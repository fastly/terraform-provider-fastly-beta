output "blocked_list_ids" {
  value = fastly_ngwaf_ip_list.blocked[*].id
}

output "admin_list" {
  value = "${fastly_ngwaf_workspace_string_list.admins.name}:${fastly_ngwaf_workspace_string_list.admins.id}"
}

output "patch_mode" {
  value = fastly_ngwaf_workspace_virtual_patch.struts.mode
}

output "slack_webhook" {
  value     = fastly_ngwaf_workspace_alert_slack_integration.ops.authentication.webhook
  sensitive = true
}

output "rule_ids" {
  value = [fastly_ngwaf_workspace_signal_rule.exclude_xss.id, fastly_ngwaf_workspace_templated_signal_rule.login.id]
}
