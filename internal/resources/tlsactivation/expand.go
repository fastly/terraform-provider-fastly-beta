package tlsactivation

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly"
)

func buildCreateInput(plan Model) *fastly.CreateTLSActivationInput {
	input := &fastly.CreateTLSActivationInput{
		Certificate: &fastly.CustomTLSCertificate{ID: service.StringValue(plan.CertificateID)},
		Domain:      &fastly.TLSDomain{ID: service.StringValue(plan.Domain)},
	}

	if configID := service.StringValue(plan.ConfigurationID); configID != "" {
		input.Configuration = &fastly.TLSConfiguration{ID: configID}
	}

	return input
}

// Certificate is always sent: the API rejects an update with neither it nor MutualAuthentication.
func buildUpdateInput(id string, plan Model) *fastly.UpdateTLSActivationInput {
	input := &fastly.UpdateTLSActivationInput{
		ID:          id,
		Certificate: &fastly.CustomTLSCertificate{ID: service.StringValue(plan.CertificateID)},
	}

	if mtlsID := service.StringValue(plan.MutualAuthenticationID); mtlsID != "" {
		input.MutualAuthentication = &fastly.TLSMutualAuthentication{ID: mtlsID}
	}

	return input
}
