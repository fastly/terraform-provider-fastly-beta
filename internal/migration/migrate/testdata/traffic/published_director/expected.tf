resource "fastly_service_cdn_auto" "demo" {
  name = "demofastly"

  backend {
    address = "127.0.0.1"
    name    = "origin1"
    port    = 80
  }

  backend {
    address = "127.0.0.2"
    name    = "origin2"
    port    = 80
  }

  director {
    name = "mydirector"
    quorum = 0
    type = "hash"
    backends = [ "origin1", "origin2" ]
  }

  force_destroy = true
}

resource "fastly_domain" "demo" {
  fqdn        = "demo.notexample.com"
  description = "demo"
  service_id  = fastly_service_cdn_auto.demo.id
}
