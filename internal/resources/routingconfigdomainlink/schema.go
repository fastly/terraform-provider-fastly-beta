package routingconfigdomainlink

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID              types.String `tfsdk:"id"`
	DomainID        types.String `tfsdk:"domain_id"`
	RoutingConfigID types.String `tfsdk:"routing_config_id"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of this resource (identical to `domain_id`).",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"domain_id": schema.StringAttribute{
			Required:    true,
			Description: "The Domain Identifier of the versionless domain being linked (UUID).",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"routing_config_id": schema.StringAttribute{
			Required:    true,
			Description: "The routing config identifier to link to the domain.",
		},
	}
}
