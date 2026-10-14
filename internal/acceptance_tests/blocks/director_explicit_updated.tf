resource "fastly_service_backend" "origin" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.BACKEND_NAME}}"
  address    = "api.example.com"
  port       = 443
  use_ssl    = true
}

resource "fastly_service_director" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.DIRECTOR_NAME}}"
  backends   = [fastly_service_backend.origin.name]
  comment    = "updated director"
  quorum     = 30
  retries    = 10
  shield     = "sjc-ca-us"
  type       = "hash"
}
