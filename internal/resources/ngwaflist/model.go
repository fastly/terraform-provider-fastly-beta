package ngwaflist

import "github.com/hashicorp/terraform-plugin-framework/types"

// Model is the shared state model for all workspace-scoped NGWAF list resources.
// The list type is implicit in the concrete Terraform resource name.
type Model struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Entries     types.List   `tfsdk:"entries"`
	ReferenceID types.String `tfsdk:"reference_id"`
}
