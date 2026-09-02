package tlsactivation

import (
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

func flattenToModel(a *fastly.TLSActivation) Model {
	m := Model{
		ID:                     types.StringValue(a.ID),
		CertificateID:          types.StringNull(),
		ConfigurationID:        types.StringNull(),
		CreatedAt:              types.StringNull(),
		Domain:                 types.StringNull(),
		MutualAuthenticationID: types.StringNull(),
	}

	if a.Certificate != nil {
		m.CertificateID = types.StringValue(a.Certificate.ID)
	}
	if a.Configuration != nil {
		m.ConfigurationID = types.StringValue(a.Configuration.ID)
	}
	if a.Domain != nil {
		m.Domain = types.StringValue(a.Domain.ID)
	}
	if a.CreatedAt != nil {
		m.CreatedAt = types.StringValue(a.CreatedAt.Format(time.RFC3339))
	}
	if a.MutualAuthentication != nil {
		m.MutualAuthenticationID = types.StringValue(a.MutualAuthentication.ID)
	}

	return m
}
