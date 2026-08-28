resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_ip_list" "ip" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.IP_LIST_NAME}}"
  description  = "IP list"

  entries = ["10.0.0.1"]
}

resource "fastly_ngwaf_workspace_string_list" "string" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.STRING_LIST_NAME}}"
  description  = "String list"

  entries = ["admin"]
}

resource "fastly_ngwaf_workspace_wildcard_list" "wildcard" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.WILDCARD_LIST_NAME}}"
  description  = "Wildcard list"

  entries = ["admin-*"]
}

resource "fastly_ngwaf_workspace_country_list" "country" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.COUNTRY_LIST_NAME}}"
  description  = "Country list"

  entries = ["US"]
}

resource "fastly_ngwaf_workspace_signal_list" "signal" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.SIGNAL_LIST_NAME}}"
  description  = "Signal list"

  entries = ["XSS"]
}

data "fastly_ngwaf_workspace_lists" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [
    fastly_ngwaf_workspace_ip_list.ip,
    fastly_ngwaf_workspace_string_list.string,
    fastly_ngwaf_workspace_wildcard_list.wildcard,
    fastly_ngwaf_workspace_country_list.country,
    fastly_ngwaf_workspace_signal_list.signal,
  ]
}
