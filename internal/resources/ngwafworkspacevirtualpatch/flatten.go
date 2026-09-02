package ngwafworkspacevirtualpatch

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	vp "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/virtualpatches"
)

func FlattenToModel(workspaceID string, virtualPatch *vp.VirtualPatch) (Model, error) {
	if virtualPatch == nil {
		return Model{}, fmt.Errorf("cannot flatten nil NGWAF workspace virtual patch")
	}
	if virtualPatch.ID == "" {
		return Model{}, fmt.Errorf("invalid NGWAF workspace virtual patch: id is missing")
	}

	return Model{
		ID:             types.StringValue(virtualPatch.ID),
		WorkspaceID:    types.StringValue(workspaceID),
		VirtualPatchID: types.StringValue(virtualPatch.ID),
		Mode:           types.StringValue(virtualPatch.Mode),
		Enabled:        types.BoolValue(virtualPatch.Enabled),
		Description:    types.StringValue(virtualPatch.Description),
	}, nil
}
