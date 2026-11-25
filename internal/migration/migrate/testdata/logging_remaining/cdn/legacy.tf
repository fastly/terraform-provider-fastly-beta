variable "credential" { type = string }
variable "format" { type = string }

resource "fastly_service_vcl" "demo" {
  name = "migration-remaining-logging"
  force_destroy = true

  domain {
    name = "migration.example.invalid"
  }
  backend {
    name = "origin"
    address = "origin.example.invalid"
  }

  logging_datadog {
    name = "datadog"
    token = var.credential
    format = var.format
  }

  logging_ftp {
    name = "ftp"
    address = "ftp.example.invalid"
    user = "log-user"
    password = var.credential
    path = "logs/"
    format = var.format
  }

  logging_sftp {
    name = "sftp"
    address = "example.com"
    user = "username"
    secret_key = var.credential
    path = "logs/"
    ssh_known_hosts = "example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEmuNYPrqg9tjqfR14ye3Pvsm9sjIx6EJwm5tMXIMaN1"
    format = var.format
  }

  logging_loggly {
    name = "loggly"
    token = var.credential
    format = var.format
  }

  logging_grafanacloudlogs {
    name = "grafanacloudlogs"
    url = "https://logs.example.invalid/loki/api/v1/push"
    user = "12345"
    index = "{\"service\":\"test\"}"
    token = var.credential
    format = var.format
  }

  logging_scalyr {
    name = "scalyr"
    token = var.credential
    format = var.format
  }

  logging_newrelic {
    name = "newrelic"
    token = var.credential
    format = var.format
  }

  logging_newrelicotlp {
    name = "newrelicotlp"
    token = var.credential
    format = var.format
  }

  logging_heroku {
    name = "heroku"
    url = "https://heroku.example.invalid"
    token = var.credential
    format = var.format
  }

  logging_honeycomb {
    name = "honeycomb"
    dataset = "example"
    token = var.credential
    format = var.format
  }

  logging_logshuttle {
    name = "logshuttle"
    url = "https://logs.example.invalid"
    token = var.credential
    format = var.format
  }

  logging_papertrail {
    name = "papertrail"
    address = "logs.example.invalid"
    port = 514
    format = var.format
  }

  logging_sumologic {
    name = "sumologic"
    url = "https://logs.example.invalid/receiver"
    format = var.format
  }

  logging_fos {
    name = "fos"
    bucket_name = "migration-test-bucket"
    region = "us-east"
    path = "logs/"
    format = var.format
  }

}
