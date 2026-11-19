package botmanagementcategory

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Actions are the values accepted by a category action.
var Actions = []string{"allow", "block", "challenge"}

type Model struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CategoryID  types.String `tfsdk:"category_id"`
	Action      types.String `tfsdk:"action"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The resource ID, in the form `workspace_id/category_id`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"workspace_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the Bot Management workspace the category belongs to.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"category_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the bot category, such as `cat_ai_crawler`. Use the `fastly_bot_management_categories` data source to list the available IDs.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"action": schema.StringAttribute{
			Required:    true,
			Description: "The action applied to every bot in the category that is set to `inherit`, including bots added to the category later. One of `allow`, `block`, or `challenge`.",
			Validators: []validator.String{
				stringvalidator.OneOf(Actions...),
			},
		},
		"name": schema.StringAttribute{
			Computed:    true,
			Description: "The display name of the category.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"description": schema.StringAttribute{
			Computed:    true,
			Description: "A description of the bots the category contains.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
	}
}
