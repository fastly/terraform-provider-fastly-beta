package tlssubscriptionvalidation

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

// flattenToModel keys validity off certificate presence, not subscription state - "issued" and
// "renewing" both count, avoiding destroy/recreate churn during renewals. An empty CertificateID
// signals the caller to treat the resource as not (or no longer) validated.
func flattenToModel(subscription *fastly.TLSSubscription, subscriptionID string) Model {
	certificateID := ""
	if len(subscription.Certificates) > 0 {
		certificateID = subscription.Certificates[0].ID
	}

	if certificateID == "" {
		return Model{
			ID:             types.StringNull(),
			CertificateID:  types.StringNull(),
			SubscriptionID: types.StringValue(subscriptionID),
		}
	}

	return Model{
		ID:             types.StringValue(subscriptionID),
		CertificateID:  types.StringValue(certificateID),
		SubscriptionID: types.StringValue(subscriptionID),
	}
}
