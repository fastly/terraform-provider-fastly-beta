# Compare the exact persisted endpoint configuration. Only representation
# changes with known equivalent semantics are normalized; credentials are
# never discarded, hashed, or printed by the calling script.
def endpoints: ["logging_https", "logging_elasticsearch", "logging_kafka", "logging_splunk", "logging_syslog"];

def flatten($block):
  if $type == "fastly_service_cdn_auto" then
    .tls as $tls |
    if ($tls | type) != "object" then error("missing refreshed TLS object")
    else del(.tls) + {
      tls_ca_cert: $tls.ca_cert, tls_client_cert: $tls.client_cert,
      tls_client_key: $tls.client_key, tls_hostname: $tls.hostname
    } end |
    if $block == "logging_https" then .
    else .authentication as $auth |
      if ($auth | type) != "object" then error("missing refreshed authentication object")
      else del(.authentication) + $auth end
    end |
    if $block == "logging_splunk" then
      if .request_max_bytes != 0 or .request_max_entries != 0 then
        error("target-only Splunk request limits are nonzero; not safe to compare")
      else del(.request_max_bytes, .request_max_entries) end
    else . end
  else . end |
  # SDKv2 uses an empty string for API auto-placement; Framework uses null.
  .placement = (.placement // "");

[.resources[] | select(.type == $type and .name == "demo") | .instances[].attributes] |
if length != 1 then error("expected exactly one migrated test service") else .[0] end |
. as $state |
{ id: .id, active_version: .active_version, logging:
  (reduce endpoints[] as $block ({};
    .[$block] = (($state[$block] // []) | map(flatten($block)) | sort_by(.name))))
} |
if ([.logging[] | length] | add) == 0 then error("no selected logging endpoints in state") else . end
