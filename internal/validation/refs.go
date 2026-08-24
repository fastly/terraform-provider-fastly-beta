package validation

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// UniqueNames errors if two items share a name, which would otherwise silently collapse in
// reconcile.Run's by-name map. kind labels the resource type in the error message.
func UniqueNames[T any](items []T, kind string, getName func(T) types.String) error {
	seen := make(map[string]struct{}, len(items))

	for _, item := range items {
		name := getName(item)
		if name.IsUnknown() || name.IsNull() {
			continue
		}

		v := name.ValueString()
		if _, ok := seen[v]; ok {
			return fmt.Errorf("multiple %ss with the same name %q; names must be unique within a service version", kind, v)
		}
		seen[v] = struct{}{}
	}

	return nil
}

// NameSet builds References' validNames argument from the referenced resource's items.
func NameSet[T any](items []T, getName func(T) types.String) map[string]struct{} {
	names := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := getName(item)
		if name.IsUnknown() || name.IsNull() {
			continue
		}
		names[name.ValueString()] = struct{}{}
	}
	return names
}

// References rejects a getRefs reference absent from validNames, catching at plan time what
// would otherwise surface as a 404 at apply time. getRefs should skip unknown/unset names.
func References[T any](items []T, kind string, getName func(T) types.String, attrLabel string, getRefs func(T) []string, refKind string, validNames map[string]struct{}) error {
	for _, item := range items {
		for _, ref := range getRefs(item) {
			if ref == "" {
				continue
			}
			if _, ok := validNames[ref]; !ok {
				return fmt.Errorf("%s %q: %s %q does not match any configured %s", kind, getName(item).ValueString(), attrLabel, ref, refKind)
			}
		}
	}

	return nil
}
