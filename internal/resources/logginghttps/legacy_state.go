package logginghttps

import (
	"math/big"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// LegacyComputeMoveState retains the name and SDKv2 gzip-level hint that
// cannot always be reconstructed by a GET. With no prior hint the API's
// zero could mean an explicit zero or the unset sentinel (-1). A codec can
// also cause the API to choose its own gzip level. All other fields,
// including credentials, are refreshed normally from the remote endpoint.
func LegacyComputeMoveState(source types.Set) ([]ComputeNestedModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if source.IsNull() {
		return nil, diags
	}
	invalid := func() {
		diags.AddError("Unsupported legacy HTTPS logging state",
			"The legacy HTTPS logging state contains a malformed, unknown or duplicate endpoint name or gzip level. Refresh with the legacy provider before retrying migration.")
	}
	if source.IsUnknown() {
		invalid()
		return nil, diags
	}
	var models []ComputeNestedModel
	names := make(map[string]bool)
	for _, value := range source.Elements() {
		object, ok := value.(types.Object)
		if !ok || object.IsNull() || object.IsUnknown() {
			invalid()
			return nil, diags
		}
		attrs := object.Attributes()
		name, ok := attrs["name"].(types.String)
		gzip, valid := attrs["gzip_level"].(types.Number)
		if !ok || name.IsNull() || name.IsUnknown() || name.ValueString() == "" ||
			names[name.ValueString()] || !valid || gzip.IsUnknown() {
			invalid()
			return nil, diags
		}
		level := int64(DefaultGzipLevel)
		if !gzip.IsNull() {
			var accuracy big.Accuracy
			level, accuracy = gzip.ValueBigFloat().Int64()
			if accuracy != big.Exact || level < DefaultGzipLevel || level > 9 {
				invalid()
				return nil, diags
			}
		}
		names[name.ValueString()] = true
		models = append(models, ComputeNestedModel{commonModel: commonModel{
			Name: name, GzipLevel: types.Int64Value(level), TLS: types.ObjectNull(tlsAttributeTypes),
		}})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name.ValueString() < models[j].Name.ValueString()
	})
	return models, diags
}

// LegacyMoveState provides the same hint for the VCL-specific model.
func LegacyMoveState(source types.Set) ([]NestedModel, diag.Diagnostics) {
	models, diags := LegacyComputeMoveState(source)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]NestedModel, 0, len(models))
	for _, model := range models {
		result = append(result, NestedModel{commonModel: model.commonModel})
	}
	return result, diags
}
