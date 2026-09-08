package domainmanagement

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID          types.String `tfsdk:"id"`
	Description types.String `tfsdk:"description"`
	FQDN        types.String `tfsdk:"fqdn"`
	ServiceID   types.String `tfsdk:"service_id"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The Domain Identifier (UUID).",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"description": schema.StringAttribute{
			Optional:    true,
			Description: "The description for your domain.",
		},
		"fqdn": schema.StringAttribute{
			Required:    true,
			Description: "The fully-qualified domain name for your domain (e.g. `www.example.com`). Can be created, but not updated.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"service_id": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Description: "The service_id associated with your domain or null if there is no association. Computed so that a link created via `fastly_domain_service_link` is picked up on refresh instead of being planned away.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
	}
}
