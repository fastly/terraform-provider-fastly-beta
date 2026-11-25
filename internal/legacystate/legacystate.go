// Package legacystate holds helpers shared by state movers that decode state
// written by the legacy SDKv2 Fastly provider.
package legacystate

import (
	"fmt"

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
		AddMoveError(diags, typeName, "was saved by an unsupported provider release", target, "")
		return false
	}
	if req.SourceState == nil {
		AddMoveError(diags, typeName, "can't be read", target, "")
		return false
	}
	return true
}

// AddMoveError reports state of typeName that cannot be moved and how to
// recover: remove it from state, then import it as target. importWith, when
// set, says which ID to import with, such as "the ACL ID".
func AddMoveError(diags *diag.Diagnostics, typeName, problem, target, importWith string) {
	importStep := fmt.Sprintf("import it as `%s`", target)
	if importWith != "" {
		importStep += " with " + importWith
	}
	diags.AddError("Cannot migrate "+typeName, fmt.Sprintf(
		"This `%s` state %s, so it can't be moved. Instead, remove it from state with a `removed` block "+
			"(`lifecycle { destroy = false }`) or `terraform state rm`, then %s.", typeName, problem, importStep))
}
