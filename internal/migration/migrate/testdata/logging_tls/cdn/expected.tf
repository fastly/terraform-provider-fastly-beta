variable "cert" { type = string }
variable "key" { type = string }
variable "ca" { type = string }
variable "password" { type = string }
variable "token" { type = string }
variable "format" { type = string }
variable "hostname" { type = string }

locals {
  tls_hostname = var.hostname
  response_name = "log-response"
  header = "SYNTHETIC_HEADER_MARKER"
}

resource "fastly_service_cdn_auto" "demo" {
  name = "migration-test"
  force_destroy = true

  domain {
    name = "migration.example.invalid"
  }

  backend {
    name = "origin"
    address = "origin.example.invalid"
  }
  condition {
    name = local.response_name
    type = "RESPONSE"
    statement = "resp.status == 200"
    priority = 10
  }

  logging_https {
    name = "https"
    url = "https://logs.example.invalid/"
    header_name = "Authorization"
    header_value = local.header
    gzip_level = 0
    request_max_bytes = 4096
    request_max_entries = 50
    tls = {
      ca_cert = trimspace(file(var.ca))
      client_cert = trimspace(file(var.cert))
      client_key = trimspace(file(var.key))
      hostname = local.tls_hostname
    }
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_https {
    name = "https-codec"
    url = "https://logs.example.invalid/other"
    compression_codec = "gzip"
    tls = {
      hostname = var.hostname # keep the hostname comment
    }
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_elasticsearch {
    name = "elasticsearch"
    url = "https://search.example.invalid/"
    index = "migration-test"
    authentication = {
      user = "test-user"
      password = var.password
    }
    pipeline = "migration-pipeline"
    tls = {
      ca_cert = trimspace(file(var.ca))
      client_cert = trimspace(file(var.cert))
      client_key = trimspace(file(var.key))
      hostname = var.hostname
    }
    request_max_bytes = 4096
    request_max_entries = 50
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_kafka {
    name = "kafka"
    brokers = "kafka.example.invalid:9093"
    topic = "migration"
    auth_method = "scram-sha-256"
    authentication = {
      user = "migration-user"
      password = var.password
    }
    use_tls = true
    required_acks = "1"
    parse_log_keyvals = true
    tls = {
      ca_cert = trimspace(file(var.ca))
      client_cert = trimspace(file(var.cert))
      client_key = trimspace(file(var.key))
      hostname = var.hostname
    }
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_splunk {
    name = "splunk"
    url = "https://splunk.example.invalid:8088/services/collector"
    use_tls = true
    authentication = {
      token = var.token
    }
    tls = {
      ca_cert = trimspace(file(var.ca))
      client_cert = trimspace(file(var.cert))
      client_key = trimspace(file(var.key))
      hostname = var.hostname
    }
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_syslog {
    name = "syslog"
    address = "syslog.example.invalid"
    port = 6514
    use_tls = true
    authentication = {
      token = var.token
    }
    tls = {
      ca_cert = trimspace(file(var.ca))
      client_cert = trimspace(file(var.cert))
      client_key = trimspace(file(var.key))
      hostname = var.hostname
    }
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_syslog {
    name = "syslog-plain"
    address = "syslog.example.invalid"
    port = 514
    format = var.format
    format_version = 2
    placement = "none"
  }
}

output "service_id" {
  value = fastly_service_cdn_auto.demo.id
}
