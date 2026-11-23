package botmanagementcategory

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

func FlattenToModel(workspaceID string, category *policy.Category) Model {
	return Model{
		ID:          types.StringValue(workspaceID + "/" + category.CategoryID),
		WorkspaceID: types.StringValue(workspaceID),
		CategoryID:  types.StringValue(category.CategoryID),
		Action:      types.StringValue(category.Action.Type),
		Name:        types.StringValue(category.Name),
		Description: types.StringValue(category.Description),
	}
}
