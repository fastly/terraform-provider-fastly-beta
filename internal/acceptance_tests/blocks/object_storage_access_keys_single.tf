resource "fastly_object_storage_access_keys" "test" {
  description = "{{.DESCRIPTION}}"
  permission  = "{{.PERMISSION}}"
  {{.BUCKETS}}
}
