variable "credential" { type = string }
variable "format" { type = string }

resource "fastly_service_cdn_auto" "demo" {
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
    authentication = {
      token = var.credential
    }
    format = var.format
  }

  logging_ftp {
    name = "ftp"
    address = "ftp.example.invalid"
    authentication = {
      user = "log-user"
      password = var.credential
    }
    path = "logs/"
    format = var.format
  }

  logging_sftp {
    name = "sftp"
    address = "example.com"
    authentication = {
      user = "username"
      secret_key = var.credential
    }
    path = "logs/"
    ssh_known_hosts = "example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEmuNYPrqg9tjqfR14ye3Pvsm9sjIx6EJwm5tMXIMaN1"
    format = var.format
  }

  logging_loggly {
    name = "loggly"
    authentication = {
      token = var.credential
    }
    format = var.format
  }

  logging_grafanacloudlogs {
    name = "grafanacloudlogs"
    url = "https://logs.example.invalid/loki/api/v1/push"
    user = "12345"
    index = "{\"service\":\"test\"}"
    authentication = {
      token = var.credential
    }
    format = var.format
  }

  logging_scalyr {
    name = "scalyr"
    authentication = {
      token = var.credential
    }
    format = var.format
  }

  logging_newrelic {
    name = "newrelic"
    authentication = {
      token = var.credential
    }
    format = var.format
  }

  logging_newrelicotlp {
    name = "newrelicotlp"
    authentication = {
      token = var.credential
    }
    format = var.format
  }

  logging_heroku {
    name = "heroku"
    url = "https://heroku.example.invalid"
    authentication = {
      token = var.credential
    }
    format = var.format
  }

  logging_honeycomb {
    name = "honeycomb"
    dataset = "example"
    authentication = {
      token = var.credential
    }
    format = var.format
  }

  logging_logshuttle {
    name = "logshuttle"
    url = "https://logs.example.invalid"
    authentication = {
      token = var.credential
    }
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
    message_type = "classic"
  }

  logging_fos {
    name = "fos"
    bucket_name = "migration-test-bucket"
    region = "us-east"
    path = "logs/"
    format = var.format
  }

}
