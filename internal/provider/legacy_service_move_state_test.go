package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These tests exercise the actual Framework MoveResourceState RPC. Calling a
// resource's StateMover directly with a tfsdk.State bypasses JSON decoding, which
// is the compatibility boundary this suite is intended to protect.
type legacyServiceSchema struct {
	Version int64              `json:"version"`
	Block   legacyServiceBlock `json:"block"`
}

type legacyServiceBlock struct {
	Attributes map[string]any                 `json:"attributes"`
	BlockTypes map[string]legacyServiceNested `json:"block_types"`
}

type legacyServiceNested struct {
	NestingMode string             `json:"nesting_mode"`
	Block       legacyServiceBlock `json:"block"`
}

type legacyMoveCase struct {
	source, target, fixtureDir, fixtureBase, fixtureCurrent string
	version                                                 int64
}

var legacyMoveCases = []legacyMoveCase{
	{
		source: "fastly_service_vcl", target: "fastly_service_cdn_auto",
		fixtureDir: "servicecdnauto", fixtureBase: "legacy_service_vcl_schema_v1_type.json",
		fixtureCurrent: "legacy_service_vcl_schema_v1_10_3_0_type.json", version: 1,
	},
	{
		source: "fastly_service_compute", target: "fastly_service_compute_auto",
		fixtureDir: "servicecomputeauto", fixtureBase: "legacy_service_compute_schema_v0_type.json",
		fixtureCurrent: "legacy_service_compute_schema_v0_10_3_0_type.json", version: 0,
	},
}

func readLegacyMoveFixture(t *testing.T, tc legacyMoveCase, file string) legacyServiceSchema {
	t.Helper()

	// These are repository-owned, versioned test fixtures.
	raw, err := os.ReadFile(filepath.Join("..", "resources", tc.fixtureDir, "testdata", file)) // #nosec G304 -- test fixtures
	if err != nil {
		t.Fatal(err)
	}
	var fixture legacyServiceSchema
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != tc.version {
		t.Fatalf("schema version = %d, want %d", fixture.Version, tc.version)
	}
	return fixture
}

// historicalLegacyMoveFixture uses the existing SDKv2 snapshot and the
// source-derived 10.3.0 snapshot. The optional published-schema suite below
// independently checks the assumptions against the actual registry binaries.
func historicalLegacyMoveFixture(t *testing.T, tc legacyMoveCase, release string) legacyServiceSchema {
	t.Helper()

	if release == "9.1.0" || release == "9.5.0" {
		fixture := readLegacyMoveFixture(t, tc, tc.fixtureBase)
		if release == "9.1.0" {
			backend := fixture.Block.BlockTypes["backend"]
			delete(backend.Block.Attributes, "max_lifetime")
			delete(backend.Block.Attributes, "max_use")
		}
		return fixture
	}

	fixture := readLegacyMoveFixture(t, tc, tc.fixtureCurrent)
	if release != "10.3.0" {
		delete(fixture.Block.BlockTypes, "logging_fos")
	}
	if release == "9.8.0" {
		// The published 9.8.0 schemas omit public_key for both services;
		// it is present in the published 10.2.0 and 10.3.0 schemas.
		gcs := fixture.Block.BlockTypes["logging_gcs"]
		delete(gcs.Block.Attributes, "public_key")

		if tc.source == "fastly_service_vcl" {
			backend := fixture.Block.BlockTypes["backend"]
			for _, name := range []string{"iam_vendor", "iam_product", "iam_region", "iam_role", "iam_external_id"} {
				delete(backend.Block.Attributes, name)
			}
		}
	}
	return fixture
}

// legacyBlockValue synthesizes JSON state using the complete source shape,
// rather than the target decoder's schema. Every attribute is represented,
// and unconfigured nested blocks are empty just as in SDKv2 state.
func legacyBlockValue(block legacyServiceBlock) map[string]any {
	value := make(map[string]any, len(block.Attributes)+len(block.BlockTypes))
	for name := range block.Attributes {
		value[name] = nil
	}
	for name, nested := range block.BlockTypes {
		switch nested.NestingMode {
		case "set", "list":
			value[name] = []any{}
		case "map":
			value[name] = map[string]any{}
		default:
			value[name] = nil
		}
	}
	return value
}

func legacyMoveStateValue(fixture legacyServiceSchema, populated bool) map[string]any {
	state := legacyBlockValue(fixture.Block)
	state["id"] = "SID123"
	state["name"] = "schema-migration-test"
	state["activate"] = true
	state["stage"] = false
	state["force_destroy"] = true
	state["reuse"] = false

	backend := legacyBlockValue(fixture.Block.BlockTypes["backend"].Block)
	backend["name"] = "origin"
	backend["address"] = "example.com"
	backend["port"] = 80
	for _, name := range []string{"iam_vendor", "iam_product", "iam_region", "iam_role", "iam_external_id"} {
		if _, exists := backend[name]; exists {
			backend[name] = ""
		}
	}
	for _, name := range []string{"max_lifetime", "max_use"} {
		if _, exists := backend[name]; exists {
			backend[name] = 0
		}
	}
	state["backend"] = []any{backend}

	if !populated {
		return state
	}

	// Exercise the original flat TLS fields at the actual MoveResourceState
	// decoding boundary. API-readable backend values are hydrated by Read;
	// the live backend TLS acceptance test verifies that round trip.
	backend["ssl_client_cert"] = "synthetic-client-cert"
	backend["ssl_client_key"] = "synthetic-client-key"

	// Populated nested objects are important: an empty set would never
	// exercise schema drift inside its element type.
	gcs := legacyBlockValue(fixture.Block.BlockTypes["logging_gcs"].Block)
	gcs["name"] = "gcs-test"
	gcs["bucket_name"] = "synthetic-bucket"
	if _, exists := gcs["public_key"]; exists {
		gcs["public_key"] = "synthetic-public-key"
	}
	state["logging_gcs"] = []any{gcs}
	for _, block := range []string{"logging_https", "logging_elasticsearch", "logging_kafka", "logging_splunk", "logging_syslog"} {
		endpoint := legacyBlockValue(fixture.Block.BlockTypes[block].Block)
		endpoint["name"] = "synthetic-endpoint"
		for _, name := range []string{"tls_ca_cert", "tls_client_cert", "tls_client_key", "tls_hostname", "user", "password", "token", "header_value"} {
			if _, exists := endpoint[name]; exists {
				endpoint[name] = "synthetic-" + name
			}
		}
		if block == "logging_https" {
			endpoint["gzip_level"] = 0
		}
		state[block] = []any{endpoint}
	}

	if fos, exists := fixture.Block.BlockTypes["logging_fos"]; exists {
		endpoint := legacyBlockValue(fos.Block)
		endpoint["name"] = "fos-test"
		endpoint["bucket_name"] = "synthetic-fos"
		endpoint["region"] = "us-east"
		endpoint["period"] = 3600
		if _, exists := endpoint["format"]; exists {
			endpoint["format"] = "%h"
			endpoint["format_version"] = 2
		}
		state["logging_fos"] = []any{endpoint}
	}

	productsSchema := fixture.Block.BlockTypes["product_enablement"].Block
	products := legacyBlockValue(productsSchema)
	products["name"] = "products"
	products["domain_inspector"] = true
	bot := legacyBlockValue(productsSchema.BlockTypes["bot_management"].Block)
	bot["enabled"] = true
	bot["contentguard"] = "off"
	products["bot_management"] = []any{bot}
	state["product_enablement"] = []any{products}

	if pkg, exists := fixture.Block.BlockTypes["package"]; exists {
		pkgState := legacyBlockValue(pkg.Block)
		pkgState["filename"] = "synthetic.tar.gz"
		pkgState["content"] = ""
		pkgState["source_code_hash"] = "synthetic-hash"
		state["package"] = []any{pkgState}
	}
	return state
}

func invokeLegacyMoveRPC(t *testing.T, tc legacyMoveCase, schemaVersion int64, raw []byte) (*tfprotov6.MoveResourceStateResponse, tfprotov6.ProviderServer) {
	t.Helper()

	server, err := providerserver.NewProtocol6WithError(New())()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.MoveResourceState(context.Background(), &tfprotov6.MoveResourceStateRequest{
		SourceProviderAddress: "registry.terraform.io/fastly/fastly",
		SourceTypeName:        tc.source,
		SourceSchemaVersion:   schemaVersion,
		SourceState:           &tfprotov6.RawState{JSON: raw},
		TargetTypeName:        tc.target,
	})
	if err != nil {
		t.Fatalf("MoveResourceState RPC: %v", err)
	}
	if resp == nil {
		t.Fatal("nil MoveResourceState response")
	}
	return resp, server
}

func legacyMoveError(resp *tfprotov6.MoveResourceStateResponse) bool {
	for _, diagnostic := range resp.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

// assertUnpopulatedLegacyMoveBlock verifies that no nested objects were copied
// into the target state. The Framework can represent an unset nested list as
// an empty collection at the protocol boundary rather than a null value.
func assertUnpopulatedLegacyMoveBlock(t *testing.T, attrs map[string]tftypes.Value, name string) {
	t.Helper()

	value, exists := attrs[name]
	if !exists {
		t.Fatalf("missing target block %s", name)
	}
	if !value.IsKnown() {
		t.Fatalf("%s unexpectedly unknown before the first refresh", name)
	}
	if value.IsNull() {
		return
	}
	var elements []tftypes.Value
	if err := value.As(&elements); err != nil {
		t.Fatalf("%s should be an unpopulated nested list before the first refresh: %v", name, err)
	}
	if len(elements) != 0 {
		t.Fatalf("%s should be unpopulated before the first refresh, got %d elements", name, len(elements))
	}
}

func assertLegacyMoveState(t *testing.T, tc legacyMoveCase, fixture legacyServiceSchema, state map[string]any, populated bool) {
	t.Helper()

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	resp, server := invokeLegacyMoveRPC(t, tc, fixture.Version, raw)
	if legacyMoveError(resp) {
		for _, diagnostic := range resp.Diagnostics {
			t.Logf("MoveResourceState diagnostic: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
		t.Fatal("legacy JSON state did not move successfully")
	}

	warned := false
	for _, diagnostic := range resp.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityWarning &&
			diagnostic.Summary == "Legacy product enablement requires standalone resources" {
			warned = true
		}
	}
	if warned != populated {
		t.Fatalf("standalone product warning = %t, want %t", warned, populated)
	}

	schemas, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	schema := schemas.ResourceSchemas[tc.target]
	if schema == nil || resp.TargetState == nil {
		t.Fatal("missing target schema or moved state")
	}
	moved, err := resp.TargetState.Unmarshal(schema.ValueType())
	if err != nil {
		t.Fatal(err)
	}
	var attrs map[string]tftypes.Value
	if err := moved.As(&attrs); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := attrs["id"].As(&id); err != nil || id != "SID123" {
		t.Fatalf("service id = %q, error = %v", id, err)
	}
	for _, name := range []string{"force_destroy", "reuse"} {
		var actual bool
		expected, _ := state[name].(bool)
		if attrs[name].IsNull() {
			t.Fatalf("%s unexpectedly null after move", name)
		}
		if err := attrs[name].As(&actual); err != nil || actual != expected {
			t.Fatalf("%s = %t, want %t (error = %v)", name, actual, expected, err)
		}
	}
	// API-readable blocks are intentionally reconstructed by Read, not copied.
	for _, name := range []string{"backend", "logging_gcs", "logging_fos", "logging_elasticsearch", "logging_kafka", "logging_splunk", "logging_syslog"} {
		assertUnpopulatedLegacyMoveBlock(t, attrs, name)
	}
	if endpoints, _ := state["logging_https"].([]any); len(endpoints) == 0 {
		assertUnpopulatedLegacyMoveBlock(t, attrs, "logging_https")
	} else {
		var models []tftypes.Value
		if err := attrs["logging_https"].As(&models); err != nil || len(models) != 1 {
			t.Fatal("missing HTTPS gzip hint")
		}
		var fields map[string]tftypes.Value
		if err := models[0].As(&fields); err != nil {
			t.Fatal(err)
		}
		var name string
		if err := fields["name"].As(&name); err != nil || name != "synthetic-endpoint" {
			t.Fatal("HTTPS endpoint name hint not preserved")
		}
		if !fields["gzip_level"].Equal(tftypes.NewValue(tftypes.Number, big.NewFloat(0))) {
			t.Fatal("explicit HTTPS gzip_level=0 was not preserved")
		}
		for field, value := range fields {
			if field != "name" && field != "gzip_level" && !value.IsNull() {
				t.Fatalf("HTTPS field %s was copied instead of refreshed", field)
			}
		}
	}
	if tc.source == "fastly_service_compute" {
		if len(resp.TargetPrivate) == 0 {
			t.Fatal("Compute move did not return package-refresh private state")
		}
		sourcePackages, _ := state["package"].([]any)
		if len(sourcePackages) == 0 {
			assertUnpopulatedLegacyMoveBlock(t, attrs, "package")
			return
		}
		var packages []tftypes.Value
		if err := attrs["package"].As(&packages); err != nil || len(packages) != 1 {
			t.Fatalf("moved package missing or invalid: %v", err)
		}
		var fields map[string]tftypes.Value
		if err := packages[0].As(&fields); err != nil {
			t.Fatal(err)
		}
		input := sourcePackages[0].(map[string]any)
		for _, name := range []string{"filename", "content", "source_code_hash"} {
			want, _ := input[name].(string)
			if want == "" {
				if !fields[name].IsNull() {
					t.Fatalf("omitted package %s should be null", name)
				}
				continue
			}
			var got string
			if err := fields[name].As(&got); err != nil || got != want {
				// Do not print local package inputs, even with synthetic fixtures.
				t.Fatalf("package %s was not preserved", name)
			}
		}
	}
}

func TestLegacyServiceMoveStateHistoricalSchemas(t *testing.T) {
	t.Parallel()

	for _, tc := range legacyMoveCases {
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			releases := []string{"9.5.0", "9.8.0", "10.2.0", "10.3.0"}
			if tc.source == "fastly_service_vcl" {
				releases = append([]string{"9.1.0"}, releases...)
			}
			for _, release := range releases {
				t.Run(release, func(t *testing.T) {
					t.Parallel()
					fixture := historicalLegacyMoveFixture(t, tc, release)
					for _, populated := range []bool{false, true} {
						t.Run(fmt.Sprintf("populated=%t", populated), func(t *testing.T) {
							assertLegacyMoveState(t, tc, fixture, legacyMoveStateValue(fixture, populated), populated)
						})
					}
				})
			}
		})
	}
}

func TestLegacyServiceMoveStateAbsentAndNullValues(t *testing.T) {
	t.Parallel()

	for _, tc := range legacyMoveCases {
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			fixture := readLegacyMoveFixture(t, tc, tc.fixtureCurrent)
			t.Run("reuse enabled", func(t *testing.T) {
				state := legacyMoveStateValue(fixture, false)
				state["force_destroy"] = false
				state["reuse"] = true
				assertLegacyMoveState(t, tc, fixture, state, false)
			})
			for _, absent := range []bool{false, true} {
				t.Run(fmt.Sprintf("absent=%t", absent), func(t *testing.T) {
					state := legacyMoveStateValue(fixture, false)
					for _, name := range []string{"force_destroy", "reuse", "logging_fos", "product_enablement"} {
						if absent {
							delete(state, name)
						} else {
							state[name] = nil
						}
					}
					backend := state["backend"].([]any)[0].(map[string]any)
					for _, name := range []string{"iam_vendor", "iam_product", "iam_region", "iam_role", "iam_external_id"} {
						if _, exists := backend[name]; !exists {
							continue
						}
						if absent {
							delete(backend, name)
						} else {
							backend[name] = nil
						}
					}
					assertLegacyMoveState(t, tc, fixture, state, false)
				})
			}
		})
	}
}

func TestLegacyServiceMoveStateComputePackageInputs(t *testing.T) {
	t.Parallel()

	tc := legacyMoveCases[1]
	fixture := readLegacyMoveFixture(t, tc, tc.fixtureCurrent)
	for _, c := range []struct {
		name, filename, content string
	}{
		{"filename", "synthetic.tar.gz", ""},
		{"content", "", "c3ludGhldGlj"},
		{"imported hash only", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			state := legacyMoveStateValue(fixture, true)
			pkg := state["package"].([]any)[0].(map[string]any)
			pkg["filename"] = c.filename
			pkg["content"] = c.content
			assertLegacyMoveState(t, tc, fixture, state, true)
		})
	}
}

func TestLegacyServiceMoveStateRejectsUnsafeJSON(t *testing.T) {
	t.Parallel()

	for _, tc := range legacyMoveCases {
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			fixture := readLegacyMoveFixture(t, tc, tc.fixtureCurrent)
			cases := []struct {
				name string
				edit func(map[string]any)
			}{
				{"missing ID", func(s map[string]any) { delete(s, "id") }},
				{"empty ID", func(s map[string]any) { s["id"] = "" }},
				{"invalid backend shape", func(s map[string]any) { s["backend"] = "not a set" }},
				{"invalid FOS shape", func(s map[string]any) { s["logging_fos"] = true }},
				{"invalid GCS element", func(s map[string]any) { s["logging_gcs"] = []any{42} }},
			}
			if tc.source == "fastly_service_vcl" {
				for _, name := range []string{"iam_vendor", "iam_product", "iam_region", "iam_role", "iam_external_id"} {
					cases = append(cases, struct {
						name string
						edit func(map[string]any)
					}{
						name: "configured " + name,
						edit: func(s map[string]any) {
							s["backend"].([]any)[0].(map[string]any)[name] = "synthetic-IAM-setting"
						},
					})
				}
			}
			if tc.source == "fastly_service_compute" {
				cases = append(cases, struct {
					name string
					edit func(map[string]any)
				}{
					name: "ambiguous package inputs",
					edit: func(s map[string]any) {
						pkg := legacyBlockValue(fixture.Block.BlockTypes["package"].Block)
						pkg["filename"] = "synthetic.tar.gz"
						pkg["content"] = "c3ludGhldGlj"
						s["package"] = []any{pkg}
					},
				})
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					state := legacyMoveStateValue(fixture, false)
					c.edit(state)
					raw, err := json.Marshal(state)
					if err != nil {
						t.Fatal(err)
					}
					resp, _ := invokeLegacyMoveRPC(t, tc, fixture.Version, raw)
					if !legacyMoveError(resp) {
						t.Fatal("unsafe state was accepted")
					}
					if strings.HasPrefix(c.name, "configured iam_") {
						found := false
						for _, diagnostic := range resp.Diagnostics {
							if diagnostic.Summary == "Unsupported legacy backend IAM authentication state" {
								found = true
							}
							if strings.Contains(diagnostic.Summary, "synthetic-IAM-setting") ||
								strings.Contains(diagnostic.Detail, "synthetic-IAM-setting") {
								t.Fatal("IAM setting leaked into diagnostics")
							}
						}
						if !found {
							t.Fatal("expected the explicit unsupported-IAM diagnostic")
						}
					}
				})
			}
			t.Run("unsupported schema version", func(t *testing.T) {
				raw, err := json.Marshal(legacyMoveStateValue(fixture, false))
				if err != nil {
					t.Fatal(err)
				}
				resp, _ := invokeLegacyMoveRPC(t, tc, fixture.Version+1, raw)
				if !legacyMoveError(resp) {
					t.Fatal("unsupported source schema version was accepted")
				}
			})
		})
	}
}

// Published Terraform schema output attaches metadata to attribute types.
// Normalize it to the same type-only shape as the checked-in test fixtures.
func normalizePublishedLegacyBlock(t *testing.T, block legacyServiceBlock) legacyServiceBlock {
	t.Helper()

	for name, value := range block.Attributes {
		metadata, ok := value.(map[string]any)
		if !ok || metadata["type"] == nil {
			t.Fatalf("published schema attribute %s has no Terraform type", name)
		}
		block.Attributes[name] = metadata["type"]
	}
	for name, nested := range block.BlockTypes {
		nested.Block = normalizePublishedLegacyBlock(t, nested.Block)
		block.BlockTypes[name] = nested
	}
	return block
}

func readPublishedLegacySchema(t *testing.T, dir, release, resourceType string) legacyServiceSchema {
	t.Helper()

	// The opt-in shell harness generates this file from terraform providers schema.
	raw, err := os.ReadFile(filepath.Join(dir, release+".json")) // #nosec G304 G703 -- explicit test schema directory and fixed release names
	if err != nil {
		t.Fatal(err)
	}
	var schemas struct {
		Providers map[string]struct {
			Resources map[string]legacyServiceSchema `json:"resource_schemas"`
		} `json:"provider_schemas"`
	}
	if err := json.Unmarshal(raw, &schemas); err != nil {
		t.Fatal(err)
	}
	fixture, exists := schemas.Providers["registry.terraform.io/fastly/fastly"].Resources[resourceType]
	if !exists {
		t.Fatalf("%s %s schema missing from published provider", release, resourceType)
	}
	fixture.Block = normalizePublishedLegacyBlock(t, fixture.Block)
	return fixture
}

// comparePublishedLegacyBlock reports individual shape differences without
// dumping the entire provider schema. Nil and empty maps are equivalent.
func comparePublishedLegacyBlock(t *testing.T, prefix string, actual, expected legacyServiceBlock) {
	t.Helper()

	for name, want := range expected.Attributes {
		got, exists := actual.Attributes[name]
		if !exists || !reflect.DeepEqual(got, want) {
			t.Errorf("%s.%s: published type = %v (present=%t), fixture type = %v", prefix, name, got, exists, want)
		}
	}
	for name := range actual.Attributes {
		if _, exists := expected.Attributes[name]; !exists {
			t.Errorf("%s.%s: published attribute missing from fixture", prefix, name)
		}
	}
	for name, want := range expected.BlockTypes {
		got, exists := actual.BlockTypes[name]
		if !exists {
			t.Errorf("%s.%s: fixture block missing from published schema", prefix, name)
			continue
		}
		if got.NestingMode != want.NestingMode {
			t.Errorf("%s.%s: published nesting = %s, fixture nesting = %s", prefix, name, got.NestingMode, want.NestingMode)
		}
		comparePublishedLegacyBlock(t, prefix+"."+name, got.Block, want.Block)
	}
	for name := range actual.BlockTypes {
		if _, exists := expected.BlockTypes[name]; !exists {
			t.Errorf("%s.%s: published block missing from fixture", prefix, name)
		}
	}
}

// This opt-in test uses exact schemas exported from real registry provider
// binaries. It requires no Fastly credentials and creates no infrastructure.
func TestLegacyServiceMoveStatePublishedSchemas(t *testing.T) {
	t.Parallel()

	dir := os.Getenv("FASTLY_LEGACY_SCHEMA_DIR")
	if dir == "" {
		t.Skip("run scripts/test-migration-state-schemas/run.sh to capture published schemas")
	}
	for _, tc := range legacyMoveCases {
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			releases := []string{"9.5.0", "9.8.0", "10.2.0", "10.3.0"}
			if tc.source == "fastly_service_vcl" {
				releases = append([]string{"9.1.0"}, releases...)
			}
			for _, release := range releases {
				t.Run(release, func(t *testing.T) {
					t.Parallel()
					actual := readPublishedLegacySchema(t, dir, release, tc.source)
					if actual.Version != tc.version {
						t.Fatalf("published schema version = %d, expected %d", actual.Version, tc.version)
					}
					expected := historicalLegacyMoveFixture(t, tc, release)
					comparePublishedLegacyBlock(t, tc.source, actual.Block, expected.Block)
					assertLegacyMoveState(t, tc, actual, legacyMoveStateValue(actual, false), false)
					assertLegacyMoveState(t, tc, actual, legacyMoveStateValue(actual, true), true)
				})
			}
		})
	}
}
