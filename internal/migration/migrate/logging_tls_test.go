package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingelasticsearch"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginghttps"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingkafka"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsplunk"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsyslog"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/zclconf/go-cty/cty"
)

var loggingTLSBlocks = []string{"logging_https", "logging_elasticsearch", "logging_kafka", "logging_splunk", "logging_syslog"}

func loggingFixture(t *testing.T, family, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "logging_tls", family, filename)) // #nosec G304 -- repository-owned fixtures
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func loggingFixtureResource(t *testing.T, source string) *hclsyntax.Block {
	t.Helper()
	parsed, diags := hclsyntax.ParseConfig([]byte(source), "main.tf", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() || parsed == nil {
		t.Fatal("logging fixture is not valid native HCL")
	}
	body, ok := parsed.Body.(*hclsyntax.Body)
	if !ok {
		t.Fatal("expected native HCL body")
	}
	for _, block := range body.Blocks {
		if block.Type == "resource" {
			return block
		}
	}
	t.Fatal("missing fixture resource")
	return nil
}

// Reuse the authored combined golden as individual endpoint fixtures by
// removing the other logging blocks from both sides, never regenerating
// the expected expressions via the migration implementation.
func oneLoggingFixture(t *testing.T, source, endpoint string) string {
	t.Helper()
	if endpoint == "all" {
		return source
	}
	blocks := loggingFixtureResource(t, source).Body.Blocks
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		if !compatibility.IsTLSLoggingBlock(b.Type) || b.Type == endpoint {
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

func TestPlanLoggingTLSGolden(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"cdn", "compute"} {
		for _, endpoint := range append([]string{"all"}, loggingTLSBlocks...) {
			t.Run(family+"/"+endpoint, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				source := oneLoggingFixture(t, loggingFixture(t, family, "legacy.tf"), endpoint)
				want := oneLoggingFixture(t, loggingFixture(t, family, "expected.tf"), endpoint)
				writeConfig(t, dir, "main.tf", source)
				result := newPlan(t, dir)
				if result.Blocked() || len(result.Moves) != 1 {
					t.Fatalf("logging plan blocked: %v %v", result.Issues, result.Analysis.Findings)
				}
				got := changedFile(t, result, "main.tf")
				if got != want {
					t.Fatalf("logging golden mismatch for %s/%s", family, endpoint)
				}
				loggingFixtureResource(t, got)
				for _, render := range []func(io.Writer, Result) error{WriteText, WriteApplyPreview} {
					var out bytes.Buffer
					if err := render(&out, result); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(out.String(), "SYNTHETIC_HEADER_MARKER") ||
						strings.Contains(out.String(), "+++ b/") ||
						!strings.Contains(out.String(), "HCL diff withheld") {
						t.Fatal("sensitive logging preview exposed source or lacked safety notice")
					}
				}
				before, err := os.ReadFile(filepath.Join(dir, "main.tf"))
				if err != nil || string(before) != source {
					t.Fatal("planning changed input")
				}
				requireStableAfterApply(t, dir, result)
				after, err := os.ReadFile(filepath.Join(dir, "main.tf"))
				if err != nil || string(after) != want {
					t.Fatal("write did not apply the golden plan")
				}
			})
		}
	}
}

func TestPlanLoggingTLSPinsLegacyFormat(t *testing.T) {
	t.Parallel()
	body := loggingFixtureResource(t, loggingFixture(t, "cdn", "legacy.tf")).Body
	source := loggingFixture(t, "cdn", "legacy.tf")
	// Rewrite just the fixture's explicit formats to test omission, literal
	// null and an unchanged non-null expression in different repeated blocks.
	for i := len(body.Blocks) - 1; i >= 0; i-- {
		block := body.Blocks[i]
		if !compatibility.IsTLSLoggingBlock(block.Type) {
			continue
		}
		attr := block.Body.Attributes["format"]
		start := strings.LastIndex(source[:attr.SrcRange.Start.Byte], "\n") + 1
		end := attr.SrcRange.End.Byte + 1
		replacement := ""
		if block.Type == "logging_splunk" {
			replacement = "    format = null # preserve comment\n"
		} else if block.Type == "logging_https" && block.Body.Attributes["compression_codec"] != nil {
			replacement = "    format = var.format\n"
		}
		source = source[:start] + replacement + source[end:]
	}
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("default format migration blocked: %v %v", result.Issues, result.Analysis.Findings)
	}
	after := changedFile(t, result, "main.tf")
	for _, block := range loggingFixtureResource(t, after).Body.Blocks {
		if !compatibility.IsTLSLoggingBlock(block.Type) {
			continue
		}
		attr := block.Body.Attributes["format"]
		if attr == nil {
			t.Fatal("missing pinned legacy format")
		}
		if block.Type == "logging_https" && block.Body.Attributes["compression_codec"] != nil {
			rng := attr.Expr.Range()
			if after[rng.Start.Byte:rng.End.Byte] != "var.format" {
				t.Fatal("explicit format expression changed")
			}
			continue
		}
		got, diags := attr.Expr.Value(nil)
		if diags.HasErrors() || !got.IsKnown() || got.IsNull() || got.Type() != cty.String || got.AsString() != legacyTLSLoggingFormat(block.Type) {
			t.Fatal("legacy format bytes changed or literal VCL template was not escaped")
		}
	}
	if !strings.Contains(after, "# preserve comment") {
		t.Fatal("literal-null attribute comment was lost")
	}
	requireStableAfterApply(t, dir, result)
}

func TestPlanLoggingTLSRejectsUnsupportedShapes(t *testing.T) {
	t.Parallel()
	for i, tc := range []struct{ block, extra, want string }{
		{"logging_https", "tls = local.secret", "Unsupported legacy field"},
		{"logging_https", "authentication = {}", "Unsupported legacy field"},
		{"logging_https", "tls_client_key = \"SYNTHETIC_KEY\"\n    tls = {}", "Unsupported legacy field"},
		{"logging_https", "tls_ca_cert = \"SYNTHETIC_KEY\"\n    tls {}", "no service-block compatibility rule"},
		{"logging_https", "gzip_level = -1", "gzip_level"},
		{"logging_https", "gzip_level = 1.5", "gzip_level"},
		{"logging_https", "gzip_level = 2\n    compression_codec = \"gzip\"", "gzip_level"},
		{"logging_https", "gzip_level = var.level\n    compression_codec = var.codec", "gzip_level"},
		{"logging_https", "extra_secret = \"SYNTHETIC_KEY\"", "Unsupported legacy field"},
		{"logging_https", "format = \"\"", "format"},
		{"logging_https", "placement = \"\"", "placement"},
		{"logging_elasticsearch", "authentication = local.secret", "Unsupported legacy field"},
		{"logging_elasticsearch", "password = \"SYNTHETIC_KEY\"\n    authentication = {}", "Unsupported legacy field"},
		{"logging_kafka", "auth_method = \"invalid\"", "auth_method"},
		{"logging_splunk", "request_max_entries = 100", "Unsupported legacy field"},
		{"logging_splunk", "request_max_bytes = 100", "Unsupported legacy field"},
		{"logging_syslog", "token = \"SYNTHETIC_KEY\"\n    authentication {}", "no service-block compatibility rule"},
	} {
		t.Run(fmt.Sprintf("%s/%d", tc.block, i), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			required := "url = \"https://logs.example.invalid/\""
			switch tc.block {
			case "logging_elasticsearch":
				required += "\n    index = \"logs\""
			case "logging_kafka":
				required = "brokers = \"kafka.example.invalid:9093\"\n    topic = \"logs\""
			case "logging_syslog":
				required = "address = \"syslog.example.invalid\""
			}
			source := fmt.Sprintf("resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n  %s {\n    name = \"endpoint\"\n    %s\n    %s\n  }\n}\n", tc.block, required, tc.extra)
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatal("unsafe logging plan accepted")
			}
			var out bytes.Buffer
			if err := WriteText(&out, result); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "SYNTHETIC_KEY") || !strings.Contains(out.String(), tc.want) {
				t.Fatal("missing safe diagnostic or secret marker leaked")
			}
			if _, err := result.Apply(context.Background(), privateBackupDir(t)); err == nil {
				t.Fatal("blocked migration could be applied")
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(got) != source {
				t.Fatal("blocked migration changed input")
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
				t.Fatal("blocked migration created migration.tf")
			}
		})
	}
}

func TestLoggingTLSRoutesMatchSchemas(t *testing.T) {
	t.Parallel()
	schemas := map[string][2]schema.ListNestedBlock{
		"logging_https":         {logginghttps.NestedBlockSchema(), logginghttps.ComputeNestedBlockSchema()},
		"logging_elasticsearch": {loggingelasticsearch.NestedBlockSchema(), loggingelasticsearch.ComputeNestedBlockSchema()},
		"logging_kafka":         {loggingkafka.NestedBlockSchema(), loggingkafka.ComputeNestedBlockSchema()},
		"logging_splunk":        {loggingsplunk.NestedBlockSchema(), loggingsplunk.ComputeNestedBlockSchema()},
		"logging_syslog":        {loggingsyslog.NestedBlockSchema(), loggingsyslog.ComputeNestedBlockSchema()},
	}
	for i, family := range []struct{ legacy, target, fixtureDir, version string }{
		{"fastly_service_vcl", "fastly_service_cdn_auto", "servicecdnauto", "v1"},
		{"fastly_service_compute", "fastly_service_compute_auto", "servicecomputeauto", "v0"},
	} {
		t.Run(family.legacy, func(t *testing.T) {
			t.Parallel()
			for _, release := range []string{"_type.json", "_10_3_0_type.json"} {
				filename := "legacy_service_" + strings.TrimPrefix(family.legacy, "fastly_service_") + "_schema_" + family.version + release
				raw, err := os.ReadFile(filepath.Join("..", "..", "resources", family.fixtureDir, "testdata", filename)) // #nosec G304 -- repository-owned schema snapshots
				if err != nil {
					t.Fatal(err)
				}
				var fixture struct {
					Block struct {
						BlockTypes map[string]struct {
							Block struct{ Attributes map[string]json.RawMessage }
						} `json:"block_types"`
					}
				}
				if err := json.Unmarshal(raw, &fixture); err != nil {
					t.Fatal(err)
				}
				registry := compatibility.Default()
				for block, target := range schemas {
					source := fixture.Block.BlockTypes[block].Block.Attributes
					if len(source) == 0 {
						t.Fatalf("legacy fixture lacks %s", block)
					}
					attrs := target[i].NestedObject.Attributes
					for name := range source {
						if !compatibility.TLSLoggingLegacyField(block, name, i == 0) {
							t.Fatalf("legacy %s.%s is not recognized", block, name)
						}
						object, member, moved := compatibility.TLSLoggingObjectRoute(block, name)
						if !moved {
							if attr := attrs[name]; attr == nil || (!attr.IsRequired() && !attr.IsOptional()) {
								t.Fatalf("unmapped configurable field %s.%s", block, name)
							}
							continue
						}
						if attrs[name] != nil {
							t.Fatalf("legacy flat target field %s.%s still exists", block, name)
						}
						nested, ok := attrs[object].(schema.SingleNestedAttribute)
						if !ok || !nested.Optional || !nested.Computed {
							t.Fatalf("target %s.%s is not an optional computed object", block, object)
						}
						field, ok := nested.Attributes[member].(schema.StringAttribute)
						if !ok || !field.Optional || !field.Computed {
							t.Fatalf("target %s.%s.%s is not a configurable string", block, object, member)
						}
						if (member == "client_key" || member == "password" || member == "token") && !field.Sensitive {
							t.Fatal("target credential is not marked sensitive")
						}
						var matched bool
						for _, rule := range registry.RulesForLegacy(family.legacy + "." + block + "." + name) {
							parent, destination, mapping, ok := serviceObjectField(rule, family.legacy, family.target)
							matched = matched || (ok && parent == block && destination == object && mapping.From == name && mapping.To == member)
						}
						if !matched {
							t.Fatalf("missing exact authorized route for %s.%s", block, name)
						}
					}
					// Target-only Splunk request limits must not be accepted
					// as legacy HCL. All other flat target fields are covered.
					for name := range attrs {
						if name == "authentication" || name == "tls" {
							continue
						}
						if _, exists := source[name]; !exists {
							if block != "logging_splunk" || (name != "request_max_bytes" && name != "request_max_entries") ||
								compatibility.TLSLoggingLegacyField(block, name, i == 0) {
								t.Fatalf("unreviewed target-only field %s.%s", block, name)
							}
						}
					}
				}
			}
		})
	}
}

func TestPlanLoggingTLSInvalidElasticsearchURLIsSafe(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"cdn", "compute"} {
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := oneLoggingFixture(t, loggingFixture(t, family, "legacy.tf"), "logging_elasticsearch")
			source = strings.Replace(source, "https://search.example.invalid/", "http://SYNTHETIC_CREDENTIAL@search.example.invalid/", 1)
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatal("accepted incompatible Elasticsearch URL")
			}
			for _, preview := range []struct {
				name   string
				render func(io.Writer, Result) error
			}{
				{"dry-run", WriteText},
				{"write", WriteApplyPreview},
			} {
				var out bytes.Buffer
				if err := preview.render(&out, result); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(out.String(), "SYNTHETIC_CREDENTIAL") {
					t.Fatalf("%s: credential appeared in blocked preview", preview.name)
				}
				if !strings.Contains(out.String(), "Configure an HTTPS Elasticsearch endpoint supported by the target provider.") {
					t.Fatalf("%s: missing safe URL remediation", preview.name)
				}
			}
		})
	}
}
