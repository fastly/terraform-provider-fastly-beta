variable "secret" { type = string }
variable "email" { type = string }
variable "cert" { type = string }
variable "key" { type = string }
variable "ca" { type = string }
variable "password" { type = string }
variable "token" { type = string }
variable "format" { type = string }
variable "hostname" { type = string }
variable "package_filename" { type = string }
variable "package_hash" { type = string }

locals {
  storage_bucket = "logs-bucket"
  tls_hostname = var.hostname
  response_name = "log-response"
  header = "SYNTHETIC_HEADER_MARKER"
}

resource "fastly_service_compute_auto" "demo" {
  name = "migration-test"
  force_destroy = true

  domain {
    name = "migration.example.invalid"
  }

  backend {
    name = "origin"
    address = "origin.example.invalid"
  }

  package {
    filename = var.package_filename
    source_code_hash = var.package_hash
  }

  logging_s3 {
    name = "s3"
    bucket_name = local.storage_bucket
    authentication = {
      access_key = var.key
      secret_key = trimspace(var.secret)
    }
    path = format("prefix/%s/", local.storage_bucket)
    server_side_encryption = "AES256"
    message_type = "classic"
  }

  logging_gcs {
    name = "gcs"
    bucket_name = "logs-bucket"
    authentication = {
      email = var.email
      secret_key = var.secret
    }
    path = "logs/"
  }

  logging_bigquery {
    name = "bigquery"
    project_id = "project-123"
    dataset = "logs"
    table = "events"
    authentication = {
      email = var.email
      secret_key = var.secret
    }
    template = "source"
  }

  logging_blobstorage {
    name = "blob"
    container = "logs"
    authentication = {
      account_name = "storage-account"
      sas_token = var.secret
    }
    path = "logs/"
  }

  logging_cloudfiles {
    name = "cloudfiles"
    bucket_name = "logs"
    authentication = {
      user = var.email
      access_key = var.secret
    }
    region = "DFW"
    message_type = "classic"
  }

  logging_digitalocean {
    name = "spaces"
    bucket_name = "logs"
    authentication = {
      access_key = var.key
      secret_key = var.secret
    }
    domain = "example.invalid"
  }

  logging_openstack {
    name = "openstack"
    bucket_name = "logs"
    url = "https://storage.example.invalid"
    authentication = {
      user = var.email
      access_key = var.secret
    }
  }

  logging_googlepubsub {
    name = "pubsub"
    project_id = "project-123"
    topic = "events"
    authentication = {
      email = var.email
      secret_key = var.secret
    }
  }

  logging_kinesis {
    name = "kinesis"
    topic = "stream"
    region = "us-east-1"
    authentication = {
      access_key = var.key
      secret_key = var.secret
    }
  }


}

output "service_id" {
  value = fastly_service_compute_auto.demo.id
}
