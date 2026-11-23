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
		diags.AddError(
			"Unsupported legacy Fastly state",
			fmt.Sprintf("%s can only move state from %s schema version 0; received schema version %d.", target, typeName, req.SourceSchemaVersion),
		)
		return false
	}
	if req.SourceState == nil {
		diags.AddError(
			"Unable to decode legacy Fastly state",
			fmt.Sprintf("Terraform Plugin Framework did not provide decoded source state for %s schema version 0.", typeName),
		)
		return false
	}
	return true
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
