package tlssubscriptionvalidation

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID             types.String   `tfsdk:"id"`
	CertificateID  types.String   `tfsdk:"certificate_id"`
	SubscriptionID types.String   `tfsdk:"subscription_id"`
	Timeouts       timeouts.Value `tfsdk:"timeouts"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of this resource. Matches `subscription_id` once the subscription has been validated.",
		},
		"certificate_id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of the certificate issued for the validated subscription. Only populated once the subscription reaches the `issued` state. `fastly_tls_subscription` activates TLS on its domains automatically, so this attribute is informational only and should not be used to create a `fastly_tls_activation` for those domains.",
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

func ResourceBlocks(ctx context.Context) map[string]schema.Block {
	return map[string]schema.Block{
		"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true}),
	}
}
