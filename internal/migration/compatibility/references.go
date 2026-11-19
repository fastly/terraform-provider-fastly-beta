package compatibility

import "strings"

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
	// Arbitrary object or collection reshapes need separate semantic evidence.
	return path, path != "" && !strings.Contains(path, ".")
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
