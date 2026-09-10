resource "fastly_user_service_authorization" "test" {
  service_id = {{.SERVICE_ID_REF}}
  user_id    = "{{.USER_ID}}"
  permission = "{{.PERMISSION}}"
}
