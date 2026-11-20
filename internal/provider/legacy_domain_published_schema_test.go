package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

type publishedAttribute struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Optional bool   `json:"optional"`
	Computed bool   `json:"computed"`
}

type publishedDomainSchema struct {
	Version int64 `json:"version"`
	Block   struct {
		Attributes map[string]publishedAttribute `json:"attributes"`
		BlockTypes map[string]json.RawMessage    `json:"block_types"`
	} `json:"block"`
}

var (
	publishedDomainReleases = []string{"9.1.0", "9.5.0", "9.8.0", "10.2.0", "10.3.0"}

	// These are the exact SDKv2 shapes the domain movers and upgraders decode.
	publishedDomainAttributes = map[string]map[string]publishedAttribute{
		"fastly_domain":                 domainSDKAttributes,
		"fastly_domain_v1":              domainSDKAttributes,
		"fastly_domain_service_link":    linkSDKAttributes,
		"fastly_domain_v1_service_link": linkSDKAttributes,
	}

	domainSDKAttributes = map[string]publishedAttribute{
		"description": {Type: "string", Optional: true},
		"domain_id":   {Type: "string", Computed: true},
		"fqdn":        {Type: "string", Required: true},
		"id":          {Type: "string", Optional: true, Computed: true},
		"service_id":  {Type: "string", Optional: true},
	}
	linkSDKAttributes = map[string]publishedAttribute{
		"domain_id":  {Type: "string", Required: true},
		"id":         {Type: "string", Optional: true, Computed: true},
		"service_id": {Type: "string", Required: true},
	}
)

func readPublishedProviderSchemas(t *testing.T, dir, release string) (map[string]publishedDomainSchema, map[string]json.RawMessage) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, release+".json")) // #nosec G304 G703 -- explicit test schema directory and fixed release names
	if err != nil {
		t.Fatal(err)
	}
	var schemas struct {
		Providers map[string]struct {
			Resources   map[string]json.RawMessage `json:"resource_schemas"`
			DataSources map[string]json.RawMessage `json:"data_source_schemas"`
		} `json:"provider_schemas"`
	}
	if err := json.Unmarshal(raw, &schemas); err != nil {
		t.Fatal(err)
	}
	provider := schemas.Providers["registry.terraform.io/fastly/fastly"]
	// Other resources use composite attribute types this decoder does not model.
	domains := make(map[string]publishedDomainSchema, len(publishedDomainAttributes))
	for typeName := range publishedDomainAttributes {
		raw, found := provider.Resources[typeName]
		if !found {
			continue
		}
		var schema publishedDomainSchema
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("%s: %v", typeName, err)
		}
		domains[typeName] = schema
	}
	return domains, provider.DataSources
}

// This opt-in test compares the exact schemas exported from published SDKv2
// binaries with the shapes the domain state conversions assume, then runs the
// conversions on state built from those schemas. It requires no credentials.
func TestLegacyDomainPublishedSchemas(t *testing.T) {
	t.Parallel()

	dir := os.Getenv("FASTLY_LEGACY_SCHEMA_DIR")
	if dir == "" {
		t.Skip("run scripts/test-migration-state-schemas/run.sh to capture published schemas")
	}
	for _, release := range publishedDomainReleases {
		t.Run(release, func(t *testing.T) {
			t.Parallel()
			resources, dataSources := readPublishedProviderSchemas(t, dir, release)
			for typeName, want := range publishedDomainAttributes {
				published, found := resources[typeName]
				if !found {
					t.Errorf("%s is missing from the published provider", typeName)
					continue
				}
				if published.Version != 0 {
					t.Errorf("%s schema version = %d, want 0", typeName, published.Version)
				}
				if len(published.Block.BlockTypes) != 0 {
					t.Errorf("%s unexpectedly defines nested blocks", typeName)
				}
				if !reflect.DeepEqual(published.Block.Attributes, want) {
					t.Errorf("%s published attributes = %#v, want %#v", typeName, published.Block.Attributes, want)
				}
			}
			// The data source block in the analyzer exists because the legacy
			// provider ships it and the target does not.
			if _, found := dataSources["fastly_domains_v1"]; !found {
				t.Error("fastly_domains_v1 is missing from the published provider")
			}
		})
	}
}

// TestLegacyDomainPublishedSchemaStateConversion converts state generated from
// the published schemas, so the conversions are tested against every attribute
// SDKv2 persists, not a handwritten subset.
func TestLegacyDomainPublishedSchemaStateConversion(t *testing.T) {
	t.Parallel()

	dir := os.Getenv("FASTLY_LEGACY_SCHEMA_DIR")
	if dir == "" {
		t.Skip("run scripts/test-migration-state-schemas/run.sh to capture published schemas")
	}
	for _, release := range publishedDomainReleases {
		t.Run(release, func(t *testing.T) {
			t.Parallel()
			resources, _ := readPublishedProviderSchemas(t, dir, release)
			for _, linked := range []bool{true, false} {
				for _, description := range []string{"", "demo"} {
					state := map[string]any{}
					for name := range resources["fastly_domain"].Block.Attributes {
						state[name] = nil
					}
					state["id"], state["domain_id"] = legacyDomainID, legacyDomainID
					state["fqdn"] = "www.example.com"
					state["description"] = description
					if linked {
						state["service_id"] = legacyServiceID
					}
					raw, err := json.Marshal(state)
					if err != nil {
						t.Fatal(err)
					}
					server := domainTestServer(t)
					moved, err := moveDomainRPC(server, "fastly_domain_v1", "fastly_domain", 0, string(raw))
					if err != nil || rpcHasError(moved.Diagnostics) {
						t.Fatalf("move: %v %v", err, moved.Diagnostics)
					}
					assertConvertedDomain(t, server, moved.TargetState, description, linked)
					upgraded, err := server.UpgradeResourceState(context.Background(), &tfprotov6.UpgradeResourceStateRequest{
						TypeName: "fastly_domain", Version: 0, RawState: &tfprotov6.RawState{JSON: raw},
					})
					if err != nil || rpcHasError(upgraded.Diagnostics) {
						t.Fatalf("upgrade: %v %v", err, upgraded.Diagnostics)
					}
					assertConvertedDomain(t, server, upgraded.UpgradedState, description, linked)
				}
			}
		})
	}
}

func assertConvertedDomain(t *testing.T, server tfprotov6.ProviderServer, state *tfprotov6.DynamicValue, description string, linked bool) {
	t.Helper()
	attrs := targetAttrs(t, server, "fastly_domain", state)
	if id, _ := stringAttr(t, attrs, "id"); id != legacyDomainID {
		t.Errorf("id = %q, want %q", id, legacyDomainID)
	}
	if got, present := stringAttr(t, attrs, "description"); got != description || present != (description != "") {
		t.Errorf("description = %q (present %t), want %q", got, present, description)
	}
	serviceID, present := stringAttr(t, attrs, "service_id")
	if present != linked || (linked && serviceID != legacyServiceID) {
		t.Errorf("service_id = %q (present %t), linked %t", serviceID, present, linked)
	}
}
