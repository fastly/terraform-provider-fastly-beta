package compatibility

// RemainingCDNLoggingFields is the published SDKv2 10.3.0 flat VCL surface
// for VCL-07. Values are deliberately explicit: target-only nested objects
// must never be accepted as if they were legacy configuration.
var RemainingCDNLoggingFields = map[string][]string{
	"logging_datadog":          {"format", "format_version", "name", "placement", "processing_region", "region", "response_condition", "token"},
	"logging_fos":              {"bucket_name", "compression_codec", "file_max_bytes", "format", "format_version", "gzip_level", "message_type", "name", "path", "period", "placement", "public_key", "region", "response_condition", "timestamp_format"},
	"logging_ftp":              {"address", "compression_codec", "format", "format_version", "gzip_level", "message_type", "name", "password", "path", "period", "placement", "port", "processing_region", "public_key", "response_condition", "timestamp_format", "user"},
	"logging_grafanacloudlogs": {"format", "format_version", "index", "name", "placement", "processing_region", "response_condition", "token", "url", "user"},
	"logging_heroku":           {"format", "format_version", "name", "placement", "processing_region", "response_condition", "token", "url"},
	"logging_honeycomb":        {"dataset", "format", "format_version", "name", "placement", "processing_region", "response_condition", "token"},
	"logging_loggly":           {"format", "format_version", "name", "placement", "processing_region", "response_condition", "token"},
	"logging_logshuttle":       {"format", "format_version", "name", "placement", "processing_region", "response_condition", "token", "url"},
	"logging_newrelic":         {"format", "format_version", "name", "placement", "processing_region", "region", "response_condition", "token"},
	"logging_newrelicotlp":     {"format", "format_version", "name", "placement", "processing_region", "region", "response_condition", "token", "url"},
	"logging_papertrail":       {"address", "format", "format_version", "name", "placement", "port", "processing_region", "response_condition"},
	"logging_scalyr":           {"format", "format_version", "name", "placement", "processing_region", "project_id", "region", "response_condition", "token"},
	"logging_sftp":             {"address", "compression_codec", "format", "format_version", "gzip_level", "message_type", "name", "password", "path", "period", "placement", "port", "processing_region", "public_key", "response_condition", "secret_key", "ssh_known_hosts", "timestamp_format", "user"},
	"logging_sumologic":        {"format", "format_version", "message_type", "name", "placement", "processing_region", "response_condition", "url"},
}

// IsRemainingCDNLoggingBlock identifies the fourteen VCL-07 endpoint types.
func IsRemainingCDNLoggingBlock(block string) bool {
	_, ok := RemainingCDNLoggingFields[block]
	return ok
}

// RemainingCDNLoggingLegacyField permits only the actual flat legacy fields.
func RemainingCDNLoggingLegacyField(block, field string) bool {
	for _, name := range RemainingCDNLoggingFields[block] {
		if name == field {
			return true
		}
	}
	return false
}

// RemainingCDNLoggingObjectRoute returns the reviewed flat->nested
// authentication routes. Only VCL uses these rules in CDTOOL-1925;
// shared Compute changes are coordinated separately under CDTOOL-1867.
func RemainingCDNLoggingObjectRoute(block, field string) (object, member string, ok bool) {
	switch block {
	case "logging_ftp":
		if field == "user" || field == "password" {
			return "authentication", field, true
		}
	case "logging_sftp":
		if field == "user" || field == "password" || field == "secret_key" {
			return "authentication", field, true
		}
	case "logging_datadog":
		if field == "token" {
			return "authentication", "token", true
		}
	case "logging_loggly":
		if field == "token" {
			return "authentication", "token", true
		}
	case "logging_grafanacloudlogs":
		if field == "token" {
			return "authentication", "token", true
		}
	case "logging_scalyr":
		if field == "token" {
			return "authentication", "token", true
		}
	case "logging_newrelic":
		if field == "token" {
			return "authentication", "token", true
		}
	case "logging_newrelicotlp":
		if field == "token" {
			return "authentication", "token", true
		}
	case "logging_heroku":
		if field == "token" {
			return "authentication", "token", true
		}
	case "logging_honeycomb":
		if field == "token" {
			return "authentication", "token", true
		}
	case "logging_logshuttle":
		if field == "token" {
			return "authentication", "token", true
		}
	}
	return "", "", false
}
