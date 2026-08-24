  condition {
    name      = "{{.CONDITION_NAME}}"
    type      = "RESPONSE"
    statement = "resp.status == 200"
  }

  logging_s3 {
    name               = "{{.LOGGING_S3_NAME}}"
    bucket_name        = "{{.BUCKET_NAME}}"
    response_condition = "{{.CONDITION_NAME}}"
    authentication = {
      access_key = "AKIAIOSFODNN7EXAMPLE"
      secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    }
  }
