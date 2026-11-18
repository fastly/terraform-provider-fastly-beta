logging_fos {
  name              = "{{.LOGGING_FOS_NAME}}"
  bucket_name       = "test-fos-bucket"
  region            = "eu-central"
  period            = 600
  compression_codec = "zstd"
}
