package ngwafrule

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// SetWorkspaceIdentity records the identity of a workspace-scoped rule. The
// identity is nil when Terraform doesn't request one, such as in older cores.
func SetWorkspaceIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics, workspaceID, id types.String) {
	if identity == nil || diags.HasError() {
		return
	}
	diags.Append(identity.Set(ctx, resourceidentity.NGWAFWorkspaceObject(workspaceID, id))...)
}

// SetAccountIdentity records the identity of an account-scoped rule.
func SetAccountIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics, id types.String) {
	if identity == nil || diags.HasError() {
		return
	}
	diags.Append(identity.Set(ctx, resourceidentity.NGWAFAccountObject(id))...)
}
