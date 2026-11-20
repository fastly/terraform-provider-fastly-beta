output "blocked_list_ids" {
  value = fastly_ngwaf_account_list.blocked[*].id
}

output "admin_list" {
  value = "${fastly_ngwaf_workspace_list.admins.name}:${fastly_ngwaf_workspace_list.admins.id}"
}

output "patch_mode" {
  value = fastly_ngwaf_virtual_patches.struts.action
}

output "slack_webhook" {
  value     = fastly_ngwaf_alert_slack_integration.ops.webhook
  sensitive = true
}

output "rule_ids" {
  value = [fastly_ngwaf_workspace_rule.exclude_xss.id, fastly_ngwaf_workspace_rule.login.id]
}
