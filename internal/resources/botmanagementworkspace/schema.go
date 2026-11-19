package botmanagementworkspace

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ProtectionModes are the values accepted by the workspace protection_mode.
var ProtectionModes = []string{"off", "log", "block"}

type Model struct {
	ID             types.String `tfsdk:"id"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	ProtectionMode types.String `tfsdk:"protection_mode"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of the Bot Management workspace. Same as `workspace_id`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"workspace_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the Bot Management workspace to manage. Use the `fastly_bot_management_workspace` data source to look it up from a service ID.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"name": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Description: "The display name of the Bot Management workspace. Must be between 1 and 255 characters. When omitted, the current name is kept.",
			Validators: []validator.String{
				stringvalidator.LengthBetween(1, 255),
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"description": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Description: "A description of the Bot Management workspace. Must be at most 1000 characters. When omitted, the current description is kept.",
			Validators: []validator.String{
				stringvalidator.LengthAtMost(1000),
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"protection_mode": schema.StringAttribute{
			Required:    true,
			Description: "The protection mode of the Bot Management workspace, which gates all enforcement. One of `off` (no enforcement), `log` (bots are detected and logged, but no requests are blocked or challenged), or `block` (category and bot actions are enforced).",
			Validators: []validator.String{
				stringvalidator.OneOf(ProtectionModes...),
			},
		},
	}
}
