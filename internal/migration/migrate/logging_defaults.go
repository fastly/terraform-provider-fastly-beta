package migrate

// The SDKv2 VCL defaults have different whitespace from the Framework
// defaults. Fastly treats format as a string, not parsed JSON. Preserve the
// exact former format for omitted fields instead of relying on the new
// provider default. These literals match the legacy SDKv2 provider sources
// in github.com/fastly/terraform-provider-fastly/fastly/constants.go:
// LoggingHTTPSDefaultFormat and LoggingSplunkDefaultFormat. The HTTPS,
// Elasticsearch, Kafka and Syslog defaults use the same source value, as
// do the reviewed non-BigQuery storage/cloud logging blocks.
func legacyVCLLoggingFormat(block string) string {
	switch block {
	case "logging_ftp":
		return legacyFTPLoggingFormat
	case "logging_datadog":
		return legacyDatadogLoggingFormat
	case "logging_newrelic":
		return legacyNewRelicLoggingFormat
	case "logging_newrelicotlp":
		return legacyNewRelicOTLPLoggingFormat
	case "logging_honeycomb":
		return legacyHoneycombLoggingFormat
	}
	if block == "logging_splunk" {
		return legacySplunkFormat
	}
	if block == "logging_bigquery" {
		return legacyBigQueryLoggingFormat
	}
	return legacyJSONLoggingFormat
}

// These SDKv2 block defaults are "classic" in both the VCL and Compute
// schemas (fastly/block_fastly_service_logging_{s3,cloudfiles}.go).
// The target provider defaults both to "blank". An omitted or literal-null
// message_type must therefore be pinned when migrating either service family.
// Other storage endpoints already have matching defaults or no message_type.
var legacyLoggingMessageTypeDefaults = map[string]string{
	"logging_s3":         "classic",
	"logging_cloudfiles": "classic",
	"logging_sumologic":  "classic",
}

const legacyJSONLoggingFormat = `{
    "timestamp": "%{strftime(\{"%Y-%m-%dT%H:%M:%S%z"\}, time.start)}V",
    "client_ip": "%{req.http.Fastly-Client-IP}V",
    "geo_country": "%{client.geo.country_name}V",
    "geo_city": "%{client.geo.city}V",
    "host": "%{if(req.http.Fastly-Orig-Host, req.http.Fastly-Orig-Host, req.http.Host)}V",
    "url": "%{json.escape(req.url)}V",
    "request_method": "%{json.escape(req.method)}V",
    "request_protocol": "%{json.escape(req.proto)}V",
    "request_referer": "%{json.escape(req.http.referer)}V",
    "request_user_agent": "%{json.escape(req.http.User-Agent)}V",
    "response_state": "%{json.escape(fastly_info.state)}V",
    "response_status": %{resp.status}V,
    "response_reason": %{if(resp.response, "%22"+json.escape(resp.response)+"%22", "null")}V,
    "response_body_size": %{resp.body_bytes_written}V,
    "fastly_server": "%{json.escape(server.identity)}V",
    "fastly_is_edge": %{if(fastly.ff.visits_this_service == 0, "true", "false")}V
  }`

// Exact SDKv2 LoggingBigQueryDefaultFormat from fastly/constants.go.
// Unlike the shared JSON format, the timestamp omits %z.
const legacyBigQueryLoggingFormat = `{
    "timestamp": "%{strftime(\{"%Y-%m-%dT%H:%M:%S"\}, time.start)}V",
    "client_ip": "%{req.http.Fastly-Client-IP}V",
    "geo_country": "%{client.geo.country_name}V",
    "geo_city": "%{client.geo.city}V",
    "host": "%{if(req.http.Fastly-Orig-Host, req.http.Fastly-Orig-Host, req.http.Host)}V",
    "url": "%{json.escape(req.url)}V",
    "request_method": "%{json.escape(req.method)}V",
    "request_protocol": "%{json.escape(req.proto)}V",
    "request_referer": "%{json.escape(req.http.referer)}V",
    "request_user_agent": "%{json.escape(req.http.User-Agent)}V",
    "response_state": "%{json.escape(fastly_info.state)}V",
    "response_status": %{resp.status}V,
    "response_reason": %{if(resp.response, "%22"+json.escape(resp.response)+"%22", "null")}V,
    "response_body_size": %{resp.body_bytes_written}V,
    "fastly_server": "%{json.escape(server.identity)}V",
    "fastly_is_edge": %{if(fastly.ff.visits_this_service == 0, "true", "false")}V
  }`

const legacySplunkFormat = `{
    "time":%{time.start.sec}V,
    "host":"%{Fastly-Orig-Host}i",
    "event": {
      "service_id":"%{req.service_id}V",
      "time_start":"%{begin:%Y-%m-%dT%H:%M:%S%Z}t",
      "time_end":"%{end:%Y-%m-%dT%H:%M:%S%Z}t",
      "time_elapsed":%D,
      "client_ip":"%h",
      "client_as_name":"%{client.as.name}V",
      "client_as_number":"%{client.as.number}V",
      "client_connection_speed":"%{client.geo.conn_speed}V",
      "request":"%m",
      "protocol":"%H",
      "origin_host":"%v",
      "url":"%{json.escape(req.url)}V",
      "is_ipv6":%{if(req.is_ipv6, "true", "false")}V,
      "is_tls":%{if(req.is_ssl, "true", "false")}V,
      "tls_client_protocol":"%{json.escape(tls.client.protocol)}V",
      "tls_client_servername":"%{json.escape(tls.client.servername)}V",
      "tls_client_cipher":"%{json.escape(tls.client.cipher)}V",
      "tls_client_cipher_sha":"%{json.escape(tls.client.ciphers_sha )}V",
      "tls_client_tlsexts_sha":"%{json.escape(tls.client.tlsexts_sha)}V",
      "is_h2":%{if(fastly_info.is_h2, "true", "false")}V,
      "is_h2_push":%{if(fastly_info.h2.is_push, "true", "false")}V,
      "h2_stream_id":"%{fastly_info.h2.stream_id}V",
      "request_referer":"%{Referer}i",
      "request_user_agent":"%{User-Agent}i",
      "request_accept_content":"%{Accept}i",
      "request_accept_language":"%{Accept-Language}i",
      "request_accept_encoding":"%{Accept-Encoding}i",
      "request_accept_charset":"%{Accept-Charset}i",
      "request_connection":"%{Connection}i",
      "request_dnt":"%{DNT}i",
      "request_forwarded":"%{Forwarded}i",
      "request_via":"%{Via}i",
      "request_cache_control":"%{Cache-Control}i",
      "request_x_requested_with":"%{X-Requested-With}i",
      "request_x_att_device_id":"%{X-ATT-Device-Id}i",
      "request_x_forwarded_for":"%{X-Forwarded-For}i",
      "status":"%s",
      "content_type":"%{Content-Type}o",
      "cache_status":"%{regsub(fastly_info.state, "^(HIT-(SYNTH)|(HITPASS|HIT|MISS|PASS|ERROR|PIPE)).*", "\\2\\3")}V",
      "is_cacheable":%{if(fastly_info.state ~"^(HIT|MISS)$", "true", "false")}V,
      "response_age":"%{Age}o",
      "response_cache_control":"%{Cache-Control}o",
      "response_expires":"%{Expires}o",
      "response_last_modified":"%{Last-Modified}o",
      "response_tsv":"%{TSV}o",
      "server_datacenter":"%{server.datacenter}V",
      "server_ip":"%A",
      "geo_city":"%{client.geo.city.utf8}V",
      "geo_country_code":"%{client.geo.country_code}V",
      "geo_continent_code":"%{client.geo.continent_code}V",
      "geo_region":"%{client.geo.region}V",
      "req_header_size":%{req.header_bytes_read}V,
      "req_body_size":%{req.body_bytes_read}V,
      "resp_header_size":%{resp.header_bytes_written}V,
      "resp_body_size":%B,
      "socket_cwnd":%{client.socket.cwnd}V,
      "socket_nexthop":"%{client.socket.nexthop}V",
      "socket_tcpi_rcv_mss":%{client.socket.tcpi_rcv_mss}V,
      "socket_tcpi_snd_mss":%{client.socket.tcpi_snd_mss}V,
      "socket_tcpi_rtt":%{client.socket.tcpi_rtt}V,
      "socket_tcpi_rttvar":%{client.socket.tcpi_rttvar}V,
      "socket_tcpi_rcv_rtt":%{client.socket.tcpi_rcv_rtt}V,
      "socket_tcpi_rcv_space":%{client.socket.tcpi_rcv_space}V,
      "socket_tcpi_last_data_sent":%{client.socket.tcpi_last_data_sent}V,
      "socket_tcpi_total_retrans":%{client.socket.tcpi_total_retrans}V,
      "socket_tcpi_delta_retrans":%{client.socket.tcpi_delta_retrans}V,
      "socket_ploss":%{client.socket.ploss}V
    }
  }`

// Byte-for-byte SDKv2 legacy defaults from fastly/constants.go.
// These differ from Framework endpoint defaults and must be pinned.
const legacyFTPLoggingFormat = `{
  "timestamp":"%{strftime(\{"%Y-%m-%dT%H:%M:%S%z"\}, time.start)}V",
  "client_ip":"%{req.http.Fastly-Client-IP}V",
  "geo_country":"%{client.geo.country_name}V",
  "geo_city":"%{client.geo.city}V",
  "host":"%{if(req.http.Fastly-Orig-Host, req.http.Fastly-Orig-Host, req.http.Host)}V",
  "url":"%{json.escape(req.url)}V",
  "request_method":"%{json.escape(req.method)}V",
  "request_protocol":"%{json.escape(req.proto)}V",
  "request_referer":"%{json.escape(req.http.referer)}V",
  "request_user_agent":"%{json.escape(req.http.User-Agent)}V",
  "response_state":"%{json.escape(fastly_info.state)}V",
  "response_status":%{resp.status}V,
  "response_reason":%{if(resp.response, "%22"+json.escape(resp.response)+"%22", "null")}V,
  "response_body_size":%{resp.body_bytes_written}V,
  "fastly_server":"%{json.escape(server.identity)}V",
  "fastly_is_edge":%{if(fastly.ff.visits_this_service == 0, "true", "false")}V
}`

const legacyDatadogLoggingFormat = `{
    "ddsource": "fastly",
    "service": "%{req.service_id}V",
    "date": "%{begin:%Y-%m-%dT%H:%M:%S%z}t",
    "time_start": "%{begin:%Y-%m-%dT%H:%M:%S%Z}t",
    "time_end": "%{end:%Y-%m-%dT%H:%M:%S%Z}t",
    "http": {
      "request_time_ms": %{time.elapsed.msec}V,
      "method": "%m",
      "url": "%{json.escape(req.url)}V",
      "useragent": "%{json.escape(req.http.User-Agent)}V",
      "referer": "%{json.escape(req.http.referer)}V",
      "protocol": "%H",
      "request_x_forwarded_for": "%{X-Forwarded-For}i",
      "status_code": "%s"
    },
    "network": {
      "client": {
       "ip": "%h",
       "name": "%{client.as.name}V",
       "number": "%{client.as.number}V",
       "connection_speed": "%{client.geo.conn_speed}V"
      },
     "destination": {
       "ip": "%A"
      },
    "geoip": {
    "geo_city": "%{client.geo.city.utf8}V",
    "geo_country_code": "%{client.geo.country_code}V",
    "geo_continent_code": "%{client.geo.continent_code}V",
    "geo_region": "%{client.geo.region}V"
    },
    "bytes_written": %B,
    "bytes_read": %{req.body_bytes_read}V
    },
    "host":"%{if(req.http.Fastly-Orig-Host, req.http.Fastly-Orig-Host, req.http.Host)}V",
    "origin_host": "%v",
    "is_ipv6": %{if(req.is_ipv6, "true", "false")}V,
    "is_tls": %{if(req.is_ssl, "true", "false")}V,
    "tls_client_protocol": "%{json.escape(tls.client.protocol)}V",
    "tls_client_servername": "%{json.escape(tls.client.servername)}V",
    "tls_client_cipher": "%{json.escape(tls.client.cipher)}V",
    "tls_client_cipher_sha": "%{json.escape(tls.client.ciphers_sha)}V",
    "tls_client_tlsexts_sha": "%{json.escape(tls.client.tlsexts_sha)}V",
    "is_h2": %{if(fastly_info.is_h2, "true", "false")}V,
    "is_h2_push": %{if(fastly_info.h2.is_push, "true", "false")}V,
    "h2_stream_id": "%{fastly_info.h2.stream_id}V",
    "request_accept_content": "%{Accept}i",
    "request_accept_language": "%{Accept-Language}i",
    "request_accept_encoding": "%{Accept-Encoding}i",
    "request_accept_charset": "%{Accept-Charset}i",
    "request_connection": "%{Connection}i",
    "request_dnt": "%{DNT}i",
    "request_forwarded": "%{Forwarded}i",
    "request_via": "%{Via}i",
    "request_cache_control": "%{Cache-Control}i",
    "request_x_requested_with": "%{X-Requested-With}i",
    "request_x_att_device_id": "%{X-ATT-Device-Id}i",
    "content_type": "%{Content-Type}o",
    "is_cacheable": %{if(fastly_info.state~"^(HIT|MISS)$", "true","false")}V,
    "response_age": "%{Age}o",
    "response_cache_control": "%{Cache-Control}o",
    "response_expires": "%{Expires}o",
    "response_last_modified": "%{Last-Modified}o",
    "response_tsv": "%{TSV}o",
    "server_datacenter": "%{server.datacenter}V",
    "req_header_size": %{req.header_bytes_read}V,
    "resp_header_size": %{resp.header_bytes_written}V,
    "socket_cwnd": %{client.socket.cwnd}V,
    "socket_nexthop": "%{client.socket.nexthop}V",
    "socket_tcpi_rcv_mss": %{client.socket.tcpi_rcv_mss}V,
    "socket_tcpi_snd_mss": %{client.socket.tcpi_snd_mss}V,
    "socket_tcpi_rtt": %{client.socket.tcpi_rtt}V,
    "socket_tcpi_rttvar": %{client.socket.tcpi_rttvar}V,
    "socket_tcpi_rcv_rtt": %{client.socket.tcpi_rcv_rtt}V,
    "socket_tcpi_rcv_space": %{client.socket.tcpi_rcv_space}V,
    "socket_tcpi_last_data_sent": %{client.socket.tcpi_last_data_sent}V,
    "socket_tcpi_total_retrans": %{client.socket.tcpi_total_retrans}V,
    "socket_tcpi_delta_retrans": %{client.socket.tcpi_delta_retrans}V,
    "socket_ploss": %{client.socket.ploss}V
  }`

const legacyNewRelicLoggingFormat = `{
    "timestamp": %{time.start.msec}V,
    "logtype": "accesslogs",
    "cache_status":"%{regsub(fastly_info.state, "^(HIT-(SYNTH)|(HITPASS|HIT|MISS|PASS|ERROR|PIPE)).*", "\2\3") }V",
    "client_ip":"%h",
    "client_device_type":"%{client.platform.hwtype}V",
    "client_os_name":"%{client.os.name}V",
    "client_os_version":"%{client.os.version}V",
    "client_browser_name":"%{client.browser.name}V",
    "client_browser_version":"%{client.browser.version}V",
    "client_as_name":"%{client.as.name}V",
    "client_as_number":"%{client.as.number}V",
    "client_connection_speed": "%{client.geo.conn_speed}V",
    "client_port": %{client.port}V,
    "client_rate_bps":%{client.socket.tcpi_delivery_rate}V,
    "client_recv_bytes":%{client.socket.tcpi_bytes_received}V,
    "client_requests_count":%{client.requests}V,
    "client_resp_body_size_write": %{resp.body_bytes_written}V,
    "client_resp_header_size_write": %{resp.header_bytes_written}V,
    "client_resp_ttfb": %{time.to_first_byte}V,
    "client_rtt_us":%{client.socket.tcpi_rtt}V,
    "content_type":"%{Content-Type}o",
    "domain": "%{Fastly-Orig-Host}i",
    "fastly_datacenter": "%{server.datacenter}V",
    "fastly_host": "%{server.hostname}V",
    "fastly_is_edge": %{if(fastly.ff.visits_this_service == 0, "true", "false")}V,
    "fastly_region": "%{server.region}V",
    "fastly_server": "%{json.escape(server.identity)}V",
    "host": "%v",
    "origin_host":"%v",
    "origin_name":"%{req.backend.name}V",
    "request":"%{req.request}V",
    "request_method":"%m",
    "request_accept_charset":"%{json.escape(req.http.Accept-Charset)}V",
    "request_accept_language":"%{json.escape(req.http.Accept-Language)}V",
    "request_referer":"%{json.escape(req.http.Referer)}V",
    "request_user_agent":"%{json.escape(req.http.User-Agent)}V",
    "resp_status":"%s",
    "response": "%{resp.response}V",
    "service_id":"%{req.service_id}V",
    "service_version": "%{req.vcl.version}V",
    "status":"%s",
    "time_start":"%{begin:%Y-%m-%dT%H:%M:%SZ}t",
    "time_end":"%{end:%Y-%m-%dT%H:%M:%SZ}t",
    "time_elapsed":%D,
    "tls_cipher":"%{json.escape(tls.client.cipher)}V",
    "tls_version":"%{json.escape(tls.client.protocol)}V",
    "url":"%{json.escape(req.url)}V",
    "user_agent":"%{json.escape(req.http.User-Agent)}V",
    "user_city":"%{client.geo.city.utf8}V",
    "user_country_code":"%{client.geo.country_code}V",
    "user_continent_code":"%{client.geo.continent_code}V",
    "user_region":"%{client.geo.region}V"
  }`

const legacyNewRelicOTLPLoggingFormat = `{
  "timestamp":"%{strftime(\{"%Y-%m-%dT%H:%M:%S%z"\}, time.start)}V",
  "client_ip":"%{req.http.Fastly-Client-IP}V",
  "geo_country":"%{client.geo.country_name}V",
  "geo_city":"%{client.geo.city}V",
  "host":"%{if(req.http.Fastly-Orig-Host, req.http.Fastly-Orig-Host, req.http.Host)}V",
  "url":"%{json.escape(req.url)}V",
  "request_method":"%{json.escape(req.method)}V",
  "request_protocol":"%{json.escape(req.proto)}V",
  "request_referer":"%{json.escape(req.http.referer)}V",
  "request_user_agent":"%{json.escape(req.http.User-Agent)}V",
  "response_state":"%{json.escape(fastly_info.state)}V",
  "response_status":%{resp.status}V,
  "response_reason":%{if(resp.response, "%22"+json.escape(resp.response)+"%22", "null")}V,
  "response_body_size":%{resp.body_bytes_written}V,
  "fastly_server":"%{json.escape(server.identity)}V",
  "fastly_is_edge":%{if(fastly.ff.visits_this_service == 0, "true", "false")}V
}`

const legacyHoneycombLoggingFormat = `{
    "time":"%{begin:%Y-%m-%dT%H:%M:%SZ}t",
    "data":  {
      "service_id":"%{req.service_id}V",
      "time_elapsed":%D,
      "request":"%m",
      "host":"%{if(req.http.Fastly-Orig-Host, req.http.Fastly-Orig-Host, req.http.Host)}V",
      "url":"%{cstr_escape(req.url)}V",
      "protocol":"%H",
      "is_ipv6":%{if(req.is_ipv6, "true", "false")}V,
      "is_tls":%{if(req.is_ssl, "true", "false")}V,
      "is_h2":%{if(fastly_info.is_h2, "true", "false")}V,
      "client_ip":"%h",
      "geo_city":"%{client.geo.city.utf8}V",
      "geo_country_code":"%{client.geo.country_code}V",
      "server_datacenter":"%{server.datacenter}V",
      "request_referer":"%{Referer}i",
      "request_user_agent":"%{User-Agent}i",
      "request_accept_content":"%{Accept}i",
      "request_accept_language":"%{Accept-Language}i",
      "request_accept_charset":"%{Accept-Charset}i",
      "cache_status":"%{regsub(fastly_info.state, "^(HIT-(SYNTH)|(HITPASS|HIT|MISS|PASS|ERROR|PIPE)).*", "\\2\\3") }V",
      "status":"%s",
      "content_type":"%{Content-Type}o",
      "req_header_size":%{req.header_bytes_read}V,
      "req_body_size":%{req.body_bytes_read}V,
      "resp_header_size":%{resp.header_bytes_written}V,
      "resp_body_size":%{resp.body_bytes_written}V
    }
  }`
