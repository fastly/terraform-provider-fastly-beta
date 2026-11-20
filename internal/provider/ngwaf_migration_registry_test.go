package provider

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafmovestate"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// The migration CLI routes legacy NGWAF HCL to a target through the
// compatibility registry, and the provider moves state through ngwafmovestate.
// If the two disagree, a migrated configuration would hand a target state its
// mover rejects, so these tests pin them to each other and to the schemas.

func ngwafProviderSchemas(t *testing.T) map[string]*tfprotov6.Schema {
	t.Helper()

	server, err := providerserver.NewProtocol6WithError(New())()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return resp.ResourceSchemas
}

func ngwafSchemaHasPath(schema *tfprotov6.Schema, path string) bool {
	head, rest, nested := strings.Cut(path, ".")
	for _, attr := range schema.Block.Attributes {
		if attr.Name != head {
			continue
		}
		if !nested {
			return true
		}
		if attr.NestedType == nil {
			return false
		}
		for _, child := range attr.NestedType.Attributes {
			if child.Name == rest {
				return true
			}
		}
	}
	return false
}

func ngwafSourceFor(t *testing.T, legacy string) ngwafmovestate.Source {
	t.Helper()

	for _, source := range ngwafmovestate.Sources() {
		if source.TypeName == legacy {
			return source
		}
	}
	t.Fatalf("no state mover source for %s", legacy)
	return ngwafmovestate.Source{}
}

func TestNGWAFMigrationRoutesMatchStateMovers(t *testing.T) {
	t.Parallel()

	schemas := ngwafProviderSchemas(t)
	routed := make(map[string]string) // target -> legacy, from the registry
	for _, rule := range compatibility.Default().All() {
		if rule.Kind != compatibility.ConstructResource || !strings.HasPrefix(rule.Legacy, "fastly_ngwaf_") ||
			rule.State != compatibility.StateMove {
			continue
		}
		if rule.Support != compatibility.SupportAutomatic {
			t.Errorf("%s: NGWAF move rule is %s, want automatic", rule.ID, rule.Support)
		}
		if schemas[rule.Target] == nil {
			t.Errorf("%s: target %s is not a registered resource", rule.ID, rule.Target)
		}
		source := ngwafSourceFor(t, rule.Legacy)
		want := source.Target
		if rule.Selector != "" {
			raw, ok := strings.CutPrefix(rule.Selector, "type = ")
			kind, err := strconv.Unquote(raw)
			if !ok || err != nil {
				t.Fatalf("%s: unexpected selector %q", rule.ID, rule.Selector)
			}
			want = source.Targets[kind]
		}
		if rule.Target != want {
			t.Errorf("%s: registry routes to %s, but the state mover expects %s", rule.ID, rule.Target, want)
		}
		routed[rule.Target] = rule.Legacy
	}

	for _, source := range ngwafmovestate.Sources() {
		for _, target := range source.TargetTypes() {
			if routed[target] != source.TypeName {
				t.Errorf("state mover %s -> %s has no automatic registry route", source.TypeName, target)
			}
		}
	}
	if len(routed) != 28 {
		t.Fatalf("got %d NGWAF registry routes, want 28", len(routed))
	}
}

func TestNGWAFMigrationOutputsExistInBothSchemas(t *testing.T) {
	t.Parallel()

	schemas := ngwafProviderSchemas(t)
	legacyFor := make(map[string]ngwafmovestate.Source)
	for _, source := range ngwafmovestate.Sources() {
		for _, target := range source.TargetTypes() {
			legacyFor[target] = source
		}
	}

	outputs := compatibility.NGWAFPassThroughOutputs()
	if len(outputs) != 28 {
		t.Fatalf("pass-through outputs cover %d targets, want 28", len(outputs))
	}
	for target, names := range outputs {
		source, ok := legacyFor[target]
		if !ok || schemas[target] == nil {
			t.Errorf("pass-through outputs for unknown target %s", target)
			continue
		}
		legacy := source.LegacySchema()
		for _, name := range names {
			if !ngwafSchemaHasPath(schemas[target], name) {
				t.Errorf("%s.%s is listed as a pass-through output but the target has no such attribute", target, name)
			}
			if _, ok := legacy.Attributes[name]; !ok {
				t.Errorf("%s.%s is listed as a pass-through output but %s has no such attribute", target, name, source.TypeName)
			}
		}
	}

	for _, rule := range compatibility.Default().All() {
		if rule.Kind != compatibility.ConstructAttribute || !strings.HasPrefix(rule.Legacy, "fastly_ngwaf_") {
			continue
		}
		legacyType, legacyName, _ := strings.Cut(rule.Legacy, ".")
		targetType, targetPath, _ := strings.Cut(rule.Target, ".")
		if _, ok := ngwafSourceFor(t, legacyType).LegacySchema().Attributes[legacyName]; !ok {
			t.Errorf("%s: %s has no attribute %s", rule.ID, legacyType, legacyName)
		}
		if schemas[targetType] == nil || !ngwafSchemaHasPath(schemas[targetType], targetPath) {
			t.Errorf("%s: target path %s does not exist", rule.ID, rule.Target)
		}
	}
}
