// Package legacystate holds helpers shared by state movers that decode state
// written by the legacy SDKv2 Fastly provider.
package legacystate

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ProviderAddress is the registry address of the legacy SDKv2 provider.
const ProviderAddress = "registry.terraform.io/fastly/fastly"

// HasValue reports whether v is known, non-null and non-empty.
func HasValue(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueString() != ""
}

// AcceptMove reports whether req moves schema version 0 state of typeName from
// the legacy provider into target. Other sources return false without
// diagnostics so that Terraform can try the remaining movers.
func AcceptMove(req resource.MoveStateRequest, typeName, target string, diags *diag.Diagnostics) bool {
	if req.SourceProviderAddress != ProviderAddress || req.SourceTypeName != typeName {
		return false
	}
	if req.SourceSchemaVersion != 0 {
		diags.AddError("Cannot migrate "+typeName, fmt.Sprintf(
			"This state was saved by a Fastly provider release that migration does not support. "+
				"Remove the `moved` block and import `%s` instead.", target))
		return false
	}
	if req.SourceState == nil {
		diags.AddError("Cannot migrate "+typeName, fmt.Sprintf(
			"The existing state could not be read. Remove the `moved` block and import `%s` instead.", target))
		return false
	}
	return true
}

// AddImportError reports state that cannot be moved, and tells the user to
// import target with importID instead.
func AddImportError(diags *diag.Diagnostics, typeName, problem, target, importID string) {
	diags.AddError("Cannot migrate "+typeName, fmt.Sprintf(
		"%s Remove the `moved` block and import `%s` with the ID `%s`.", problem, target, importID))
}

// OwnedEntries returns the entries the target owns after a move: all of them
// with manage_entries = true, otherwise none. Never null, since null adopts
// every remote entry.
func OwnedEntries(manageEntries types.Bool, entries types.Map) types.Map {
	if manageEntries.ValueBool() && !entries.IsNull() && !entries.IsUnknown() {
		return entries
	}
	return types.MapValueMust(types.StringType, map[string]attr.Value{})
}
