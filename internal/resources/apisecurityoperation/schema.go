package apisecurityoperation

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID          types.String  `tfsdk:"id"`
	ServiceID   types.String  `tfsdk:"service_id"`
	OperationID types.String  `tfsdk:"operation_id"`
	Method      types.String  `tfsdk:"method"`
	Domain      types.String  `tfsdk:"domain"`
	Path        types.String  `tfsdk:"path"`
	Description types.String  `tfsdk:"description"`
	TagIDs      types.Set     `tfsdk:"tag_ids"`
	Status      types.String  `tfsdk:"status"`
	RPS         types.Float64 `tfsdk:"rps"`
	CreatedAt   types.String  `tfsdk:"created_at"`
	UpdatedAt   types.String  `tfsdk:"updated_at"`
	LastSeenAt  types.String  `tfsdk:"last_seen_at"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Alphanumeric string identifying the resource. Format: `service_id/operation_id`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"service_id": schema.StringAttribute{
			Required:    true,
			Description: "Service ID the operation belongs to. To import, use: <service_id>/<operation_id>.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"operation_id": schema.StringAttribute{
			Computed:    true,
			Description: "The operation ID.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"method": schema.StringAttribute{
			Required:    true,
			Description: "HTTP method for the operation (e.g. GET, POST). Can be created, but not updated.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"domain": schema.StringAttribute{
			Required:    true,
			Description: "Domain for the operation (exact match). Can be created, but not updated.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"path": schema.StringAttribute{
			Required:    true,
			Description: "Path for the operation (exact match). Can be created, but not updated.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"description": schema.StringAttribute{
			Optional:    true,
			Description: "A description of the operation.",
		},
		"tag_ids": schema.SetAttribute{
			Optional:    true,
			ElementType: types.StringType,
			Description: "Associated operation tag IDs.",
		},
		"status": schema.StringAttribute{
			Computed:    true,
			Description: "Discovery status (when present).",
		},
		"rps": schema.Float64Attribute{
			Computed:    true,
			Description: "Observed requests per second (when present).",
		},
		"created_at": schema.StringAttribute{
			Computed:    true,
			Description: "Created timestamp (when present).",
		},
		"updated_at": schema.StringAttribute{
			Computed:    true,
			Description: "Updated timestamp (when present).",
		},
		"last_seen_at": schema.StringAttribute{
			Computed:    true,
			Description: "Last seen timestamp (when present).",
		},
	}
}
