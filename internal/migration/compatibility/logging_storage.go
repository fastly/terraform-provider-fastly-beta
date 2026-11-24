package compatibility

import "slices"

// StorageLoggingFields lists the supported published SDKv2 flat fields.
// The inventory is checked against both CDN and Compute historical schema
// snapshots in migration tests. Target-only objects are never accepted.
var StorageLoggingFields = map[string][]string{
	"logging_s3":           {"acl", "bucket_name", "compression_codec", "domain", "file_max_bytes", "gzip_level", "message_type", "name", "path", "period", "processing_region", "public_key", "redundancy", "s3_access_key", "s3_iam_role", "s3_secret_key", "server_side_encryption", "server_side_encryption_kms_key_id", "timestamp_format"},
	"logging_gcs":          {"account_name", "bucket_name", "compression_codec", "gzip_level", "message_type", "name", "path", "period", "processing_region", "project_id", "public_key", "secret_key", "timestamp_format", "user"},
	"logging_bigquery":     {"account_name", "dataset", "email", "name", "processing_region", "project_id", "secret_key", "table", "template"},
	"logging_blobstorage":  {"account_name", "compression_codec", "container", "file_max_bytes", "gzip_level", "message_type", "name", "path", "period", "processing_region", "public_key", "sas_token", "timestamp_format"},
	"logging_cloudfiles":   {"access_key", "bucket_name", "compression_codec", "gzip_level", "message_type", "name", "path", "period", "processing_region", "public_key", "region", "timestamp_format", "user"},
	"logging_digitalocean": {"access_key", "bucket_name", "compression_codec", "domain", "gzip_level", "message_type", "name", "path", "period", "processing_region", "public_key", "secret_key", "timestamp_format"},
	"logging_openstack":    {"access_key", "bucket_name", "compression_codec", "gzip_level", "message_type", "name", "path", "period", "processing_region", "public_key", "timestamp_format", "url", "user"},
	"logging_googlepubsub": {"account_name", "name", "processing_region", "project_id", "secret_key", "topic", "user"},
	"logging_kinesis":      {"access_key", "iam_role", "name", "processing_region", "region", "secret_key", "topic"},
}

// IsStorageLoggingBlock identifies the nine reviewed storage/cloud endpoints.
func IsStorageLoggingBlock(name string) bool {
	_, ok := StorageLoggingFields[name]
	return ok
}

// StorageLoggingLegacyField checks the exact flat SDKv2 surface.
func StorageLoggingLegacyField(block, field string, vcl bool) bool {
	fields, ok := StorageLoggingFields[block]
	if !ok {
		return false
	}
	if slices.Contains(fields, field) {
		return true
	}
	return vcl && slices.Contains([]string{"format", "format_version", "placement", "response_condition"}, field) &&
		(block != "logging_bigquery" || field != "format_version")
}

// StorageLoggingObjectRoute returns only explicitly reviewed credential moves.
// The compiler must also match a registry rule with the exact target path.
func StorageLoggingObjectRoute(block, field string) (object, member string, ok bool) {
	switch block {
	case "logging_s3":
		switch field {
		case "s3_access_key":
			return "authentication", "access_key", true
		case "s3_secret_key":
			return "authentication", "secret_key", true
		case "s3_iam_role":
			return "authentication", "iam_role", true
		}
	case "logging_gcs":
		switch field {
		case "account_name":
			return "authentication", "account_name", true
		case "user":
			return "authentication", "email", true
		case "secret_key":
			return "authentication", "secret_key", true
		}
	case "logging_bigquery":
		switch field {
		case "account_name":
			return "authentication", "account_name", true
		case "email":
			return "authentication", "email", true
		case "secret_key":
			return "authentication", "secret_key", true
		}
	case "logging_blobstorage":
		switch field {
		case "account_name":
			return "authentication", "account_name", true
		case "sas_token":
			return "authentication", "sas_token", true
		}
	case "logging_cloudfiles":
		switch field {
		case "user":
			return "authentication", "user", true
		case "access_key":
			return "authentication", "access_key", true
		}
	case "logging_digitalocean":
		switch field {
		case "access_key":
			return "authentication", "access_key", true
		case "secret_key":
			return "authentication", "secret_key", true
		}
	case "logging_openstack":
		switch field {
		case "user":
			return "authentication", "user", true
		case "access_key":
			return "authentication", "access_key", true
		}
	case "logging_googlepubsub":
		switch field {
		case "account_name":
			return "authentication", "account_name", true
		case "user":
			return "authentication", "email", true
		case "secret_key":
			return "authentication", "secret_key", true
		}
	case "logging_kinesis":
		switch field {
		case "access_key":
			return "authentication", "access_key", true
		case "secret_key":
			return "authentication", "secret_key", true
		case "iam_role":
			return "authentication", "iam_role", true
		}
	}
	return "", "", false
}
