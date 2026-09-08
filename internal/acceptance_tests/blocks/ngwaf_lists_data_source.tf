resource "fastly_ngwaf_ip_list" "ip" {
  name        = "{{.IP_LIST_NAME}}"
  description = "IP list"

  entries = ["10.0.0.1"]
}

resource "fastly_ngwaf_string_list" "string" {
  name        = "{{.STRING_LIST_NAME}}"
  description = "String list"

  entries = ["admin"]
}

resource "fastly_ngwaf_wildcard_list" "wildcard" {
  name        = "{{.WILDCARD_LIST_NAME}}"
  description = "Wildcard list"

  entries = ["admin-*"]
}

resource "fastly_ngwaf_country_list" "country" {
  name        = "{{.COUNTRY_LIST_NAME}}"
  description = "Country list"

  entries = ["US"]
}

resource "fastly_ngwaf_signal_list" "signal" {
  name        = "{{.SIGNAL_LIST_NAME}}"
  description = "Signal list"

  entries = ["XSS"]
}

data "fastly_ngwaf_lists" "test" {
  depends_on = [
    fastly_ngwaf_ip_list.ip,
    fastly_ngwaf_string_list.string,
    fastly_ngwaf_wildcard_list.wildcard,
    fastly_ngwaf_country_list.country,
    fastly_ngwaf_signal_list.signal,
  ]
}
