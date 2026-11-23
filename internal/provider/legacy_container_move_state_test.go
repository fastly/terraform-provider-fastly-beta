package provider

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These tests drive the MoveResourceState RPC with SDKv2-shaped JSON state for
// the renamed versionless container resources.
const (
	legacyContainerID = "7QYy4nSAvbCyc8OyCYHz5S"
	legacyACLID       = "4Gp9l2sRnzbfn6Ym0ZgK3v"
)

func moveContainer(t *testing.T, source, target string, version int64, raw string) (map[string]tftypes.Value, *tfprotov6.MoveResourceStateResponse, tfprotov6.ProviderServer) {
	t.Helper()
	server := domainTestServer(t)
	resp, err := moveDomainRPC(server, source, target, version, raw)
	if err != nil || rpcHasError(resp.Diagnostics) {
		t.Fatalf("MoveResourceState: %v %v", err, resp.Diagnostics)
	}
	return targetAttrs(t, server, target, resp.TargetState), resp, server
}

func requireMoveRejected(t *testing.T, source, target string, version int64, raw, contains string) {
	t.Helper()
	resp, err := moveDomainRPC(domainTestServer(t), source, target, version, raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError && d.Summary == "Cannot migrate "+source && strings.Contains(d.Detail, contains) {
			return
		}
	}
	t.Fatalf("expected an error containing %q, got %v", contains, resp.Diagnostics)
}

func stringMapAttr(t *testing.T, attrs map[string]tftypes.Value, name string) map[string]string {
	t.Helper()
	value := attrs[name]
	if value.IsNull() {
		t.Fatalf("%s is null; an empty owned map must not be null", name)
	}
	var elements map[string]tftypes.Value
	if err := value.As(&elements); err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(elements))
	for key, element := range elements {
		var s string
		if err := element.As(&s); err != nil {
			t.Fatal(err)
		}
		result[key] = s
	}
	return result
}

func TestLegacyComputeACLMoveState(t *testing.T) {
	attrs, _, _ := moveContainer(t, "fastly_compute_acl", "fastly_acl", 0, `{"id":"`+legacyACLID+`","name":"blocklist"}`)
	for name, want := range map[string]string{"id": legacyACLID, "name": "blocklist"} {
		if got, _ := stringAttr(t, attrs, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestLegacyComputeACLMoveStateRejectsUnsupportedState(t *testing.T) {
	for name, tc := range map[string]struct {
		version       int64
		raw, contains string
	}{
		"future schema version": {1, `{"id":"` + legacyACLID + `","name":"blocklist"}`, "release that migration does not support"},
		"missing id":            {0, `{"id":null,"name":"blocklist"}`, "has no `id`. Remove the `moved` block and import `fastly_acl` with the ID `<acl_id>`."},
		"missing name":          {0, `{"id":"` + legacyACLID + `","name":""}`, "has no `name`."},
	} {
		t.Run(name, func(t *testing.T) {
			requireMoveRejected(t, "fastly_compute_acl", "fastly_acl", tc.version, tc.raw, tc.contains)
		})
	}
}

type entriesMoveCase struct {
	source, target   string
	containerAttr    string // legacy container ID attribute
	targetContainer  string // target container ID attribute
	targetItems      string // target map attribute
	legacyIDSuffix   string
	wantTargetSuffix string
}

func entriesMoveCases() []entriesMoveCase {
	return []entriesMoveCase{
		{"fastly_compute_acl_entries", "fastly_acl_entries", "compute_acl_id", "acl_id", "entries", "/entries", "/entries"},
		{"fastly_configstore_entries", "fastly_configstore_items", "store_id", "store_id", "items", "/entries", "/items"},
	}
}

func (c entriesMoveCase) raw(manageEntries, entries string) string {
	return `{"id":"` + legacyContainerID + c.legacyIDSuffix + `","` + c.containerAttr + `":"` + legacyContainerID +
		`","entries":` + entries + `,"manage_entries":` + manageEntries + `}`
}

func TestLegacyContainerEntriesMoveState(t *testing.T) {
	populated := `{"10.0.0.0/8":"BLOCK","192.0.2.0/24":"ALLOW"}`
	for _, c := range entriesMoveCases() {
		t.Run(c.source, func(t *testing.T) {
			for name, tc := range map[string]struct {
				manageEntries, entries string
				want                   map[string]string
			}{
				// Legacy kept every remote entry in state and enforced an exact match.
				"managed":                 {"true", populated, map[string]string{"10.0.0.0/8": "BLOCK", "192.0.2.0/24": "ALLOW"}},
				"managed without entries": {"true", "null", map[string]string{}},
				// Legacy only seeded entries; a stale map must not become owned.
				"unmanaged with stale entries": {"false", populated, map[string]string{}},
				"unmanaged as refreshed":       {"false", "{}", map[string]string{}},
				"manage_entries null":          {"null", populated, map[string]string{}},
			} {
				t.Run(name, func(t *testing.T) {
					attrs, _, _ := moveContainer(t, c.source, c.target, 0, c.raw(tc.manageEntries, tc.entries))
					if got, _ := stringAttr(t, attrs, "id"); got != legacyContainerID+c.wantTargetSuffix {
						t.Errorf("id = %q, want %q", got, legacyContainerID+c.wantTargetSuffix)
					}
					if got, _ := stringAttr(t, attrs, c.targetContainer); got != legacyContainerID {
						t.Errorf("%s = %q, want %q", c.targetContainer, got, legacyContainerID)
					}
					got := stringMapAttr(t, attrs, c.targetItems)
					if len(got) != len(tc.want) {
						t.Fatalf("%s = %v, want %v", c.targetItems, got, tc.want)
					}
					for key, value := range tc.want {
						if got[key] != value {
							t.Errorf("%s[%q] = %q, want %q", c.targetItems, key, got[key], value)
						}
					}
					for _, legacyOnly := range []string{"manage_entries", "compute_acl_id"} {
						if _, present := attrs[legacyOnly]; present {
							t.Errorf("%s must not exist in the target schema", legacyOnly)
						}
					}
				})
			}
		})
	}
}

func TestLegacyContainerEntriesMoveStateRejectsUnsupportedState(t *testing.T) {
	for _, c := range entriesMoveCases() {
		t.Run(c.source, func(t *testing.T) {
			valid := c.raw("true", "{}")
			for name, tc := range map[string]struct {
				version       int64
				raw, contains string
			}{
				"future schema version": {1, valid, "release that migration does not support"},
				"missing container id": {0, strings.Replace(valid, `"`+c.containerAttr+`":"`+legacyContainerID+`"`, `"`+c.containerAttr+`":null`, 1),
					"has no `" + c.containerAttr + "`."},
				"mismatched id": {0, strings.Replace(valid, `"id":"`+legacyContainerID, `"id":"OTHER`, 1),
					"import `" + c.target + "` with the ID `" + legacyContainerID + c.wantTargetSuffix + "`."},
				"missing id": {0, strings.Replace(valid, `"id":"`+legacyContainerID+c.legacyIDSuffix+`"`, `"id":null`, 1), "does not match"},
			} {
				t.Run(name, func(t *testing.T) {
					requireMoveRejected(t, c.source, c.target, tc.version, tc.raw, tc.contains)
				})
			}
		})
	}
}

func legacyServiceACLEntriesRaw(manageEntries, entries string) string {
	return `{"id":"` + legacyServiceID + `/` + legacyACLID + `","service_id":"` + legacyServiceID + `","acl_id":"` + legacyACLID +
		`","manage_entries":` + manageEntries + `,"entry":` + entries + `}`
}

type movedACLEntry struct {
	id, ip, comment string
	subnet          *int64
	negated         *bool
}

func movedACLEntries(t *testing.T, attrs map[string]tftypes.Value) map[string]movedACLEntry {
	t.Helper()
	value := attrs["entry"]
	if value.IsNull() {
		t.Fatal("entry is null; an empty owned set must not be null")
	}
	var elements []tftypes.Value
	if err := value.As(&elements); err != nil {
		t.Fatal(err)
	}
	result := make(map[string]movedACLEntry, len(elements))
	for _, element := range elements {
		var fields map[string]tftypes.Value
		if err := element.As(&fields); err != nil {
			t.Fatal(err)
		}
		var entry movedACLEntry
		entry.id, _ = stringAttr(t, fields, "id")
		entry.ip, _ = stringAttr(t, fields, "ip")
		entry.comment, _ = stringAttr(t, fields, "comment")
		if !fields["subnet"].IsNull() {
			var number big.Float
			if err := fields["subnet"].As(&number); err != nil {
				t.Fatal(err)
			}
			subnet, _ := number.Int64()
			entry.subnet = &subnet
		}
		if !fields["negated"].IsNull() {
			var b bool
			if err := fields["negated"].As(&b); err != nil {
				t.Fatal(err)
			}
			entry.negated = &b
		}
		result[entry.ip] = entry
	}
	return result
}

func TestLegacyServiceACLEntriesMoveState(t *testing.T) {
	entries := `[` +
		`{"id":"E1","ip":"192.0.2.0","subnet":"24","negated":false,"comment":"office"},` +
		`{"id":"E2","ip":"198.51.100.7","subnet":"","negated":true,"comment":""}]`

	t.Run("managed", func(t *testing.T) {
		attrs, resp, server := moveContainer(t, "fastly_service_acl_entries", "fastly_service_cdn_acl_entries", 0,
			legacyServiceACLEntriesRaw("true", entries))
		for name, want := range map[string]string{
			"id": legacyServiceID + "/" + legacyACLID, "service_id": legacyServiceID, "acl_id": legacyACLID,
		} {
			if got, _ := stringAttr(t, attrs, name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		got := movedACLEntries(t, attrs)
		if len(got) != 2 {
			t.Fatalf("entries = %v, want 2", got)
		}
		office := got["192.0.2.0"]
		if office.id != "E1" || office.subnet == nil || *office.subnet != 24 || office.comment != "office" ||
			office.negated == nil || *office.negated {
			t.Errorf("192.0.2.0 entry = %+v", office)
		}
		host := got["198.51.100.7"]
		// SDKv2 persisted an unset subnet and comment as "".
		if host.id != "E2" || host.subnet != nil || host.comment != "" || host.negated == nil || !*host.negated {
			t.Errorf("198.51.100.7 entry = %+v", host)
		}

		identities, err := server.GetResourceIdentitySchemas(context.Background(), &tfprotov6.GetResourceIdentitySchemasRequest{})
		if err != nil {
			t.Fatal(err)
		}
		identitySchema := identities.IdentitySchemas["fastly_service_cdn_acl_entries"]
		if identitySchema == nil || resp.TargetIdentity == nil || resp.TargetIdentity.IdentityData == nil {
			t.Fatal("missing target identity")
		}
		identity, err := resp.TargetIdentity.IdentityData.Unmarshal(identitySchema.ValueType())
		if err != nil {
			t.Fatal(err)
		}
		var identityAttrs map[string]tftypes.Value
		if err := identity.As(&identityAttrs); err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]string{"service_id": legacyServiceID, "acl_id": legacyACLID} {
			if got, _ := stringAttr(t, identityAttrs, name); got != want {
				t.Errorf("identity %s = %q, want %q", name, got, want)
			}
		}
	})

	for name, manageEntries := range map[string]string{"unmanaged": "false", "manage_entries null": "null"} {
		t.Run(name, func(t *testing.T) {
			attrs, _, _ := moveContainer(t, "fastly_service_acl_entries", "fastly_service_cdn_acl_entries", 0,
				legacyServiceACLEntriesRaw(manageEntries, entries))
			if got := movedACLEntries(t, attrs); len(got) != 0 {
				t.Fatalf("entries = %v, want none owned", got)
			}
		})
	}

	t.Run("managed without entries", func(t *testing.T) {
		attrs, _, _ := moveContainer(t, "fastly_service_acl_entries", "fastly_service_cdn_acl_entries", 0,
			legacyServiceACLEntriesRaw("true", "[]"))
		if got := movedACLEntries(t, attrs); len(got) != 0 {
			t.Fatalf("entries = %v, want none", got)
		}
	})
}

func TestLegacyServiceACLEntriesMoveStateRejectsUnsupportedState(t *testing.T) {
	entry := func(ip, subnet string) string {
		return `[{"id":"E1","ip":` + ip + `,"subnet":"` + subnet + `","negated":false,"comment":""}]`
	}
	valid := legacyServiceACLEntriesRaw("true", entry(`"192.0.2.0"`, "24"))
	for name, tc := range map[string]struct {
		version       int64
		raw, contains string
	}{
		"future schema version": {1, valid, "release that migration does not support"},
		"missing service_id":    {0, strings.Replace(valid, `"service_id":"`+legacyServiceID+`"`, `"service_id":""`, 1), "has no `service_id`."},
		"missing acl_id":        {0, strings.Replace(valid, `"acl_id":"`+legacyACLID+`"`, `"acl_id":null`, 1), "has no `acl_id`."},
		"mismatched id": {0, strings.Replace(valid, `"id":"`+legacyServiceID, `"id":"OTHER`, 1),
			"import `fastly_service_cdn_acl_entries` with the ID `" + legacyServiceID + "/" + legacyACLID + "`."},
		"entry without ip":    {0, legacyServiceACLEntriesRaw("true", entry(`""`, "24")), "has no IP address"},
		"non-numeric subnet":  {0, legacyServiceACLEntriesRaw("true", entry(`"192.0.2.0"`, "255.255.255.0")), "must be a prefix length from 0 to 128"},
		"out-of-range subnet": {0, legacyServiceACLEntriesRaw("true", entry(`"192.0.2.0"`, "129")), "must be a prefix length from 0 to 128"},
	} {
		t.Run(name, func(t *testing.T) {
			requireMoveRejected(t, "fastly_service_acl_entries", "fastly_service_cdn_acl_entries", tc.version, tc.raw, tc.contains)
		})
	}
}

func TestLegacyContainerMoveStateIgnoresOtherSources(t *testing.T) {
	for target, source := range map[string]string{
		"fastly_acl":                     "fastly_compute_acl_v2",
		"fastly_acl_entries":             "fastly_compute_acl_entries_v2",
		"fastly_configstore_items":       "fastly_configstore_entries_v2",
		"fastly_service_cdn_acl_entries": "fastly_service_acl_entries_v2",
	} {
		t.Run(target, func(t *testing.T) {
			resp, err := moveDomainRPC(domainTestServer(t), source, target, 0, `{"id":"x"}`)
			if err == nil && !rpcHasError(resp.Diagnostics) {
				t.Fatal("expected an unsupported-source error")
			}
		})
	}
}

// legacyContainerPublishedShapes are the SDKv2 shapes the container movers
// decode. They are identical in every supported release.
var legacyContainerPublishedShapes = map[string]legacyServiceBlock{
	"fastly_compute_acl": {Attributes: map[string]any{"id": "string", "name": "string"}},
	"fastly_compute_acl_entries": {Attributes: map[string]any{
		"id": "string", "compute_acl_id": "string", "entries": []any{"map", "string"}, "manage_entries": "bool",
	}},
	"fastly_configstore_entries": {Attributes: map[string]any{
		"id": "string", "store_id": "string", "entries": []any{"map", "string"}, "manage_entries": "bool",
	}},
	"fastly_service_acl_entries": {
		Attributes: map[string]any{"id": "string", "service_id": "string", "acl_id": "string", "manage_entries": "bool"},
		BlockTypes: map[string]legacyServiceNested{"entry": {NestingMode: "set", Block: legacyServiceBlock{
			Attributes: map[string]any{"id": "string", "ip": "string", "subnet": "string", "negated": "bool", "comment": "string"},
		}}},
	},
}

// This opt-in test compares the container shapes above with the schemas
// exported from the published registry binaries.
func TestLegacyContainerMoveStatePublishedSchemas(t *testing.T) {
	t.Parallel()

	dir := os.Getenv("FASTLY_LEGACY_SCHEMA_DIR")
	if dir == "" {
		t.Skip("run scripts/test-migration-state-schemas/run.sh to capture published schemas")
	}
	for _, release := range []string{"9.1.0", "9.5.0", "9.8.0", "10.2.0", "10.3.0"} {
		t.Run(release, func(t *testing.T) {
			t.Parallel()
			for typeName, want := range legacyContainerPublishedShapes {
				actual := readPublishedLegacySchema(t, dir, release, typeName)
				if actual.Version != 0 {
					t.Fatalf("%s published schema version = %d, want 0", typeName, actual.Version)
				}
				comparePublishedLegacyBlock(t, typeName, actual.Block, want)
			}
		})
	}
}
