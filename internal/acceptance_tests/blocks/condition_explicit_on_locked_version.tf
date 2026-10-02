resource "fastly_service_condition" "locked" {
  service_id = fastly_service_cdn.test.id
  version    = 2
  name       = "{{.CONDITION_NAME}}"
  type       = "REQUEST"
  statement  = "req.url ~ \"^/admin\""
}
