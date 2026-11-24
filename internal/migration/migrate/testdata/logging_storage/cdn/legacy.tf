variable "secret" { type = string }
variable "email" { type = string }
variable "cert" { type = string }
variable "key" { type = string }
variable "ca" { type = string }
variable "password" { type = string }
variable "token" { type = string }
variable "format" { type = string }
variable "hostname" { type = string }

locals {
  storage_bucket = "logs-bucket"
  tls_hostname = var.hostname
  response_name = "log-response"
  header = "SYNTHETIC_HEADER_MARKER"
}

resource "fastly_service_vcl" "demo" {
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

  logging_s3 {
    name = "s3"
    bucket_name = local.storage_bucket
    s3_access_key = var.key
    s3_secret_key = trimspace(var.secret)
    path = format("prefix/%s/", local.storage_bucket)
    server_side_encryption = "AES256"
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_gcs {
    name = "gcs"
    bucket_name = "logs-bucket"
    user = var.email
    secret_key = var.secret
    path = "logs/"
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_bigquery {
    name = "bigquery"
    project_id = "project-123"
    dataset = "logs"
    table = "events"
    email = var.email
    secret_key = var.secret
    template = "source"
    format = var.format
    response_condition = local.response_name
  }

  logging_blobstorage {
    name = "blob"
    container = "logs"
    account_name = "storage-account"
    sas_token = var.secret
    path = "logs/"
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_cloudfiles {
    name = "cloudfiles"
    bucket_name = "logs"
    user = var.email
    access_key = var.secret
    region = "DFW"
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_digitalocean {
    name = "spaces"
    bucket_name = "logs"
    access_key = var.key
    secret_key = var.secret
    domain = "example.invalid"
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_openstack {
    name = "openstack"
    bucket_name = "logs"
    url = "https://storage.example.invalid"
    user = var.email
    access_key = var.secret
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_googlepubsub {
    name = "pubsub"
    project_id = "project-123"
    topic = "events"
    user = var.email
    secret_key = var.secret
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }

  logging_kinesis {
    name = "kinesis"
    topic = "stream"
    region = "us-east-1"
    access_key = var.key
    secret_key = var.secret
    format = var.format
    format_version = 2
    response_condition = local.response_name
  }


}

output "service_id" {
  value = fastly_service_vcl.demo.id
}
