package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafmovestate"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These tests go through the MoveResourceState RPC so that legacy JSON state
// is decoded by the same path Terraform uses for a moved block.

const (
	ngwafMoveID          = "OBJ123"
	ngwafMoveWorkspaceID = "WS123"
)

type ngwafMoveCase struct {
	source ngwafmovestate.Source
	kind   string // legacy type value; empty for plain renames
	target string
}

func ngwafMoveCases() []ngwafMoveCase {
	var cases []ngwafMoveCase
	for _, source := range ngwafmovestate.Sources() {
		if source.Targets == nil {
			cases = append(cases, ngwafMoveCase{source: source, target: source.Target})
			continue
		}
		for _, kind := range sortedStrings(source.Targets) {
			cases = append(cases, ngwafMoveCase{source: source, kind: kind, target: source.Targets[kind]})
		}
	}
	return cases
}

func readNGWAFLegacyFixture(t *testing.T, source ngwafmovestate.Source) legacyServiceSchema {
	t.Helper()

	name := "legacy_" + strings.TrimPrefix(source.TypeName, "fastly_ngwaf_") + "_schema_v0_10_3_0_type.json"
	raw, err := os.ReadFile(filepath.Join("..", "ngwafmovestate", "testdata", name)) // #nosec G304 -- test fixtures
	if err != nil {
		t.Fatal(err)
	}
	var fixture legacyServiceSchema
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// populatedLegacyBlock fills every attribute with a type-correct sample and
// every nested block with one element, so decoding covers each nested type.
func populatedLegacyBlock(t *testing.T, block legacyServiceBlock) map[string]any {
	t.Helper()

	value := make(map[string]any, len(block.Attributes)+len(block.BlockTypes))
	for name, raw := range block.Attributes {
		value[name] = sampleLegacyValue(t, raw)
	}
	for name, nested := range block.BlockTypes {
		value[name] = []any{populatedLegacyBlock(t, nested.Block)}
	}
	return value
}

func sampleLegacyValue(t *testing.T, typ any) any {
	t.Helper()

	switch v := typ.(type) {
	case string:
		switch v {
		case "string":
			return "sample"
		case "bool":
			return true
		case "number":
			return 1
		}
	case []any:
		if len(v) == 2 {
			switch v[0] {
			case "list", "set":
				return []any{sampleLegacyValue(t, v[1])}
			case "map":
				return map[string]any{"k": sampleLegacyValue(t, v[1])}
			}
		}
	}
	t.Fatalf("unsupported legacy type %#v", typ)
	return nil
}

func ngwafLegacyState(t *testing.T, tc ngwafMoveCase, populated bool) map[string]any {
	t.Helper()

	fixture := readNGWAFLegacyFixture(t, tc.source)
	state := legacyBlockValue(fixture.Block)
	if populated {
		state = populatedLegacyBlock(t, fixture.Block)
	}
	state["id"] = ngwafMoveID
	if tc.source.Workspace {
		state["workspace_id"] = ngwafMoveWorkspaceID
	}
	if tc.kind != "" {
		state["type"] = tc.kind
	}
	for _, name := range tc.source.Extra {
		state[name] = ngwafMoveID
	}
	return state
}

func invokeNGWAFMove(t *testing.T, sourceType, target string, version int64, state map[string]any) (*tfprotov6.MoveResourceStateResponse, tfprotov6.ProviderServer) {
	t.Helper()

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	server, err := providerserver.NewProtocol6WithError(New())()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.MoveResourceState(context.Background(), &tfprotov6.MoveResourceStateRequest{
		SourceProviderAddress: "registry.terraform.io/fastly/fastly",
		SourceTypeName:        sourceType,
		SourceSchemaVersion:   version,
		SourceState:           &tfprotov6.RawState{JSON: raw},
		TargetTypeName:        target,
	})
	if err != nil {
		t.Fatalf("MoveResourceState RPC: %v", err)
	}
	return resp, server
}

func requireNGWAFMoveError(t *testing.T, resp *tfprotov6.MoveResourceStateResponse, contains string) {
	t.Helper()

	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError && strings.Contains(d.Summary+" "+d.Detail, contains) {
			if resp.TargetState != nil {
				t.Fatal("rejected move returned target state")
			}
			return
		}
	}
	t.Fatalf("expected an error containing %q, got %v", contains, resp.Diagnostics)
}

func objectAttributes(t *testing.T, value tftypes.Value) map[string]tftypes.Value {
	t.Helper()

	var attrs map[string]tftypes.Value
	if err := value.As(&attrs); err != nil {
		t.Fatal(err)
	}
	return attrs
}

func requireString(t *testing.T, attrs map[string]tftypes.Value, name, want string) {
	t.Helper()

	var got string
	if err := attrs[name].As(&got); err != nil || got != want {
		t.Fatalf("%s = %q, want %q (error = %v)", name, got, want, err)
	}
}

func TestLegacyNGWAFMoveStateAllTargets(t *testing.T) {
	t.Parallel()

	cases := ngwafMoveCases()
	if len(cases) != 28 {
		t.Fatalf("got %d move cases, want 28", len(cases))
	}
	for _, tc := range cases {
		for _, populated := range []bool{false, true} {
			name := tc.target
			if populated {
				name += "/populated"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				resp, server := invokeNGWAFMove(t, tc.source.TypeName, tc.target, 0, ngwafLegacyState(t, tc, populated))
				for _, d := range resp.Diagnostics {
					if d.Severity == tfprotov6.DiagnosticSeverityError {
						t.Fatalf("move failed: %s: %s", d.Summary, d.Detail)
					}
				}

				ctx := context.Background()
				schemas, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
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
				attrs := objectAttributes(t, moved)

				copied := map[string]string{"id": ngwafMoveID}
				if tc.source.Workspace {
					copied["workspace_id"] = ngwafMoveWorkspaceID
				}
				for _, extra := range tc.source.Extra {
					copied[extra] = ngwafMoveID
				}
				for name, want := range copied {
					requireString(t, attrs, name, want)
				}
				// Everything else, secrets included, is rebuilt by Read.
				for name, value := range attrs {
					if _, ok := copied[name]; ok || value.IsNull() {
						continue
					}
					var elements []tftypes.Value
					if err := value.As(&elements); err != nil || len(elements) != 0 {
						t.Errorf("%s was copied into the target state; only Read should populate it", name)
					}
				}

				identities, err := server.GetResourceIdentitySchemas(ctx, &tfprotov6.GetResourceIdentitySchemasRequest{})
				if err != nil {
					t.Fatal(err)
				}
				identitySchema := identities.IdentitySchemas[tc.target]
				if identitySchema == nil || resp.TargetIdentity == nil || resp.TargetIdentity.IdentityData == nil {
					t.Fatal("missing target identity")
				}
				identity, err := resp.TargetIdentity.IdentityData.Unmarshal(identitySchema.ValueType())
				if err != nil {
					t.Fatal(err)
				}
				identityAttrs := objectAttributes(t, identity)
				requireString(t, identityAttrs, "id", ngwafMoveID)
				if tc.source.Workspace {
					requireString(t, identityAttrs, "workspace_id", ngwafMoveWorkspaceID)
				}
			})
		}
	}
}

func TestLegacyNGWAFMoveStateRejectsMismatchedType(t *testing.T) {
	t.Parallel()

	for _, tc := range ngwafMoveCases() {
		if tc.kind == "" {
			continue
		}
		for _, other := range sortedStrings(tc.source.Targets) {
			if other == tc.kind {
				continue
			}
			t.Run(tc.target+"/"+other, func(t *testing.T) {
				t.Parallel()

				state := ngwafLegacyState(t, tc, false)
				state["type"] = other
				resp, _ := invokeNGWAFMove(t, tc.source.TypeName, tc.target, 0, state)
				requireNGWAFMoveError(t, resp, "must move to "+tc.source.Targets[other])
			})
		}
	}
}

func TestLegacyNGWAFMoveStateRejectsUnsafeState(t *testing.T) {
	t.Parallel()

	rateLimit := ngwafMoveCase{source: ngwafmovestate.WorkspaceRule, kind: "rate_limit", target: "fastly_ngwaf_workspace_rate_limit_rule"}
	redaction := ngwafMoveCase{source: ngwafmovestate.Redaction, target: "fastly_ngwaf_workspace_redaction"}
	patch := ngwafMoveCase{source: ngwafmovestate.VirtualPatches, target: "fastly_ngwaf_workspace_virtual_patch"}
	accountRequest := ngwafMoveCase{source: ngwafmovestate.AccountRule, kind: "request", target: "fastly_ngwaf_request_rule"}

	tests := []struct {
		name     string
		tc       ngwafMoveCase
		version  int64
		mutate   func(map[string]any)
		contains string
	}{
		{"unsupported schema version", redaction, 1, func(map[string]any) {}, "schema version 1"},
		{"undecodable state", redaction, 0, func(s map[string]any) { s["field"] = []any{"password"} }, "state cannot be decoded as schema version 0"},
		{"unknown type", rateLimit, 0, func(s map[string]any) { s["type"] = "bogus" }, `type "bogus", which has no dedicated resource type`},
		{"account scope has no rate limit rule", accountRequest, 0, func(s map[string]any) { s["type"] = "rate_limit" }, `type "rate_limit", which has no dedicated resource type`},
		{"missing type", rateLimit, 0, func(s map[string]any) { s["type"] = nil }, "has no type"},
		{"missing id", redaction, 0, func(s map[string]any) { s["id"] = "" }, "has no id"},
		{"missing workspace", rateLimit, 0, func(s map[string]any) { s["workspace_id"] = nil }, "has no workspace_id"},
		{"missing virtual patch id", patch, 0, func(s map[string]any) { s["virtual_patch_id"] = "" }, "has no virtual_patch_id"},
		{"virtual patch id differs from id", patch, 0, func(s map[string]any) { s["virtual_patch_id"] = "other" }, `but virtual_patch_id "other"; they must match`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			state := ngwafLegacyState(t, test.tc, false)
			test.mutate(state)
			resp, _ := invokeNGWAFMove(t, test.tc.source.TypeName, test.tc.target, test.version, state)
			requireNGWAFMoveError(t, resp, test.contains)
		})
	}
}

// A legacy type must not move to a target from a different family, even when
// its state happens to decode: no mover accepts it, so Terraform reports it.
func TestLegacyNGWAFMoveStateIgnoresOtherSources(t *testing.T) {
	t.Parallel()

	tc := ngwafMoveCase{source: ngwafmovestate.AccountList, kind: "ip", target: "fastly_ngwaf_ip_list"}
	resp, _ := invokeNGWAFMove(t, ngwafmovestate.AccountSignal.TypeName, tc.target, 0, ngwafLegacyState(t, tc, false))
	if resp.TargetState != nil && !legacyMoveError(resp) {
		t.Fatal("a mover accepted state from an unrelated legacy type")
	}
	if !legacyMoveError(resp) {
		t.Fatal("expected Terraform to report the unsupported move")
	}
}

func sortedStrings(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// This opt-in test compares every NGWAF fixture with the schemas exported from
// the published registry binaries, then moves state in each published shape.
func TestLegacyNGWAFMoveStatePublishedSchemas(t *testing.T) {
	t.Parallel()

	dir := os.Getenv("FASTLY_LEGACY_SCHEMA_DIR")
	if dir == "" {
		t.Skip("run scripts/test-migration-state-schemas/run.sh to capture published schemas")
	}
	for _, release := range []string{"9.1.0", "9.5.0", "9.8.0", "10.2.0", "10.3.0"} {
		t.Run(release, func(t *testing.T) {
			t.Parallel()
			for _, source := range ngwafmovestate.Sources() {
				actual := readPublishedLegacySchema(t, dir, release, source.TypeName)
				if actual.Version != 0 {
					t.Fatalf("%s published schema version = %d, want 0", source.TypeName, actual.Version)
				}
				comparePublishedLegacyBlock(t, source.TypeName, actual.Block, readNGWAFLegacyFixture(t, source).Block)
			}
		})
	}
}
