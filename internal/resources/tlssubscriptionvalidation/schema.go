package tlssubscriptionvalidation

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID             types.String `tfsdk:"id"`
	CertificateID  types.String `tfsdk:"certificate_id"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of this resource. Matches `subscription_id` once the subscription has been validated.",
		},
		"certificate_id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of the certificate issued for the validated subscription. Only populated once the subscription reaches the `issued` state. Reference this from `fastly_tls_activation.certificate_id` to guarantee the activation is created after the certificate exists, within a single apply.",
		},
		"subscription_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the TLS Subscription that should be validated.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
	}
}
