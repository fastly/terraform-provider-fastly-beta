package ngwafworkspacevirtualpatch

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafquery"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"

	"github.com/fastly/go-fastly/v19/fastly"
	vp "github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/workspaces/virtualpatches"
)

func NewListResource() list.ListResource {
	return ngwafquery.NewListResource(ngwafquery.ListDefinition[vp.VirtualPatch]{
		TypeName: "_ngwaf_workspace_virtual_patch",
		Noun:     "workspace virtual patches",
		List: func(ctx context.Context, client *fastly.Client, workspaceID string) ([]vp.VirtualPatch, error) {
			return vp.List(ctx, client, &vp.ListInput{WorkspaceID: &workspaceID})
		},
		ID:          func(patch *vp.VirtualPatch) string { return patch.ID },
		DisplayName: func(_ *vp.VirtualPatch) []string { return nil },
		Flatten: func(_ context.Context, workspaceID string, patch *vp.VirtualPatch) (any, diag.Diagnostics) {
			var diags diag.Diagnostics
			model, err := FlattenToModel(workspaceID, patch)
			if err != nil {
				diags.AddError("Error reading NGWAF workspace virtual patch", err.Error())
			}
			return &model, diags
		},
	})
}
