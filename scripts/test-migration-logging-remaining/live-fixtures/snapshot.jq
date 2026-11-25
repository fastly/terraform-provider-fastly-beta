[.values.root_module.resources[]?
   | select(.address==$address and .values.id==$id)] |
  if length != 1 then error("expected exactly one managed service") else .[0].values end |
  {id, active_version, endpoint:
    [(.[$endpoint] // [])[] |
      if $endpoint == "logging_fos" then
        {name, bucket_name, region, path, format, format_version, message_type}
      else
        {name, address, port, format, format_version}
      end
    ] | sort_by(.name)}
