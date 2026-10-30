package compatibility

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry provides immutable lookup access to the compatibility rules.
type Registry struct {
	rules    []Rule
	byID     map[string]Rule
	byLegacy map[string][]Rule
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
	defaultRegistryErr  error
)

// NewRegistry validates rules and returns an immutable compatibility registry.
func NewRegistry(rules []Rule) (*Registry, error) {
	if err := validateRules(rules); err != nil {
		return nil, err
	}

	registry := &Registry{
		rules:    cloneRules(rules),
		byID:     make(map[string]Rule, len(rules)),
		byLegacy: make(map[string][]Rule),
	}

	for _, rule := range registry.rules {
		registry.byID[rule.ID] = cloneRule(rule)
		registry.byLegacy[rule.Legacy] = append(registry.byLegacy[rule.Legacy], cloneRule(rule))
	}

	for legacy := range registry.byLegacy {
		sort.Slice(registry.byLegacy[legacy], func(i, j int) bool {
			left := registry.byLegacy[legacy][i]
			right := registry.byLegacy[legacy][j]
			if left.Selector != right.Selector {
				return left.Selector < right.Selector
			}
			return left.ID < right.ID
		})
	}

	return registry, nil
}

// Default returns the validated built-in legacy-to-_auto compatibility registry.
//
// The built-in rules are validated once. A validation failure indicates a provider
// development error and panics rather than allowing migration code to run with an
// ambiguous or incomplete registry.
func Default() *Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry, defaultRegistryErr = NewRegistry(allRules())
	})
	if defaultRegistryErr != nil {
		panic(defaultRegistryErr)
	}
	return defaultRegistry
}

// All returns a defensive copy of every compatibility rule.
func (r *Registry) All() []Rule {
	if r == nil {
		return nil
	}
	return cloneRules(r.rules)
}

// RuleByID returns a defensive copy of the rule with the stable identifier.
func (r *Registry) RuleByID(id string) (Rule, bool) {
	if r == nil {
		return Rule{}, false
	}
	rule, ok := r.byID[id]
	if !ok {
		return Rule{}, false
	}
	return cloneRule(rule), true
}

// RulesForLegacy returns every rule for an exact legacy construct path.
//
// An empty result is intentionally meaningful: callers such as fastly-tf-migrate
// analyze must treat an unknown legacy construct as unsupported rather than
// silently ignoring it.
func (r *Registry) RulesForLegacy(legacy string) []Rule {
	if r == nil {
		return nil
	}
	return cloneRules(r.byLegacy[legacy])
}

func allRules() []Rule {
	rules := make([]Rule, 0, len(resourceRules)+len(serviceRules)+len(productRules)+len(attributeRules))
	rules = append(rules, resourceRules...)
	rules = append(rules, serviceRules...)
	rules = append(rules, productRules...)
	rules = append(rules, attributeRules...)
	return rules
}

func validateRules(rules []Rule) error {
	var errs []error
	ids := make(map[string]struct{}, len(rules))
	routes := make(map[string]string, len(rules))

	for index, rule := range rules {
		prefix := fmt.Sprintf("rule[%d]", index)
		if rule.ID == "" {
			errs = append(errs, fmt.Errorf("%s: id is required", prefix))
		} else if _, exists := ids[rule.ID]; exists {
			errs = append(errs, fmt.Errorf("%s: duplicate id %q", prefix, rule.ID))
		} else {
			ids[rule.ID] = struct{}{}
			prefix = fmt.Sprintf("rule %q", rule.ID)
		}

		if !validConstructKind(rule.Kind) {
			errs = append(errs, fmt.Errorf("%s: invalid construct kind %q", prefix, rule.Kind))
		}
		if !validFamily(rule.Family) {
			errs = append(errs, fmt.Errorf("%s: invalid family %q", prefix, rule.Family))
		}
		if strings.TrimSpace(rule.Legacy) == "" {
			errs = append(errs, fmt.Errorf("%s: legacy construct is required", prefix))
		}
		if !validSupport(rule.Support) {
			errs = append(errs, fmt.Errorf("%s: invalid support status %q", prefix, rule.Support))
		}
		if !validHCLStrategy(rule.HCL) {
			errs = append(errs, fmt.Errorf("%s: invalid HCL strategy %q", prefix, rule.HCL))
		}
		if !validStateStrategy(rule.State) {
			errs = append(errs, fmt.Errorf("%s: invalid state strategy %q", prefix, rule.State))
		}
		if !validImportIdentity(rule.ImportIdentity) {
			errs = append(errs, fmt.Errorf("%s: invalid import identity %q", prefix, rule.ImportIdentity))
		}

		if rule.Support == SupportBlocked && strings.TrimSpace(rule.ProviderPrerequisite) == "" {
			errs = append(errs, fmt.Errorf("%s: blocked rule requires a provider prerequisite", prefix))
		}
		if rule.State == StateImport && rule.ImportIdentity == ImportNone {
			errs = append(errs, fmt.Errorf("%s: import state strategy requires an import identity", prefix))
		}
		if rule.State != StateImport && rule.ImportIdentity != ImportNone {
			errs = append(errs, fmt.Errorf("%s: import identity %q requires import state strategy", prefix, rule.ImportIdentity))
		}

		targetMayBeEmpty := rule.Support == SupportUnsupported || rule.HCL == HCLRemove || rule.HCL == HCLReject
		if strings.TrimSpace(rule.Target) == "" && !targetMayBeEmpty {
			errs = append(errs, fmt.Errorf("%s: target is required for this migration strategy", prefix))
		}

		for _, version := range rule.SourceSchemaVersions {
			if version < 0 {
				errs = append(errs, fmt.Errorf("%s: source schema version must be non-negative", prefix))
			}
		}

		routeKey := strings.Join([]string{
			string(rule.Kind),
			string(rule.Family),
			rule.Legacy,
			rule.Selector,
		}, "\x00")
		if existing, exists := routes[routeKey]; exists {
			errs = append(errs, fmt.Errorf("%s: ambiguous route duplicates rule %q", prefix, existing))
		} else {
			routes[routeKey] = rule.ID
		}
	}

	return errors.Join(errs...)
}

func validConstructKind(value ConstructKind) bool {
	switch value {
	case ConstructResource, ConstructServiceBlock, ConstructProduct, ConstructAttribute:
		return true
	default:
		return false
	}
}

func validFamily(value ServiceFamily) bool {
	switch value {
	case FamilyGlobal, FamilyCDN, FamilyCompute:
		return true
	default:
		return false
	}
}

func validSupport(value SupportStatus) bool {
	switch value {
	case SupportAutomatic, SupportManual, SupportBlocked, SupportUnsupported:
		return true
	default:
		return false
	}
}

func validHCLStrategy(value HCLStrategy) bool {
	switch value {
	case HCLUnchanged, HCLRename, HCLReshape, HCLSplit, HCLRemove, HCLReject:
		return true
	default:
		return false
	}
}

func validStateStrategy(value StateStrategy) bool {
	switch value {
	case StateUnchanged, StateMove, StateUpgrade, StateImport, StateNone:
		return true
	default:
		return false
	}
}

func validImportIdentity(value ImportIdentity) bool {
	switch value {
	case ImportNone, ImportServiceID, ImportLegacyID, ImportProviderSpecific:
		return true
	default:
		return false
	}
}

func cloneRules(rules []Rule) []Rule {
	result := make([]Rule, len(rules))
	for index, rule := range rules {
		result[index] = cloneRule(rule)
	}
	return result
}

func cloneRule(rule Rule) Rule {
	cloned := rule
	if rule.SourceSchemaVersions != nil {
		cloned.SourceSchemaVersions = append([]int64(nil), rule.SourceSchemaVersions...)
	}
	return cloned
}
