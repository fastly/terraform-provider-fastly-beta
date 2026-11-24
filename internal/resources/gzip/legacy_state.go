package gzip

import (
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// LegacyMoveState retains each gzip name and whether its content_types and
// extensions were unset. The API fills unset lists with its own defaults,
// so the first Read needs these hints to keep them null, as SDKv2 did.
// All other fields are refreshed from the API.
func LegacyMoveState(source types.Set) ([]NestedModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if source.IsNull() {
		return nil, diags
	}
	invalid := func() {
		diags.AddError("Unsupported legacy gzip state",
			"A gzip block in the fastly_service_vcl state has a missing, unreadable or duplicate name, so the service cannot be moved. Run terraform apply -refresh-only with the legacy provider, then run the migration again.")
	}
	if source.IsUnknown() {
		invalid()
		return nil, diags
	}
	hint := func(value any) (types.List, bool) {
		list, ok := value.(types.List)
		if !ok || list.IsUnknown() {
			return types.List{}, false
		}
		if list.IsNull() || len(list.Elements()) == 0 {
			return types.ListNull(types.StringType), true
		}
		return list, true
	}
	var models []NestedModel
	names := make(map[string]bool)
	for _, value := range source.Elements() {
		object, ok := value.(types.Object)
		if !ok || object.IsNull() || object.IsUnknown() {
			invalid()
			return nil, diags
		}
		attrs := object.Attributes()
		name, ok := attrs["name"].(types.String)
		contentTypes, validContentTypes := hint(attrs["content_types"])
		extensions, validExtensions := hint(attrs["extensions"])
		if !ok || name.IsNull() || name.IsUnknown() || name.ValueString() == "" ||
			names[name.ValueString()] || !validContentTypes || !validExtensions {
			invalid()
			return nil, diags
		}
		names[name.ValueString()] = true
		models = append(models, NestedModel{
			Name:           name,
			CacheCondition: types.StringNull(),
			ContentTypes:   contentTypes,
			Extensions:     extensions,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name.ValueString() < models[j].Name.ValueString()
	})
	return models, diags
}
