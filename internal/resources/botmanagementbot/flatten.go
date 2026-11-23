package botmanagementbot

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

// FlattenToModel takes categoryID from the caller because the single-bot
// response doesn't include it.
func FlattenToModel(workspaceID, categoryID string, bot *policy.Bot) Model {
	return Model{
		ID:          types.StringValue(workspaceID + "/" + categoryID + "/" + bot.BotID),
		WorkspaceID: types.StringValue(workspaceID),
		CategoryID:  types.StringValue(categoryID),
		BotID:       types.StringValue(bot.BotID),
		Action:      types.StringValue(bot.Action.Type),
		Name:        types.StringValue(bot.Name),
	}
}
