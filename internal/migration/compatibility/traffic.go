package compatibility

// VCLNameOrderedBlocks lists repeated fastly_service_vcl blocks that legacy
// stored as sets and the target reads back as lists sorted by name. The
// migrator sorts them by literal name so the first plan has no reorder.
var VCLNameOrderedBlocks = []string{
	"acl",
	"backend",
	"cache_setting",
	"condition",
	"dictionary",
	"director",
	"domain",
	"dynamicsnippet",
	"gzip",
	"header",
	"healthcheck",
	"logging_bigquery",
	"logging_blobstorage",
	"logging_cloudfiles",
	"logging_datadog",
	"logging_digitalocean",
	"logging_elasticsearch",
	"logging_ftp",
	"logging_gcs",
	"logging_googlepubsub",
	"logging_grafanacloudlogs",
	"logging_heroku",
	"logging_honeycomb",
	"logging_https",
	"logging_kafka",
	"logging_kinesis",
	"logging_loggly",
	"logging_logshuttle",
	"logging_newrelic",
	"logging_newrelicotlp",
	"logging_openstack",
	"logging_papertrail",
	"logging_s3",
	"logging_scalyr",
	"logging_sftp",
	"logging_splunk",
	"logging_sumologic",
	"logging_syslog",
	"rate_limiter",
	"request_setting",
	"response_object",
	"snippet",
	"vcl",
}

// LiteralRewrite describes a value change for one nested attribute.
// Values maps a legacy literal (whole numbers in decimal form) to the target
// literal; every key is also an accepted legacy value. RemoveEmpty drops a
// literal "" that the target would read back as null.
type LiteralRewrite struct {
	Block           string
	Attribute       string
	Values          map[string]string
	CaseInsensitive bool
	RemoveEmpty     bool
}

// VCLLiteralRewrites are the traffic-management value changes.
var VCLLiteralRewrites = []LiteralRewrite{
	{
		// Legacy accepted only 1, 3 and 4; round robin (2) was never valid.
		Block: "director", Attribute: "type",
		Values: map[string]string{"1": "random", "3": "hash", "4": "client"},
	},
	{
		// Legacy converted these to lowercase before sending them to the API.
		Block: "cache_setting", Attribute: "action",
		Values:          map[string]string{"cache": "cache", "pass": "pass", "restart": "restart"},
		CaseInsensitive: true, RemoveEmpty: true,
	},
	{Block: "cache_setting", Attribute: "cache_condition", RemoveEmpty: true},
	{Block: "gzip", Attribute: "cache_condition", RemoveEmpty: true},
	{
		Block: "request_setting", Attribute: "action",
		Values:          map[string]string{"lookup": "lookup", "pass": "pass"},
		CaseInsensitive: true, RemoveEmpty: true,
	},
	{
		Block: "request_setting", Attribute: "xff",
		Values: map[string]string{
			"clear": "clear", "leave": "leave", "append": "append",
			"append_all": "append_all", "overwrite": "overwrite",
		},
		CaseInsensitive: true, RemoveEmpty: true,
	},
	{Block: "response_object", Attribute: "content_type", RemoveEmpty: true},
}
