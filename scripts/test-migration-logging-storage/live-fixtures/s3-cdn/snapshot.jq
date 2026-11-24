[.values.root_module.resources[]?
   | select(.address==$address and .values.id==$id)] |
  if length != 1 then error("expected one managed service") else .[0].values end |
  {id, active_version, logging_s3:
    [(.logging_s3 // [])[] | {
      name, bucket_name, path, format, format_version,
      acl, compression_codec, domain, file_max_bytes, gzip_level,
      message_type, period, processing_region, public_key, redundancy,
      server_side_encryption, server_side_encryption_kms_key_id, timestamp_format,
      access_key: (if $is_legacy then .s3_access_key else .authentication.access_key end),
      secret_key: (if $is_legacy then .s3_secret_key else .authentication.secret_key end)
    }] | sort_by(.name)}
