package compatibility

import "testing"

func TestVCLTrafficTablesMatchAutomaticBlockRules(t *testing.T) {
	t.Parallel()
	registry := Default()
	automatic := func(block string) bool {
		for _, rule := range registry.RulesForLegacy("fastly_service_vcl." + block) {
			if rule.Kind == ConstructServiceBlock && rule.Support == SupportAutomatic {
				return true
			}
		}
		return false
	}
	for _, block := range VCLNameOrderedBlocks {
		if !automatic(block) {
			t.Errorf("ordered block %q has no automatic VCL block rule", block)
		}
	}
	for _, change := range VCLLiteralRewrites {
		if !automatic(change.Block) {
			t.Errorf("literal rewrite block %q has no automatic VCL block rule", change.Block)
		}
		if change.CaseInsensitive && !change.RemoveEmpty {
			t.Errorf("%s.%s is case-insensitive without RemoveEmpty", change.Block, change.Attribute)
		}
		for _, rule := range registry.RulesForLegacy("fastly_service_vcl." + change.Block) {
			if rule.Kind == ConstructServiceBlock && rule.HCL != HCLReshape {
				t.Errorf("block %q has literal rewrites but HCL strategy %s", change.Block, rule.HCL)
			}
		}
	}
	for _, list := range VCLEmptyListAttributes {
		if !automatic(list.Block) {
			t.Errorf("empty-list block %q has no automatic VCL block rule", list.Block)
		}
		for _, rule := range registry.RulesForLegacy("fastly_service_vcl." + list.Block) {
			if rule.Kind == ConstructServiceBlock && rule.HCL != HCLReshape {
				t.Errorf("block %q has empty-list rewrites but HCL strategy %s", list.Block, rule.HCL)
			}
		}
	}
}
