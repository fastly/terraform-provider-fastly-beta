package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These tests drive the real protocol RPCs with SDKv2-shaped JSON state, so the
// legacy-only domain_id attribute and the implicit id are exercised exactly as
// Terraform Core persists them.
const (
	legacyDomainID  = "5h4cbhqi4Cc8mHrRtNLTKP"
	legacyServiceID = "SU1Z0isxPaozGVKXdv0eY"
)

func domainTestServer(t *testing.T) tfprotov6.ProviderServer {
	t.Helper()
	server, err := providerserver.NewProtocol6WithError(New())()
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func rpcHasError(diags []*tfprotov6.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func targetAttrs(t *testing.T, server tfprotov6.ProviderServer, typeName string, state *tfprotov6.DynamicValue) map[string]tftypes.Value {
	t.Helper()
	schemas, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	schema := schemas.ResourceSchemas[typeName]
	if schema == nil || state == nil {
		t.Fatalf("missing schema or state for %s", typeName)
	}
	value, err := state.Unmarshal(schema.ValueType())
	if err != nil {
		t.Fatal(err)
	}
	var attrs map[string]tftypes.Value
	if err := value.As(&attrs); err != nil {
		t.Fatal(err)
	}
	return attrs
}

func stringAttr(t *testing.T, attrs map[string]tftypes.Value, name string) (string, bool) {
	t.Helper()
	value, ok := attrs[name]
	if !ok {
		t.Fatalf("missing attribute %s", name)
	}
	if value.IsNull() {
		return "", false
	}
	var s string
	if err := value.As(&s); err != nil {
		t.Fatal(err)
	}
	return s, true
}

func TestLegacyDomainSameTypeStateUpgrade(t *testing.T) {
	// fastly_domain keeps its name and schema version 0. The persisted SDKv2
	// state includes domain_id, which the target schema no longer defines.
	cases := map[string]string{
		"linked":   `{"id":"` + legacyDomainID + `","domain_id":"` + legacyDomainID + `","fqdn":"www.example.com","description":"demo","service_id":"` + legacyServiceID + `"}`,
		"unlinked": `{"id":"` + legacyDomainID + `","domain_id":"` + legacyDomainID + `","fqdn":"www.example.com","description":null,"service_id":null}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			server := domainTestServer(t)
			resp, err := server.UpgradeResourceState(context.Background(), &tfprotov6.UpgradeResourceStateRequest{
				TypeName: "fastly_domain", Version: 0, RawState: &tfprotov6.RawState{JSON: []byte(raw)},
			})
			if err != nil || rpcHasError(resp.Diagnostics) {
				t.Fatalf("UpgradeResourceState: %v %v", err, resp.Diagnostics)
			}
			attrs := targetAttrs(t, server, "fastly_domain", resp.UpgradedState)
			if id, _ := stringAttr(t, attrs, "id"); id != legacyDomainID {
				t.Fatalf("id = %q", id)
			}
			if fqdn, _ := stringAttr(t, attrs, "fqdn"); fqdn != "www.example.com" {
				t.Fatalf("fqdn = %q", fqdn)
			}
			_, linked := stringAttr(t, attrs, "service_id")
			if linked != (name == "linked") {
				t.Fatalf("service_id present = %t", linked)
			}
			if _, present := attrs["domain_id"]; present {
				t.Fatal("domain_id must not exist in the target schema")
			}
		})
	}
}

func TestLegacyDomainServiceLinkSameTypeStateUpgrade(t *testing.T) {
	server := domainTestServer(t)
	raw := `{"id":"` + legacyDomainID + `","domain_id":"` + legacyDomainID + `","service_id":"` + legacyServiceID + `"}`
	resp, err := server.UpgradeResourceState(context.Background(), &tfprotov6.UpgradeResourceStateRequest{
		TypeName: "fastly_domain_service_link", Version: 0, RawState: &tfprotov6.RawState{JSON: []byte(raw)},
	})
	if err != nil || rpcHasError(resp.Diagnostics) {
		t.Fatalf("UpgradeResourceState: %v %v", err, resp.Diagnostics)
	}
	attrs := targetAttrs(t, server, "fastly_domain_service_link", resp.UpgradedState)
	for name, want := range map[string]string{"id": legacyDomainID, "domain_id": legacyDomainID, "service_id": legacyServiceID} {
		if got, _ := stringAttr(t, attrs, name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func moveDomainRPC(server tfprotov6.ProviderServer, source, target string, version int64, raw string) (*tfprotov6.MoveResourceStateResponse, error) {
	return server.MoveResourceState(context.Background(), &tfprotov6.MoveResourceStateRequest{
		SourceProviderAddress: "registry.terraform.io/fastly/fastly",
		SourceTypeName:        source,
		SourceSchemaVersion:   version,
		SourceState:           &tfprotov6.RawState{JSON: []byte(raw)},
		TargetTypeName:        target,
	})
}

func TestLegacyDomainV1MoveState(t *testing.T) {
	server := domainTestServer(t)
	raw := `{"id":"` + legacyDomainID + `","domain_id":"` + legacyDomainID + `","fqdn":"www.example.com","description":"demo","service_id":"` + legacyServiceID + `"}`
	resp, err := moveDomainRPC(server, "fastly_domain_v1", "fastly_domain", 0, raw)
	if err != nil || rpcHasError(resp.Diagnostics) {
		t.Fatalf("MoveResourceState: %v %v", err, resp.Diagnostics)
	}
	attrs := targetAttrs(t, server, "fastly_domain", resp.TargetState)
	want := map[string]string{"id": legacyDomainID, "fqdn": "www.example.com", "description": "demo", "service_id": legacyServiceID}
	for name, value := range want {
		if got, _ := stringAttr(t, attrs, name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestLegacyDomainV1MoveStatePreservesUnconfiguredValues(t *testing.T) {
	server := domainTestServer(t)
	raw := `{"id":"` + legacyDomainID + `","domain_id":"` + legacyDomainID + `","fqdn":"www.example.com","description":null,"service_id":null}`
	resp, err := moveDomainRPC(server, "fastly_domain_v1", "fastly_domain", 0, raw)
	if err != nil || rpcHasError(resp.Diagnostics) {
		t.Fatalf("MoveResourceState: %v %v", err, resp.Diagnostics)
	}
	attrs := targetAttrs(t, server, "fastly_domain", resp.TargetState)
	for _, name := range []string{"description", "service_id"} {
		if _, present := stringAttr(t, attrs, name); present {
			t.Fatalf("%s should stay null", name)
		}
	}
}

func TestLegacyDomainV1MoveStateRejectsUnsupportedState(t *testing.T) {
	validBody := `"fqdn":"www.example.com","description":null,"service_id":null`
	cases := map[string]struct {
		version int64
		raw     string
	}{
		"future schema version": {1, `{"id":"` + legacyDomainID + `","domain_id":"` + legacyDomainID + `",` + validBody + `}`},
		"missing id":            {0, `{"id":null,"domain_id":null,` + validBody + `}`},
		"empty id":              {0, `{"id":"","domain_id":"",` + validBody + `}`},
		"missing fqdn":          {0, `{"id":"` + legacyDomainID + `","domain_id":"` + legacyDomainID + `","fqdn":null,"description":null,"service_id":null}`},
		"conflicting ids":       {0, `{"id":"` + legacyDomainID + `","domain_id":"OTHER",` + validBody + `}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := moveDomainRPC(domainTestServer(t), "fastly_domain_v1", "fastly_domain", tc.version, tc.raw)
			if err == nil && !rpcHasError(resp.Diagnostics) {
				t.Fatal("expected the move to be rejected")
			}
		})
	}
}

func TestLegacyDomainV1MoveStateIgnoresOtherSources(t *testing.T) {
	// A mover that does not recognize the source must not claim the state.
	resp, err := moveDomainRPC(domainTestServer(t), "fastly_domain_v2", "fastly_domain", 0, `{"id":"x"}`)
	if err == nil && !rpcHasError(resp.Diagnostics) {
		t.Fatal("expected an unsupported-source error")
	}
}

func TestLegacyDomainV1ServiceLinkMoveState(t *testing.T) {
	server := domainTestServer(t)
	raw := `{"id":"` + legacyDomainID + `","domain_id":"` + legacyDomainID + `","service_id":"` + legacyServiceID + `"}`
	resp, err := moveDomainRPC(server, "fastly_domain_v1_service_link", "fastly_domain_service_link", 0, raw)
	if err != nil || rpcHasError(resp.Diagnostics) {
		t.Fatalf("MoveResourceState: %v %v", err, resp.Diagnostics)
	}
	attrs := targetAttrs(t, server, "fastly_domain_service_link", resp.TargetState)
	for name, want := range map[string]string{"id": legacyDomainID, "domain_id": legacyDomainID, "service_id": legacyServiceID} {
		if got, _ := stringAttr(t, attrs, name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestLegacyDomainV1ServiceLinkMoveStateRejectsUnsupportedState(t *testing.T) {
	cases := map[string]struct {
		version int64
		raw     string
	}{
		"future schema version": {1, `{"id":"A","domain_id":"A","service_id":"S"}`},
		"missing domain":        {0, `{"id":"A","domain_id":null,"service_id":"S"}`},
		"missing service":       {0, `{"id":"A","domain_id":"A","service_id":null}`},
		"conflicting ids":       {0, `{"id":"B","domain_id":"A","service_id":"S"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := moveDomainRPC(domainTestServer(t), "fastly_domain_v1_service_link", "fastly_domain_service_link", tc.version, tc.raw)
			if err == nil && !rpcHasError(resp.Diagnostics) {
				t.Fatal("expected the move to be rejected")
			}
		})
	}
}
