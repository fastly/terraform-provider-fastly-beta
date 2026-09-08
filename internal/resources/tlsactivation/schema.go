package tlsactivation

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID                     types.String `tfsdk:"id"`
	CertificateID          types.String `tfsdk:"certificate_id"`
	ConfigurationID        types.String `tfsdk:"configuration_id"`
	CreatedAt              types.String `tfsdk:"created_at"`
	Domain                 types.String `tfsdk:"domain"`
	MutualAuthenticationID types.String `tfsdk:"mutual_authentication_id"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Alphanumeric string identifying a TLS activation.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"certificate_id": schema.StringAttribute{
			Required:    true,
			Description: "ID of certificate to use. Must have the `domain` specified in the certificate's Subject Alternative Names.",
		},
		"configuration_id": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Description: "ID of TLS configuration to be used to terminate TLS traffic, or use the default one if missing.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
				stringplanmodifier.RequiresReplace(),
			},
		},
		"created_at": schema.StringAttribute{
			Computed:    true,
			Description: "Time-stamp (GMT) when TLS was enabled.",
		},
		"domain": schema.StringAttribute{
			Required:    true,
			Description: "Domain to enable TLS on. Must be assigned to an existing Fastly Service.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"mutual_authentication_id": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Description: "An alphanumeric string identifying a mutual authentication.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
	}
}
