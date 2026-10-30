package compatibility

import (
	"strings"
	"testing"
)

func TestDefaultRegistryCoversLegacyResources(t *testing.T) {
	t.Parallel()

	registry := Default()
	for _, legacy := range legacyResourceInventory {
		rules := registry.RulesForLegacy(legacy)
		if !containsKind(rules, ConstructResource) {
			t.Errorf("legacy resource %q has no resource compatibility rule", legacy)
		}
	}
}

func TestDefaultRegistryCoversLegacyServiceBlocks(t *testing.T) {
	t.Parallel()

	registry := Default()
	tests := []struct {
		resource string
		blocks   []string
	}{
		{resource: "fastly_service_vcl", blocks: legacyVCLBlockInventory},
		{resource: "fastly_service_compute", blocks: legacyComputeBlockInventory},
	}

	for _, test := range tests {
		for _, block := range test.blocks {
			legacy := test.resource + "." + block
			rules := registry.RulesForLegacy(legacy)
			if !containsKind(rules, ConstructServiceBlock) {
				t.Errorf("legacy service block %q has no compatibility rule", legacy)
			}
		}
	}
}

func TestDefaultRegistryCoversLegacyProductEnablement(t *testing.T) {
	t.Parallel()

	registry := Default()
	tests := []struct {
		resource string
		products []string
	}{
		{resource: "fastly_service_vcl", products: legacyVCLProductInventory},
		{resource: "fastly_service_compute", products: legacyComputeProductInventory},
	}

	for _, test := range tests {
		for _, product := range test.products {
			legacy := test.resource + ".product_enablement." + product
			rules := registry.RulesForLegacy(legacy)
			if !containsKind(rules, ConstructProduct) {
				t.Errorf("legacy product %q has no product compatibility rule", legacy)
			}
		}
	}
}

func TestDefaultRegistryResourceTargetsExist(t *testing.T) {
	t.Parallel()

	targets := stringSet(targetResourceInventory)
	for _, rule := range Default().All() {
		if rule.Target == "" || strings.Contains(rule.Target, "*") {
			continue
		}
		if rule.Kind != ConstructResource && rule.Kind != ConstructProduct {
			continue
		}
		if _, ok := targets[rule.Target]; !ok {
			t.Errorf("rule %q targets unknown Framework resource %q", rule.ID, rule.Target)
		}
	}
}

func TestDefaultRegistryAttributeTargetsReferenceKnownResources(t *testing.T) {
	t.Parallel()

	targets := stringSet(targetResourceInventory)
	for _, rule := range Default().All() {
		if rule.Kind != ConstructAttribute || rule.Target == "" || strings.Contains(rule.Target, "*") {
			continue
		}
		resourceName, _, ok := strings.Cut(rule.Target, ".")
		if !ok {
			t.Errorf("attribute rule %q has invalid target %q", rule.ID, rule.Target)
			continue
		}
		if _, ok := targets[resourceName]; !ok {
			t.Errorf("attribute rule %q targets unknown Framework resource %q", rule.ID, resourceName)
		}
	}
}

func TestDefaultRegistryServiceBlockTargetsExist(t *testing.T) {
	t.Parallel()

	targets := map[string]map[string]struct{}{
		"fastly_service_cdn_auto":     stringSet(targetCDNAutoBlockInventory),
		"fastly_service_compute_auto": stringSet(targetComputeAutoBlockInventory),
	}

	for _, rule := range Default().All() {
		if rule.Kind != ConstructServiceBlock || rule.Target == "" || rule.HCL == HCLSplit {
			continue
		}

		resourceName, blockName, ok := strings.Cut(rule.Target, ".")
		if !ok {
			t.Errorf("service block rule %q has invalid target %q", rule.ID, rule.Target)
			continue
		}
		blocks, ok := targets[resourceName]
		if !ok {
			t.Errorf("service block rule %q targets unknown auto service %q", rule.ID, resourceName)
			continue
		}
		if _, ok := blocks[blockName]; !ok {
			t.Errorf("service block rule %q targets unknown block %q on %s", rule.ID, blockName, resourceName)
		}
	}
}

func TestDefaultRegistryServiceStateSchemasAreExplicit(t *testing.T) {
	t.Parallel()

	tests := map[string][]int64{
		"resource-service-vcl":     {1},
		"resource-service-compute": {0},
	}
	for id, want := range tests {
		rule, ok := Default().RuleByID(id)
		if !ok {
			t.Fatalf("missing rule %q", id)
		}
		if len(rule.SourceSchemaVersions) != len(want) {
			t.Fatalf("%s source schema versions = %v, want %v", id, rule.SourceSchemaVersions, want)
		}
		for index := range want {
			if rule.SourceSchemaVersions[index] != want[index] {
				t.Fatalf("%s source schema versions = %v, want %v", id, rule.SourceSchemaVersions, want)
			}
		}
	}
}

func TestRegistryUnknownLegacyConstructFailsClosed(t *testing.T) {
	t.Parallel()

	if got := Default().RulesForLegacy("fastly_service_vcl.future_unknown_block"); len(got) != 0 {
		t.Fatalf("unknown legacy construct returned %d rules, want 0", len(got))
	}
}

func TestRegistryReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	registry := Default()
	rules := registry.All()
	if len(rules) == 0 {
		t.Fatal("default registry is empty")
	}

	originalID := rules[0].ID
	rules[0].ID = "mutated"

	fresh := registry.All()
	if fresh[0].ID != originalID {
		t.Fatalf("registry was mutated through All(): got %q, want %q", fresh[0].ID, originalID)
	}

	rule, ok := registry.RuleByID(originalID)
	if !ok {
		t.Fatalf("RuleByID(%q) not found after defensive-copy mutation", originalID)
	}
	rule.SourceSchemaVersions = append(rule.SourceSchemaVersions, 999)

	again, _ := registry.RuleByID(originalID)
	if len(again.SourceSchemaVersions) > 0 && again.SourceSchemaVersions[len(again.SourceSchemaVersions)-1] == 999 {
		t.Fatal("registry SourceSchemaVersions was mutated through RuleByID")
	}
}

func TestNewRegistryRejectsInvalidRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rule Rule
	}{
		{
			name: "blocked without prerequisite",
			rule: validTestRule(func(rule *Rule) {
				rule.Support = SupportBlocked
			}),
		},
		{
			name: "import without identity",
			rule: validTestRule(func(rule *Rule) {
				rule.State = StateImport
			}),
		},
		{
			name: "identity without import",
			rule: validTestRule(func(rule *Rule) {
				rule.ImportIdentity = ImportServiceID
			}),
		},
		{
			name: "automatic missing target",
			rule: validTestRule(func(rule *Rule) {
				rule.Target = ""
			}),
		},
		{
			name: "negative schema version",
			rule: validTestRule(func(rule *Rule) {
				rule.SourceSchemaVersions = []int64{-1}
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewRegistry([]Rule{test.rule}); err == nil {
				t.Fatal("NewRegistry() accepted invalid rule")
			}
		})
	}
}

func TestNewRegistryRejectsNonCanonicalRoutingFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Rule)
	}{
		{
			name: "id leading whitespace",
			mutate: func(rule *Rule) {
				rule.ID = " test"
			},
		},
		{
			name: "id trailing whitespace",
			mutate: func(rule *Rule) {
				rule.ID = "test "
			},
		},
		{
			name: "legacy leading whitespace",
			mutate: func(rule *Rule) {
				rule.Legacy = " fastly_legacy"
			},
		},
		{
			name: "legacy trailing whitespace",
			mutate: func(rule *Rule) {
				rule.Legacy = "fastly_legacy "
			},
		},
		{
			name: "selector leading whitespace",
			mutate: func(rule *Rule) {
				rule.Selector = " type=request"
			},
		},
		{
			name: "selector trailing whitespace",
			mutate: func(rule *Rule) {
				rule.Selector = "type=request "
			},
		},
		{
			name: "target leading whitespace",
			mutate: func(rule *Rule) {
				rule.Target = " fastly_target"
			},
		},
		{
			name: "target trailing whitespace",
			mutate: func(rule *Rule) {
				rule.Target = "fastly_target "
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewRegistry([]Rule{validTestRule(test.mutate)}); err == nil {
				t.Fatal("NewRegistry() accepted a rule with non-canonical routing fields")
			}
		})
	}
}

func TestNewRegistryRejectsAmbiguousRoutes(t *testing.T) {
	t.Parallel()

	first := validTestRule(nil)
	second := first
	second.ID = "test-two"

	if _, err := NewRegistry([]Rule{first, second}); err == nil {
		t.Fatal("NewRegistry() accepted duplicate compatibility route")
	}
}

func containsKind(rules []Rule, kind ConstructKind) bool {
	for _, rule := range rules {
		if rule.Kind == kind {
			return true
		}
	}
	return false
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func validTestRule(mutate func(*Rule)) Rule {
	rule := Rule{
		ID:             "test",
		Kind:           ConstructResource,
		Family:         FamilyGlobal,
		Legacy:         "fastly_legacy",
		Target:         "fastly_target",
		Support:        SupportAutomatic,
		HCL:            HCLRename,
		State:          StateMove,
		ImportIdentity: ImportNone,
	}
	if mutate != nil {
		mutate(&rule)
	}
	return rule
}
