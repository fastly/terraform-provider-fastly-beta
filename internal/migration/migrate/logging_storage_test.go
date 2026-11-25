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

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingbigquery"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingblobstorage"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingcloudfiles"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingdigitalocean"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginggcs"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginggooglepubsub"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingkinesis"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingopenstack"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggings3"
)

var storageLoggingBlocks = []string{
	"logging_s3", "logging_gcs", "logging_bigquery", "logging_blobstorage",
	"logging_cloudfiles", "logging_digitalocean", "logging_openstack",
	"logging_googlepubsub", "logging_kinesis",
}

func storageFixture(t *testing.T, family, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "logging_storage", family, filename)) // #nosec G304 -- repository-owned fixtures
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Keep both sides of the fixture independently authored. Remove only other
// endpoint blocks; do not generate expected HCL through the implementation.
func oneStorageFixture(t *testing.T, source, endpoint string) string {
	t.Helper()
	if endpoint == "all" {
		return source
	}
	blocks := loggingFixtureResource(t, source).Body.Blocks
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		if !compatibility.IsStorageLoggingBlock(b.Type) || b.Type == endpoint {
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

func TestPlanStorageLoggingGolden(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"cdn", "compute"} {
		for _, endpoint := range append([]string{"all"}, storageLoggingBlocks...) {
			t.Run(family+"/"+endpoint, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				source := oneStorageFixture(t, storageFixture(t, family, "legacy.tf"), endpoint)
				want := oneStorageFixture(t, storageFixture(t, family, "expected.tf"), endpoint)
				writeConfig(t, dir, "main.tf", source)
				plan := newPlan(t, dir)
				if plan.Blocked() || len(plan.Moves) != 1 {
					t.Fatalf("storage migration blocked: issues=%v findings=%v", plan.Issues, plan.Analysis.Findings)
				}
				got := changedFile(t, plan, "main.tf")
				if got != want {
					// No raw HCL in test failure: fixtures may contain credentials.
					t.Fatalf("storage migration golden mismatch: %s/%s", family, endpoint)
				}
				loggingFixtureResource(t, got)
				for _, render := range []func(io.Writer, Result) error{WriteText, WriteApplyPreview} {
					var out bytes.Buffer
					if err := render(&out, plan); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(out.String(), "+++ b/") || !strings.Contains(out.String(), "HCL diff withheld") {
						t.Fatal("sensitive storage HCL preview exposed or lacked safety notice")
					}
				}
				original, err := os.ReadFile(filepath.Join(dir, "main.tf"))
				if err != nil || string(original) != source {
					t.Fatal("planning mutated legacy HCL")
				}
				requireStableAfterApply(t, dir, plan)
				applied, err := os.ReadFile(filepath.Join(dir, "main.tf"))
				if err != nil || string(applied) != want {
					t.Fatal("write did not match independent golden HCL")
				}
			})
		}
	}
}

func TestPlanStorageLoggingRejectsUnsafeShapes(t *testing.T) {
	t.Parallel()
	cases := []struct{ block, extra, expect string }{
		{"logging_s3", "authentication = {}", "authentication"},
		{"logging_s3", "s3_iam_role = \"role\"", "s3_iam_role"},
		{"logging_gcs", "account_name = \"service\"", "account_name"},
		{"logging_bigquery", "account_name = \"service\"", "account_name"},
		{"logging_bigquery", "format_version = 2", "format_version"},
		{"logging_googlepubsub", "account_name = \"service\"", "account_name"},
		{"logging_blobstorage", "authentication = local.secret", "authentication"},
		{"logging_cloudfiles", "unknown_secret = \"SYNTHETIC_STORAGE_SECRET\"", "unknown_secret"},
		{"logging_digitalocean", "authentication {}", "authentication"},
		{"logging_openstack", "access_key_extra = \"SYNTHETIC_STORAGE_SECRET\"", "access_key_extra"},
		{"logging_kinesis", "iam_role = \"role\"", "iam_role"},
		{"logging_s3", "gzip_level = -1", "gzip_level"},
		{"logging_gcs", "compression_codec = \"gzip\"\n    gzip_level = 3", "gzip_level"},
		{"logging_blobstorage", "gzip_level = var.gzip_level\n    compression_codec = var.codec", "gzip_level"},
		{"logging_s3", "placement = \"invalid\"", "placement"},
	}
	for _, tc := range cases {
		t.Run(tc.block+"/"+tc.expect, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := oneStorageFixture(t, storageFixture(t, "cdn", "legacy.tf"), tc.block)
			// Insert directly after the chosen block's opening brace.
			marker := "  " + tc.block + " {\n"
			if !strings.Contains(source, marker) {
				t.Fatal("fixture missing endpoint")
			}
			source = strings.Replace(source, marker, marker+"    "+tc.extra+"\n", 1)
			writeConfig(t, dir, "main.tf", source)
			plan := newPlan(t, dir)
			if !plan.Blocked() || len(plan.Moves) != 0 || len(plan.Changes) != 0 {
				t.Fatal("unsafe storage plan was accepted")
			}
			var out bytes.Buffer
			if err := WriteText(&out, plan); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.expect) || strings.Contains(out.String(), "SYNTHETIC_STORAGE_SECRET") {
				t.Fatal("safe diagnostic missing or leaked sensitive value")
			}
			if _, err := plan.Apply(context.Background(), privateBackupDir(t)); err == nil {
				t.Fatal("blocked migration applied")
			}
			actual, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(actual) != source {
				t.Fatal("blocked migration changed source")
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
				t.Fatal("blocked migration created migration.tf")
			}
		})
	}
}

func TestPlanStorageLoggingPreservesExpressionPlacement(t *testing.T) {
	t.Parallel()
	source := oneStorageFixture(t, storageFixture(t, "cdn", "legacy.tf"), "logging_s3")
	source = strings.Replace(source, "  logging_s3 {\n", "  logging_s3 {\n    placement = var.placement\n", 1)
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("unresolved placement should be left for target validation: %v %v", plan.Issues, plan.Analysis.Findings)
	}
	if !strings.Contains(changedFile(t, plan, "main.tf"), "placement = var.placement") {
		t.Fatal("placement expression was lost")
	}
}

func TestStorageLoggingRoutesMatchPublishedSchemas(t *testing.T) {
	t.Parallel()
	targets := map[string][2]schema.ListNestedBlock{
		"logging_s3":           {loggings3.NestedBlockSchema(), loggings3.ComputeNestedBlockSchema()},
		"logging_gcs":          {logginggcs.NestedBlockSchema(), logginggcs.ComputeNestedBlockSchema()},
		"logging_bigquery":     {loggingbigquery.NestedBlockSchema(), loggingbigquery.ComputeNestedBlockSchema()},
		"logging_blobstorage":  {loggingblobstorage.NestedBlockSchema(), loggingblobstorage.ComputeNestedBlockSchema()},
		"logging_cloudfiles":   {loggingcloudfiles.NestedBlockSchema(), loggingcloudfiles.ComputeNestedBlockSchema()},
		"logging_digitalocean": {loggingdigitalocean.NestedBlockSchema(), loggingdigitalocean.ComputeNestedBlockSchema()},
		"logging_openstack":    {loggingopenstack.NestedBlockSchema(), loggingopenstack.ComputeNestedBlockSchema()},
		"logging_googlepubsub": {logginggooglepubsub.NestedBlockSchema(), logginggooglepubsub.ComputeNestedBlockSchema()},
		"logging_kinesis":      {loggingkinesis.NestedBlockSchema(), loggingkinesis.ComputeNestedBlockSchema()},
	}
	families := []struct{ legacy, target, fixtureDir, version string }{
		{"fastly_service_vcl", "fastly_service_cdn_auto", "servicecdnauto", "v1"},
		{"fastly_service_compute", "fastly_service_compute_auto", "servicecomputeauto", "v0"},
	}
	for i, family := range families {
		t.Run(family.legacy, func(t *testing.T) {
			t.Parallel()
			for _, release := range []string{"_type.json", "_10_3_0_type.json"} {
				filename := "legacy_service_" + strings.TrimPrefix(family.legacy, "fastly_service_") + "_schema_" + family.version + release
				data, err := os.ReadFile(filepath.Join("..", "..", "resources", family.fixtureDir, "testdata", filename)) // #nosec G304 -- repository-owned schema snapshots
				if err != nil {
					t.Fatal(err)
				}
				var legacy struct {
					Block struct {
						BlockTypes map[string]struct {
							Block struct{ Attributes map[string]json.RawMessage }
						} `json:"block_types"`
					}
				}
				if err := json.Unmarshal(data, &legacy); err != nil {
					t.Fatal(err)
				}
				for block, schemas := range targets {
					source := legacy.Block.BlockTypes[block].Block.Attributes
					if len(source) == 0 {
						t.Fatalf("missing legacy schema for %s", block)
					}
					target := schemas[i].NestedObject.Attributes
					for name := range source {
						if !compatibility.StorageLoggingLegacyField(block, name, i == 0) {
							t.Fatalf("unreviewed legacy field %s.%s", block, name)
						}
						object, member, moved := compatibility.StorageLoggingObjectRoute(block, name)
						if !moved {
							if attr := target[name]; attr == nil || (!attr.IsRequired() && !attr.IsOptional()) {
								t.Fatalf("no configurable target field %s.%s", block, name)
							}
							continue
						}
						if target[name] != nil {
							t.Fatalf("legacy credential field remains flat in target: %s.%s", block, name)
						}
						nested, ok := target[object].(schema.SingleNestedAttribute)
						if !ok || (!nested.Optional && !nested.Required) {
							t.Fatalf("target %s.%s missing object", block, object)
						}
						memberAttr, ok := nested.Attributes[member].(schema.StringAttribute)
						if !ok || (!memberAttr.Optional && !memberAttr.Required) {
							t.Fatalf("invalid target auth member %s.%s.%s", block, object, member)
						}
						matched := false
						for _, rule := range compatibility.Default().RulesForLegacy(family.legacy + "." + block + "." + name) {
							p, o, f, ok := serviceObjectField(rule, family.legacy, family.target)
							if ok && p == block && o == object && f.From == name && f.To == member {
								matched = true
							}
						}
						if !matched {
							t.Fatalf("missing authorized source-to-target route %s.%s", block, name)
						}
					}
					// format_version is a Framework CDN (VCL) option, not
					// a Compute attribute. The legacy provider had neither.
					if block == "logging_bigquery" && (target["format_version"] != nil) != (i == 0) {
						t.Fatalf("%s: unexpected BigQuery format_version target schema", family.target)
					}
				}
			}
		})
	}
}

func TestStorageLoggingGoldenSyntax(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"cdn", "compute"} {
		for _, kind := range []string{"legacy.tf", "expected.tf"} {
			source := storageFixture(t, family, kind)
			block := loggingFixtureResource(t, source)
			n := 0
			for _, nested := range block.Body.Blocks {
				if compatibility.IsStorageLoggingBlock(nested.Type) {
					n++
				}
			}
			if n != len(storageLoggingBlocks) {
				t.Fatalf("%s/%s: expected %d storage blocks, got %d", family, kind, len(storageLoggingBlocks), n)
			}
			_, diag := hclsyntax.ParseConfig([]byte(source), fmt.Sprintf("%s/%s", family, kind), hcl.Pos{Line: 1, Column: 1})
			if diag.HasErrors() {
				t.Fatalf("%s/%s has invalid HCL", family, kind)
			}
		}
	}
}

// TestPlanStorageLoggingPinsVCLDefaults verifies that an omitted or literal
// null format is replaced with the exact reviewed SDKv2 bytes, including
// BigQuery's different timestamp format.
func TestPlanStorageLoggingPinsVCLDefaults(t *testing.T) {
	t.Parallel()
	for _, block := range storageLoggingBlocks {
		for _, literalNull := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/null=%t", block, literalNull), func(t *testing.T) {
				t.Parallel()
				source := oneStorageFixture(t, storageFixture(t, "cdn", "legacy.tf"), block)
				replacement := ""
				if literalNull {
					replacement = "    format = null\n"
				}
				source = strings.Replace(source, "    format = var.format\n", replacement, 1)
				dir := t.TempDir()
				writeConfig(t, dir, "main.tf", source)
				plan := newPlan(t, dir)
				if plan.Blocked() {
					t.Fatalf("pinning default blocked: %v %v", plan.Issues, plan.Analysis.Findings)
				}
				after := changedFile(t, plan, "main.tf")
				resource := loggingFixtureResource(t, after)
				for _, b := range resource.Body.Blocks {
					if b.Type != block {
						continue
					}
					a := b.Body.Attributes["format"]
					if a == nil {
						t.Fatal("missing pinned log format")
					}
					value, diags := a.Expr.Value(nil)
					if diags.HasErrors() || !value.IsKnown() || value.IsNull() ||
						value.AsString() != legacyVCLLoggingFormat(block) {
						t.Fatal("default format does not match legacy bytes")
					}
				}
				requireStableAfterApply(t, dir, plan)
			})
		}
	}
}

// SDKv2 defaults S3 and Cloud Files message_type to "classic", while the
// Framework target defaults both to "blank". Pin that exact legacy value in
// CDN and Compute without overwriting explicitly configured expressions.
func TestPlanStorageLoggingPinsLegacyMessageType(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"cdn", "compute"} {
		for _, block := range []string{"logging_s3", "logging_cloudfiles"} {
			for _, tc := range []struct {
				name string
				expr string
				want string
			}{
				{name: "omitted", want: "classic"},
				{name: "literal-null", expr: "null", want: "classic"},
				{name: "explicit-value", expr: `"logplex"`, want: "logplex"},
				{name: "expression", expr: "var.message_type"},
			} {
				t.Run(family+"/"+block+"/"+tc.name, func(t *testing.T) {
					t.Parallel()
					source := oneStorageFixture(t, storageFixture(t, family, "legacy.tf"), block)
					if tc.expr != "" {
						needle := "  " + block + " {\n"
						if !strings.Contains(source, needle) {
							t.Fatal("missing legacy logging block")
						}
						source = strings.Replace(source, needle, needle+"    message_type = "+tc.expr+"\n", 1)
					}
					dir := t.TempDir()
					writeConfig(t, dir, "main.tf", source)
					plan := newPlan(t, dir)
					if plan.Blocked() {
						t.Fatalf("message type migration blocked: %v %v", plan.Issues, plan.Analysis.Findings)
					}
					after := changedFile(t, plan, "main.tf")
					var found bool
					for _, b := range loggingFixtureResource(t, after).Body.Blocks {
						if b.Type != block {
							continue
						}
						found = true
						attr := b.Body.Attributes["message_type"]
						if attr == nil {
							t.Fatal("missing pinned or explicit message type")
						}
						if tc.name == "expression" {
							rng := attr.Expr.Range()
							if after[rng.Start.Byte:rng.End.Byte] != tc.expr {
								t.Fatal("explicit message type expression was changed")
							}
						} else {
							value, diags := attr.Expr.Value(nil)
							if diags.HasErrors() || !value.IsKnown() || value.IsNull() ||
								value.AsString() != tc.want {
								t.Fatal("unexpected migrated message_type")
							}
						}
					}
					if !found {
						t.Fatal("missing migrated logging endpoint")
					}
					requireStableAfterApply(t, dir, plan)
				})
			}
		}
	}
}

func TestPlanStorageLoggingRepeatedEndpoints(t *testing.T) {
	t.Parallel()
	source := oneStorageFixture(t, storageFixture(t, "cdn", "legacy.tf"), "logging_s3")
	block := loggingFixtureResource(t, source).Body.Blocks
	var s3 *hclsyntax.Block
	for _, b := range block {
		if b.Type == "logging_s3" {
			s3 = b
			break
		}
	}
	if s3 == nil {
		t.Fatal("fixture missing S3 endpoint")
	}
	rng := s3.Range()
	repeated := strings.Replace(source[rng.Start.Byte:rng.End.Byte], `name = "s3"`, `name = "s3-secondary"`, 1)
	source = source[:rng.End.Byte] + "\n  " + repeated + source[rng.End.Byte:]
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("repeated S3 endpoint blocked: %v %v", plan.Issues, plan.Analysis.Findings)
	}
	after := changedFile(t, plan, "main.tf")
	count := 0
	for _, b := range loggingFixtureResource(t, after).Body.Blocks {
		if b.Type != "logging_s3" {
			continue
		}
		count++
		if b.Body.Attributes["authentication"] == nil || b.Body.Attributes["s3_access_key"] != nil {
			t.Fatal("repeated endpoint lacks independently nested authentication")
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 S3 endpoints, got %d", count)
	}
	requireStableAfterApply(t, dir, plan)
}

func TestPlanStorageLoggingNeverPrintsLiteralCredential(t *testing.T) {
	t.Parallel()
	source := oneStorageFixture(t, storageFixture(t, "cdn", "legacy.tf"), "logging_s3")
	source = strings.Replace(source, "s3_secret_key = trimspace(var.secret)", `s3_secret_key = "SYNTHETIC_STORAGE_SECRET"`, 1)
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", source)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("synthetic secret migration blocked: %v %v", plan.Issues, plan.Analysis.Findings)
	}
	for _, render := range []func(io.Writer, Result) error{WriteText, WriteApplyPreview} {
		var out bytes.Buffer
		if err := render(&out, plan); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "SYNTHETIC_STORAGE_SECRET") ||
			strings.Contains(out.String(), "+++ b/") || !strings.Contains(out.String(), "HCL diff withheld") {
			t.Fatal("sensitive source content leaked in migration preview")
		}
	}
}

func TestPlanStorageLoggingAlternativeAuthModes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		block    string
		removals []string
		insert   string
		member   string
	}{
		{"logging_s3", []string{
			"    s3_access_key = var.key\n",
			"    s3_secret_key = trimspace(var.secret)\n",
		}, "    s3_iam_role = \"role-arn\"\n", "iam_role"},
		{"logging_kinesis", []string{
			"    access_key = var.key\n",
			"    secret_key = var.secret\n",
		}, "    iam_role = \"role-arn\"\n", "iam_role"},
		{"logging_gcs", []string{
			"    user = var.email\n",
			"    secret_key = var.secret\n",
		}, "    account_name = \"service-acct\"\n", "account_name"},
		{"logging_bigquery", []string{
			"    email = var.email\n",
			"    secret_key = var.secret\n",
		}, "    account_name = \"service-acct\"\n", "account_name"},
		{"logging_googlepubsub", []string{
			"    user = var.email\n",
			"    secret_key = var.secret\n",
		}, "    account_name = \"service-acct\"\n", "account_name"},
	}
	for _, tc := range cases {
		t.Run(tc.block, func(t *testing.T) {
			t.Parallel()
			source := oneStorageFixture(t, storageFixture(t, "cdn", "legacy.tf"), tc.block)
			for _, old := range tc.removals {
				if !strings.Contains(source, old) {
					t.Fatal("test fixture drift")
				}
				source = strings.Replace(source, old, "", 1)
			}
			marker := "  " + tc.block + " {\n"
			if !strings.Contains(source, marker) {
				t.Fatal("test fixture missing block")
			}
			source = strings.Replace(source, marker, marker+tc.insert, 1)
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", source)
			plan := newPlan(t, dir)
			if plan.Blocked() {
				t.Fatalf("alternative authentication blocked: %v %v", plan.Issues, plan.Analysis.Findings)
			}
			target := changedFile(t, plan, "main.tf")
			for _, b := range loggingFixtureResource(t, target).Body.Blocks {
				if b.Type != tc.block {
					continue
				}
				a := b.Body.Attributes["authentication"]
				if a == nil {
					t.Fatal("missing nested authentication object")
				}
				raw := target[a.Expr.Range().Start.Byte:a.Expr.Range().End.Byte]
				if !strings.Contains(raw, tc.member+" = ") {
					t.Fatal("wrong authentication member")
				}
			}
			requireStableAfterApply(t, dir, plan)
		})
	}
}

// Unlike Kinesis, these endpoints let a missing credential field take its
// value from an environment default in the target Framework schema.
func TestPlanStorageLoggingAllowsEnvBackedPartialCredentials(t *testing.T) {
	t.Parallel()
	cases := []struct{ block, remove string }{
		{"logging_s3", "    s3_secret_key = trimspace(var.secret)\n"},
		{"logging_gcs", "    secret_key = var.secret\n"},
		{"logging_bigquery", "    secret_key = var.secret\n"},
		{"logging_blobstorage", "    sas_token = var.secret\n"},
		{"logging_googlepubsub", "    secret_key = var.secret\n"},
	}
	for _, tc := range cases {
		t.Run(tc.block, func(t *testing.T) {
			t.Parallel()
			source := oneStorageFixture(t, storageFixture(t, "cdn", "legacy.tf"), tc.block)
			if !strings.Contains(source, tc.remove) {
				t.Fatal("fixture drift")
			}
			source = strings.Replace(source, tc.remove, "", 1)
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", source)
			plan := newPlan(t, dir)
			if plan.Blocked() || len(plan.Moves) != 1 {
				t.Fatalf("environment-backed partial credentials were blocked: %v %v", plan.Issues, plan.Analysis.Findings)
			}
			if !strings.Contains(changedFile(t, plan, "main.tf"), "authentication = {") {
				t.Fatal("remaining configured credential was not migrated")
			}
			requireStableAfterApply(t, dir, plan)
		})
	}
}

func TestPlanStorageLoggingRejectsPartialCredentialsAndMissingRequired(t *testing.T) {
	t.Parallel()
	cases := []struct{ block, remove, expect string }{
		{"logging_kinesis", "    secret_key = var.secret\n", "access_key"},
		{"logging_cloudfiles", "    bucket_name = \"logs\"\n", "bucket_name"},
		{"logging_digitalocean", "    secret_key = var.secret\n", "secret_key"},
		{"logging_openstack", "    url = \"https://storage.example.invalid\"\n", "url"},
	}
	for _, tc := range cases {
		t.Run(tc.block, func(t *testing.T) {
			t.Parallel()
			source := oneStorageFixture(t, storageFixture(t, "cdn", "legacy.tf"), tc.block)
			if !strings.Contains(source, tc.remove) {
				t.Fatal("fixture drift")
			}
			source = strings.Replace(source, tc.remove, "", 1)
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", source)
			plan := newPlan(t, dir)
			if !plan.Blocked() || len(plan.Moves) != 0 || len(plan.Changes) != 0 {
				t.Fatal("partial credentials or missing required field accepted")
			}
			var out bytes.Buffer
			if err := WriteText(&out, plan); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.expect) {
				t.Fatal("missing actionable option name")
			}
			original, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(original) != source {
				t.Fatal("failed plan modified input")
			}
		})
	}
}
