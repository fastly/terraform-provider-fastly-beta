package apisecurityoperationtag

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID             types.String `tfsdk:"id"`
	ServiceID      types.String `tfsdk:"service_id"`
	TagID          types.String `tfsdk:"tag_id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	OperationCount types.Int64  `tfsdk:"operation_count"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Alphanumeric string identifying the resource.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"service_id": schema.StringAttribute{
			Required:    true,
			Description: "Service ID the tag belongs to.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"tag_id": schema.StringAttribute{
			Computed:    true,
			Description: "The tag ID.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "The name of the operation tag.",
		},
		"description": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(""),
			Description: "The description of the operation tag.",
		},
		"operation_count": schema.Int64Attribute{
			Computed:    true,
			Description: "Number of operations associated with this tag (when present).",
		},
		"created_at": schema.StringAttribute{
			Computed:    true,
			Description: "Created timestamp (when present).",
		},
		"updated_at": schema.StringAttribute{
			Computed:    true,
			Description: "Updated timestamp (when present).",
		},
	}
}
