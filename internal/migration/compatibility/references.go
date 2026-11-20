package compatibility

import (
	"slices"
	"strings"
)

// unchangedReferenceOutputs lists scalar outputs that keep their name across a
// versionless resource migration. Domain description and service_id also keep
// their value except when unset: SDKv2 persists "" and the target reads null.
// The legacy-only domain_id is not listed: its registry rule renames it to id.
var unchangedReferenceOutputs = map[string][]string{
	"fastly_domain>fastly_domain":                              {"id", "description", "fqdn", "service_id"},
	"fastly_domain_v1>fastly_domain":                           {"id", "description", "fqdn", "service_id"},
	"fastly_domain_v1_service_link>fastly_domain_service_link": {"id", "domain_id", "service_id"},
}

// ReferenceAttribute resolves a scalar output, not a configurable HCL attribute.
// A removed input with default semantics does not imply an equivalent output.
// The returned path is relative to the destination resource instance.
func (r *Registry) ReferenceAttribute(sourceType, targetType, name string) (string, bool) {
	// Read-only service outputs are absent from the configurable-attribute
	// inventory. The two service schemas expose id and active_version unchanged.
	if (sourceType == "fastly_service_vcl" && targetType == "fastly_service_cdn_auto") ||
		(sourceType == "fastly_service_compute" && targetType == "fastly_service_compute_auto") {
		if name == "id" || name == "active_version" {
			return name, true
		}
	}
	if slices.Contains(unchangedReferenceOutputs[sourceType+">"+targetType], name) {
		return name, true
	}
	if outputs, ngwaf := ngwafPassThroughOutputs[targetType]; ngwaf && r.ngwafTarget(sourceType, targetType) {
		if outputs[name] {
			return name, true
		}
	}
	var path string
	for _, rule := range r.RulesForLegacy(sourceType + "." + name) {
		if rule.Kind != ConstructAttribute {
			continue
		}
		if rule.Support != SupportAutomatic || rule.Selector != "" ||
			(rule.HCL != HCLUnchanged && rule.HCL != HCLRename && rule.HCL != HCLReshape) {
			return "", false
		}
		target, ok := strings.CutPrefix(rule.Target, targetType+".")
		if !ok || target == "" || path != "" {
			return "", false
		}
		path = target
	}
	if strings.HasPrefix(path, "settings.") && sourceType == "fastly_service_vcl" {
		// The target schema defines settings as a singleton list, not an object.
		return "settings[0]." + strings.TrimPrefix(path, "settings."), true
	}
	// The alert integration secrets are the one verified object reshape: a
	// single-nested object, so the dotted path is a valid reference.
	if object, field, nested := strings.Cut(path, "."); nested && object == "authentication" &&
		strings.HasPrefix(sourceType, "fastly_ngwaf_alert_") && field != "" && !strings.Contains(field, ".") {
		return path, true
	}
	// Arbitrary object or collection reshapes need separate semantic evidence.
	return path, path != "" && !strings.Contains(path, ".")
}

// ngwafTarget reports whether targetType is a registered resource target of
// the legacy NGWAF type, so a pass-through output is never applied to an
// unrelated destination.
func (r *Registry) ngwafTarget(sourceType, targetType string) bool {
	for _, rule := range r.RulesForLegacy(sourceType) {
		if rule.Kind == ConstructResource && rule.Support == SupportAutomatic && rule.Target == targetType {
			return true
		}
	}
	return false
}

// ngwafPassThroughOutputs lists, per target type, the scalar outputs that
// keep their name and meaning from the legacy resource. Renamed outputs come
// from attribute rules instead. Outputs a target drops (such as the split
// discriminator `type`, account-list `applies_to`, or `request_logging` on
// signal rules) are deliberately absent, so references to them block
// migration. applies_to on signals and account rules is now a set, so only
// whole-value references pass; indexing is rejected as a nested path.
var ngwafPassThroughOutputs = func() map[string]map[string]bool {
	targets := map[string]map[string]bool{
		"fastly_ngwaf_request_rule":                    outputs("id", "description", "enabled", "group_operator", "request_logging", "applies_to"),
		"fastly_ngwaf_signal_rule":                     outputs("id", "description", "enabled", "group_operator", "applies_to"),
		"fastly_ngwaf_workspace_request_rule":          outputs("id", "description", "enabled", "group_operator", "request_logging", "workspace_id"),
		"fastly_ngwaf_workspace_signal_rule":           outputs("id", "description", "enabled", "group_operator", "workspace_id"),
		"fastly_ngwaf_workspace_rate_limit_rule":       outputs("id", "description", "enabled", "group_operator", "workspace_id"),
		"fastly_ngwaf_workspace_templated_signal_rule": outputs("id", "enabled", "group_operator", "workspace_id"),
		"fastly_ngwaf_signal":                          outputs("id", "name", "description", "reference_id", "applies_to"),
		"fastly_ngwaf_workspace_redaction":             outputs("id", "field", "type", "workspace_id"),
		"fastly_ngwaf_workspace_threshold":             outputs("id", "action", "dont_notify", "duration", "enabled", "interval", "limit", "name", "signal", "workspace_id"),
		"fastly_ngwaf_workspace_virtual_patch":         outputs("id", "enabled", "virtual_patch_id", "workspace_id"),
	}
	for _, kind := range []string{"ip", "string", "country", "wildcard", "signal"} {
		targets["fastly_ngwaf_"+kind+"_list"] = outputs("id", "name", "description", "entries")
		targets["fastly_ngwaf_workspace_"+kind+"_list"] = outputs("id", "name", "description", "entries", "workspace_id")
	}
	alerts := map[string][]string{
		"datadog":         {"site"},
		"jira":            {"host", "issue_type", "project", "username"},
		"mailing_list":    nil,
		"microsoft_teams": nil,
		"opsgenie":        nil,
		"pagerduty":       nil,
		"slack":           nil,
		"webhook":         nil,
	}
	for suffix, extra := range alerts {
		targets["fastly_ngwaf_workspace_alert_"+suffix+"_integration"] = outputs(append([]string{"id", "description", "workspace_id"}, extra...)...)
	}
	return targets
}()

func outputs(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// NGWAFPassThroughOutputs returns the pass-through outputs of every NGWAF
// target type, for schema consistency tests.
func NGWAFPassThroughOutputs() map[string][]string {
	result := make(map[string][]string, len(ngwafPassThroughOutputs))
	for target, set := range ngwafPassThroughOutputs {
		for name := range set {
			result[target] = append(result[target], name)
		}
	}
	return result
}

// ReferenceIdentityCollection identifies the narrowly supported set-to-list
// projections. Both schemas expose these scalar names and container IDs; map
// comprehensions keyed by name do not depend on collection order.
func (r *Registry) ReferenceIdentityCollection(sourceType, targetType, block string) (targetBlock, identity, targetIdentity string, ok bool) {
	switch sourceType + "." + block {
	case "fastly_service_vcl.acl":
		identity = "acl_id"
	case "fastly_service_vcl.dictionary", "fastly_service_compute.dictionary":
		identity = "dictionary_id"
	case "fastly_service_vcl.dynamicsnippet":
		identity = "snippet_id"
	case "fastly_service_vcl.rate_limiter":
		identity = "ratelimiter_id"
	default:
		return "", "", "", false
	}
	for _, rule := range r.RulesForLegacy(sourceType + "." + block) {
		if rule.Kind != ConstructServiceBlock || rule.Support != SupportAutomatic ||
			rule.Selector != "" || (rule.HCL != HCLUnchanged && rule.HCL != HCLRename && rule.HCL != HCLReshape) {
			continue
		}
		target, found := strings.CutPrefix(rule.Target, targetType+".")
		if !found || target == "" || strings.Contains(target, ".") {
			return "", "", "", false
		}
		targetIdentity = identity
		// A read-only identity rename is safe inside the verified scalar
		// projection, even though writing a computed field would be invalid.
		for _, field := range r.RulesForLegacy(sourceType + "." + block + "." + identity) {
			if field.Kind != ConstructAttribute {
				continue
			}
			destination, found := strings.CutPrefix(field.Target, targetType+"."+target+".")
			if !found || destination == "" || strings.Contains(destination, ".") ||
				field.Support != SupportAutomatic || field.Selector != "" ||
				(field.HCL != HCLUnchanged && field.HCL != HCLRename) {
				return "", "", "", false
			}
			targetIdentity = destination
		}
		return target, identity, targetIdentity, true
	}
	return "", "", "", false
}
