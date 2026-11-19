package botmanagementworkspace

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	ws "github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces"
)

func FlattenToModel(workspace *ws.Workspace) Model {
	return Model{
		ID:             types.StringValue(workspace.ID),
		WorkspaceID:    types.StringValue(workspace.ID),
		Name:           types.StringValue(workspace.Name),
		Description:    types.StringValue(workspace.Description),
		ProtectionMode: types.StringValue(workspace.ProtectionMode),
	}
}
