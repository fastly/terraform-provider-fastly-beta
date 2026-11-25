package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingdatadog"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingfos"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingftp"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginggrafanacloudlogs"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingheroku"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginghoneycomb"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingloggly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginglogshuttle"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingnewrelic"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingnewrelicotlp"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingpapertrail"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingscalyr"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsftp"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsumologic"
)

var remainingCDNLoggingKinds = []string{
	"logging_datadog",
	"logging_ftp",
	"logging_sftp",
	"logging_loggly",
	"logging_grafanacloudlogs",
	"logging_scalyr",
	"logging_newrelic",
	"logging_newrelicotlp",
	"logging_heroku",
	"logging_honeycomb",
	"logging_logshuttle",
	"logging_papertrail",
	"logging_sumologic",
	"logging_fos",
}

func remainingCDNFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "logging_remaining", "cdn", name)) // #nosec G304 -- repository-owned fixture
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Remove unrelated endpoint blocks without rewriting their contents, so
// endpoint-specific tests continue to use the same independent HCL fixtures.
func oneRemainingCDNFixture(t *testing.T, source, endpoint string) string {
	t.Helper()
	blocks := loggingFixtureResource(t, source).Body.Blocks
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		if !compatibility.IsRemainingCDNLoggingBlock(b.Type) || b.Type == endpoint {
			continue
		}
		rng := b.Range()
		start := strings.LastIndex(source[:rng.Start.Byte], "\n") + 1
		end := rng.End.Byte
		if end < len(source) && source[end] == '\n' {
			end++
		}
		source = source[:start] + source[end:]
	}
	return source
}

func TestPlanRemainingCDNLoggingGolden(t *testing.T) {
	t.Parallel()
	for _, endpoint := range append([]string{"all"}, remainingCDNLoggingKinds...) {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := remainingCDNFixture(t, "legacy.tf")
			want := remainingCDNFixture(t, "expected.tf")
			if endpoint != "all" {
				source = oneRemainingCDNFixture(t, source, endpoint)
				want = oneRemainingCDNFixture(t, want, endpoint)
			}
			writeConfig(t, dir, "main.tf", source)
			plan := newPlan(t, dir)
			if plan.Blocked() || len(plan.Moves) != 1 {
				t.Fatalf("VCL-07 plan blocked for %s: issues=%v findings=%v", endpoint, plan.Issues, plan.Analysis.Findings)
			}
			if got := changedFile(t, plan, "main.tf"); got != want {
				// Do not print raw HCL: actual configurations may hold secrets.
				t.Fatalf("VCL-07 output differs from independently authored golden for %s", endpoint)
			}
			// All VCL-07 endpoint diffs must be withheld to avoid displaying
			// credential values (including values in locals or comments).
			for _, render := range []func(io.Writer, Result) error{WriteText, WriteApplyPreview} {
				var out bytes.Buffer
				if err := render(&out, plan); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(out.String(), "+++ b/") || !strings.Contains(out.String(), "HCL diff withheld") {
					t.Fatal("VCL-07 HCL diff exposed in preview")
				}
			}
			requireStableAfterApply(t, dir, plan)
		})
	}
}

func TestPlanRemainingCDNLoggingDefaults(t *testing.T) {
	t.Parallel()
	for _, endpoint := range remainingCDNLoggingKinds {
		for _, nullLiteral := range []bool{false, true} {
			t.Run(endpoint+"/null="+map[bool]string{true: "yes", false: "no"}[nullLiteral], func(t *testing.T) {
				t.Parallel()
				source := oneRemainingCDNFixture(t, remainingCDNFixture(t, "legacy.tf"), endpoint)
				replacement := ""
				if nullLiteral {
					replacement = "    format = null\n"
				}
				source = strings.Replace(source, "    format = var.format\n", replacement, 1)
				dir := t.TempDir()
				writeConfig(t, dir, "main.tf", source)
				plan := newPlan(t, dir)
				if plan.Blocked() {
					t.Fatalf("legacy default pinning blocked for %s: %v %v", endpoint, plan.Issues, plan.Analysis.Findings)
				}
				got := changedFile(t, plan, "main.tf")
				found := false
				for _, b := range loggingFixtureResource(t, got).Body.Blocks {
					if b.Type != endpoint {
						continue
					}
					found = true
					attr := b.Body.Attributes["format"]
					if attr == nil {
						t.Fatal("missing pinned SDKv2 log format")
					}
					val, diags := attr.Expr.Value(nil)
					if diags.HasErrors() || !val.IsKnown() || val.IsNull() ||
						val.AsString() != legacyVCLLoggingFormat(endpoint) {
						t.Fatal("SDKv2 log format bytes not preserved")
					}
					if endpoint == "logging_sumologic" && b.Body.Attributes["message_type"] == nil {
						t.Fatal("Sumo Logic legacy message_type was not pinned")
					}
				}
				if !found {
					t.Fatal("missing migrated endpoint")
				}
				requireStableAfterApply(t, dir, plan)
			})
		}
	}
}

func TestPlanRemainingCDNLoggingPreservesExplicitExpressions(t *testing.T) {
	t.Parallel()
	source := oneRemainingCDNFixture(t, remainingCDNFixture(t, "legacy.tf"), "logging_datadog")
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("explicit format or token expression blocked: %v", plan.Issues)
	}
	output := changedFile(t, plan, "main.tf")
	for _, expected := range []string{
		"format = var.format",
		"token = var.credential",
		"authentication = {",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing preserved expression or credential nesting: %s", expected)
		}
	}
}

func TestPlanRemainingCDNLoggingRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct{ endpoint, original, replacement, diagnostic string }{
		{"logging_datadog", "    token = var.credential\n", "", "token"},
		{"logging_datadog", "    token = var.credential", "    token = null", "token"},
		{"logging_ftp", "    password = var.credential\n", "", "password"},
		{"logging_sftp", "    secret_key = var.credential\n", "", "password/secret_key"},
		{"logging_sftp", "    secret_key = var.credential", "    secret_key = var.credential\n    password = var.credential", "password/secret_key"},
		{"logging_fos", "    region = \"us-east\"\n", "", "region"},
		{"logging_heroku", "    url = \"https://heroku.example.invalid\"\n", "", "url"},
		{"logging_heroku", "    url = \"https://heroku.example.invalid\"", "    url = null", "url"},
		{"logging_heroku", "    url = \"https://heroku.example.invalid\"", `    url = ""`, "url"},
		{"logging_datadog", "    format = var.format", `    format = ""`, "format"},
		{"logging_datadog", "    format = var.format", `    format = var.format
    placement = "invalid"`, "placement"},
		{"logging_ftp", "    format = var.format", `    format = var.format
    authentication = { password = "SYNTHETIC_SECRET" }`, "authentication"},
		{"logging_fos", "    format = var.format", `    format = var.format
    gzip_level = 10`, "gzip_level"},
		{"logging_datadog", "    format = var.format", `    format = var.format
    region = "NOWHERE"`, "region"},
		{"logging_datadog", "    format = var.format", `    format = var.format
    processing_region = "moon"`, "processing_region"},
		{"logging_datadog", "    format = var.format", `    format = var.format
    format_version = 99`, "format_version"},
		{"logging_sumologic", "    format = var.format", `    format = var.format
    message_type = "surprise"`, "message_type"},
		{"logging_fos", "    format = var.format", `    format = var.format
    compression_codec = "brotli"`, "compression_codec"},
	}
	for _, tc := range tests {
		t.Run(tc.endpoint+"/"+tc.diagnostic, func(t *testing.T) {
			t.Parallel()
			source := oneRemainingCDNFixture(t, remainingCDNFixture(t, "legacy.tf"), tc.endpoint)
			if !strings.Contains(source, tc.original) {
				t.Fatal("fixture drift")
			}
			source = strings.Replace(source, tc.original, tc.replacement, 1)
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", source)
			plan := newPlan(t, dir)
			if !plan.Blocked() || len(plan.Moves) != 0 || len(plan.Changes) != 0 {
				t.Fatal("unsafe VCL-07 configuration was accepted")
			}
			var out bytes.Buffer
			if err := WriteText(&out, plan); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.diagnostic) ||
				strings.Contains(out.String(), "SYNTHETIC_SECRET") {
				t.Fatal("expected non-sensitive diagnostic missing")
			}
			if _, err := plan.Apply(context.Background(), privateBackupDir(t)); err == nil {
				t.Fatal("blocked migration unexpectedly wrote")
			}
			original, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(original) != source {
				t.Fatal("blocked migration changed source HCL")
			}
		})
	}
}

func TestRemainingCDNLoggingPublishedSchemaInventory(t *testing.T) {
	t.Parallel()
	targets := map[string]schema.ListNestedBlock{
		"logging_datadog":          loggingdatadog.NestedBlockSchema(),
		"logging_ftp":              loggingftp.NestedBlockSchema(),
		"logging_sftp":             loggingsftp.NestedBlockSchema(),
		"logging_loggly":           loggingloggly.NestedBlockSchema(),
		"logging_grafanacloudlogs": logginggrafanacloudlogs.NestedBlockSchema(),
		"logging_scalyr":           loggingscalyr.NestedBlockSchema(),
		"logging_newrelic":         loggingnewrelic.NestedBlockSchema(),
		"logging_newrelicotlp":     loggingnewrelicotlp.NestedBlockSchema(),
		"logging_heroku":           loggingheroku.NestedBlockSchema(),
		"logging_honeycomb":        logginghoneycomb.NestedBlockSchema(),
		"logging_logshuttle":       logginglogshuttle.NestedBlockSchema(),
		"logging_papertrail":       loggingpapertrail.NestedBlockSchema(),
		"logging_sumologic":        loggingsumologic.NestedBlockSchema(),
		"logging_fos":              loggingfos.NestedBlockSchema(),
	}
	// Ensure the independently authored golden actually supplies every
	// required top-level target attribute, not just supported attribute names.
	for _, block := range loggingFixtureResource(t, remainingCDNFixture(t, "expected.tf")).Body.Blocks {
		target, ok := targets[block.Type]
		if !ok {
			continue
		}
		for name, attr := range target.NestedObject.Attributes {
			if attr.IsRequired() && block.Body.Attributes[name] == nil {
				t.Errorf("golden fixture missing required target attribute %s.%s", block.Type, name)
			}
		}
	}
	for _, suffix := range []string{"_type.json", "_10_3_0_type.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "resources", "servicecdnauto", "testdata", "legacy_service_vcl_schema_v1"+suffix)) // #nosec G304 -- repository fixture
		if err != nil {
			t.Fatal(err)
		}
		var legacy struct {
			Block struct {
				BlockTypes map[string]struct {
					Block struct{ Attributes map[string]json.RawMessage }
				} `json:"block_types"`
			} `json:"block"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			t.Fatal(err)
		}
		seen := 0
		for block := range legacy.Block.BlockTypes {
			if strings.HasPrefix(block, "logging_") {
				seen++
			}
		}
		if suffix == "_10_3_0_type.json" && seen != 29 {
			t.Fatalf("expected 29 published VCL logging blocks, got %d", seen)
		}
		for _, block := range remainingCDNLoggingKinds {
			sourceFields := legacy.Block.BlockTypes[block].Block.Attributes
			if len(sourceFields) == 0 {
				if block == "logging_fos" && suffix == "_type.json" {
					// FOS was introduced in the later SDKv2 provider.
					continue
				}
				t.Fatalf("%s missing legacy schema %s", suffix, block)
			}
			targetAttrs := targets[block].NestedObject.Attributes
			for name := range sourceFields {
				if !compatibility.RemainingCDNLoggingLegacyField(block, name) {
					t.Errorf("unreviewed SDKv2 attribute %s.%s", block, name)
					continue
				}
				object, member, moved := compatibility.RemainingCDNLoggingObjectRoute(block, name)
				if moved {
					auth, ok := targetAttrs[object].(schema.SingleNestedAttribute)
					if !ok {
						t.Errorf("missing nested auth %s.%s", block, object)
						continue
					}
					memberAttr, ok := auth.Attributes[member].(schema.StringAttribute)
					if !ok || (!memberAttr.Optional && !memberAttr.Required) || targetAttrs[name] != nil {
						t.Errorf("invalid nested auth mapping %s.%s -> %s.%s", block, name, object, member)
					}
					found := false
					for _, rule := range compatibility.Default().RulesForLegacy("fastly_service_vcl." + block + "." + name) {
						p, o, f, ok := serviceObjectField(rule, "fastly_service_vcl", "fastly_service_cdn_auto")
						if ok && p == block && o == object && f.From == name && f.To == member {
							found = true
						}
					}
					if !found {
						t.Errorf("missing reviewed compatibility route %s.%s", block, name)
					}
				} else if targetAttrs[name] == nil || (!targetAttrs[name].IsRequired() && !targetAttrs[name].IsOptional()) {
					t.Errorf("missing configurable target attribute %s.%s", block, name)
				}
			}
		}
	}
}

func TestRemainingLoggingInventoryAndFailClosedPolicy(t *testing.T) {
	t.Parallel()
	registry := compatibility.Default()
	if len(remainingCDNLoggingKinds) != 14 {
		t.Fatalf("expected fourteen VCL-07 endpoints, got %d", len(remainingCDNLoggingKinds))
	}
	for _, block := range remainingCDNLoggingKinds {
		automatic := false
		for _, rule := range registry.RulesForLegacy("fastly_service_vcl." + block) {
			if rule.Kind == compatibility.ConstructServiceBlock && rule.Support == compatibility.SupportAutomatic &&
				rule.State == compatibility.StateMove {
				automatic = true
			}
		}
		if !automatic {
			t.Errorf("missing automatic CDN migration for %s", block)
		}
	}
	for _, source := range []string{"fastly_service_vcl", "fastly_service_compute"} {
		found := false
		for _, rule := range registry.RulesForLegacy(source + ".logging_logentries") {
			if rule.Support == compatibility.SupportUnsupported && rule.State == compatibility.StateNone {
				found = true
			}
		}
		if !found {
			t.Errorf("%s must retain explicit Logentries refusal", source)
		}
	}
	for _, block := range append(append([]string{}, remainingCDNLoggingKinds...), "logging_logentries") {
		if block == "logging_papertrail" || block == "logging_sumologic" || block == "logging_logentries" {
			continue
		}
		for _, rule := range registry.RulesForLegacy("fastly_service_compute." + block) {
			if rule.Kind == compatibility.ConstructServiceBlock && rule.Support != compatibility.SupportBlocked {
				t.Errorf("Compute %s must stay blocked pending CDTOOL-1867", block)
			}
		}
	}
}

// A removed upstream logging integration must never be treated as a no-op
// source block merely because the target schema lacks the destination.
func TestPlanRemainingCDNLoggingRefusesLogentries(t *testing.T) {
	t.Parallel()
	source := `resource "fastly_service_vcl" "demo" {
  name = "logentries-is-unsupported"
  logging_logentries {
    name = "legacy"
    token = "SYNTHETIC_SECRET"
  }
}
`
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if !plan.Blocked() || len(plan.Moves) != 0 || len(plan.Changes) != 0 {
		t.Fatal("Logentries migration must remain blocked")
	}
	if _, err := plan.Apply(context.Background(), privateBackupDir(t)); err == nil {
		t.Fatal("unsupported Logentries migration wrote files")
	}
	b, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil || string(b) != source {
		t.Fatal("unsupported Logentries migration changed source")
	}
}

func TestPlanRemainingCDNLoggingSFTPPasswordAlternative(t *testing.T) {
	t.Parallel()
	source := oneRemainingCDNFixture(t, remainingCDNFixture(t, "legacy.tf"), "logging_sftp")
	source = strings.Replace(source, "secret_key = var.credential", `password = "password"`, 1)
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("valid SFTP password authentication blocked: %v %v", plan.Issues, plan.Analysis.Findings)
	}
	got := changedFile(t, plan, "main.tf")
	if !strings.Contains(got, "authentication = {") ||
		!strings.Contains(got, `password = "password"`) ||
		strings.Contains(got, "secret_key = var.credential") {
		t.Fatal("SFTP password was not moved to the authentication object")
	}
	requireStableAfterApply(t, dir, plan)
}

// TestRemainingVCLLoggingInventoryHasExact29PublishedTypes keeps the ticket
// groups exhaustive as the SDKv2 fixture evolves. Any unexpected new
// endpoint must be reviewed rather than silently classified as supported.
func TestRemainingVCLLoggingInventoryHasExact29PublishedTypes(t *testing.T) {
	t.Parallel()
	groups := map[string][]string{
		"VCL-05": {
			"logging_https", "logging_elasticsearch", "logging_kafka",
			"logging_splunk", "logging_syslog",
		},
		"VCL-06": {
			"logging_s3", "logging_gcs", "logging_bigquery", "logging_blobstorage",
			"logging_cloudfiles", "logging_digitalocean", "logging_openstack",
			"logging_googlepubsub", "logging_kinesis",
		},
		"VCL-07":      remainingCDNLoggingKinds,
		"Unsupported": {"logging_logentries"},
	}
	seen := make(map[string]string)
	for group, names := range groups {
		for _, name := range names {
			if prev, exists := seen[name]; exists {
				t.Fatalf("%s belongs to both %s and %s", name, prev, group)
			}
			seen[name] = group
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "servicecdnauto",
		"testdata", "legacy_service_vcl_schema_v1_10_3_0_type.json")) // #nosec G304 -- repository fixture
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		Block struct {
			BlockTypes map[string]json.RawMessage `json:"block_types"`
		} `json:"block"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	for name := range legacy.Block.BlockTypes {
		if !strings.HasPrefix(name, "logging_") {
			continue
		}
		if _, ok := seen[name]; !ok {
			t.Errorf("published SDKv2 logging type %s has no work-package classification", name)
		}
	}
	if len(seen) != 29 {
		t.Fatalf("expected 29 VCL logging types, got %d", len(seen))
	}
}

func TestPlanRemainingComputeFOSRemainsBlocked(t *testing.T) {
	t.Parallel()
	source := `resource "fastly_service_compute" "demo" {
  name = "compute-fos-must-be-reviewed"
  logging_fos {
    name        = "fos"
    bucket_name = "test-bucket"
    region      = "us-east"
  }
}
`
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if !plan.Blocked() || len(plan.Moves) != 0 || len(plan.Changes) != 0 {
		t.Fatal("Compute FOS must be blocked pending CDTOOL-1867")
	}
	b, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil || string(b) != source {
		t.Fatal("blocked Compute FOS migration changed source")
	}
}

// The Sumo Logic default differs in both service families. CDTOOL-1925
// pins it when migrating Compute too, without enabling other Compute
// authentication shapes that still require CDTOOL-1867.
func TestPlanRemainingComputeSumologicPreservesLegacyMessageType(t *testing.T) {
	t.Parallel()
	source := `resource "fastly_service_compute" "demo" {
  name = "compute-sumologic-default"
  logging_sumologic {
    name = "s"
    url  = "https://logs.example.invalid/receiver"
  }
}
`
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("Compute Sumo Logic migration blocked: %v %v", plan.Issues, plan.Analysis.Findings)
	}
	got := changedFile(t, plan, "main.tf")
	if !strings.Contains(got, `message_type = "classic"`) {
		t.Fatal("Compute Sumo Logic did not preserve SDKv2 message_type default")
	}
	requireStableAfterApply(t, dir, plan)
}

func TestPlanRemainingComputeTokenAuthenticationIsBlocked(t *testing.T) {
	t.Parallel()
	source := `resource "fastly_service_compute" "demo" {
  name = "compute-token-migration-not-yet-reviewed"
  logging_datadog {
    name  = "endpoint"
    token = "SYNTHETIC_SECRET"
  }
}
`
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if !plan.Blocked() || len(plan.Moves) != 0 || len(plan.Changes) != 0 {
		t.Fatal("Compute Datadog flat authentication must be blocked pending CDTOOL-1867")
	}
	b, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil || string(b) != source {
		t.Fatal("blocked Compute authentication migration changed source")
	}
}
