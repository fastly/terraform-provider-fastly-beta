package botmanagementbot

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// InheritAction makes a bot use its category's action.
const InheritAction = "inherit"

// Actions are the values accepted by a bot action.
var Actions = []string{"allow", "block", "challenge", InheritAction}

type Model struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CategoryID  types.String `tfsdk:"category_id"`
	BotID       types.String `tfsdk:"bot_id"`
	Action      types.String `tfsdk:"action"`
	Name        types.String `tfsdk:"name"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The resource ID, in the form `workspace_id/category_id/bot_id`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"workspace_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the Bot Management workspace the bot belongs to.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"category_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the category the bot belongs to, such as `cat_ai_crawler`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"bot_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the bot, such as `bot_gptbot`. Use the `fastly_bot_management_bots` data source to list the available IDs.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"action": schema.StringAttribute{
			Required:    true,
			Description: "The action applied to requests from the bot. One of `allow`, `block`, or `challenge` to override the category's action, or `inherit` to use the category's action. Destroying this resource sets the action to `inherit`.",
			Validators: []validator.String{
				stringvalidator.OneOf(Actions...),
			},
		},
		"name": schema.StringAttribute{
			Computed:    true,
			Description: "The display name of the bot.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
	}
}
