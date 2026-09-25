resource "fastly_service_gzip" "test" {
  service_id    = fastly_service_cdn.test.id
  version       = {{.SERVICE_VERSION}}
  name          = "{{.GZIP_NAME}}"
  content_types = ["text/html", "text/css", "application/json"]
  extensions    = ["css", "js", "json"]
}
