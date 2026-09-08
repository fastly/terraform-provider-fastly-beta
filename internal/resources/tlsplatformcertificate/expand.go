package tlsplatformcertificate

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly"
)

func BuildCreateInput(plan Model) *fastly.CreateBulkCertificateInput {
	return &fastly.CreateBulkCertificateInput{
		CertBlob:          service.StringValue(plan.CertificateBody),
		IntermediatesBlob: service.StringValue(plan.IntermediatesBlob),
		Configurations: []*fastly.TLSConfiguration{
			{ID: service.StringValue(plan.ConfigurationID)},
		},
		AllowUntrusted: service.BoolValue(plan.AllowUntrustedRoot),
	}
}

// BuildUpdateInput never includes Configurations: the API has no way to
// change a bulk certificate's TLS configuration after creation, matching
// configuration_id's RequiresReplace plan modifier.
func BuildUpdateInput(id string, plan Model) *fastly.UpdateBulkCertificateInput {
	return &fastly.UpdateBulkCertificateInput{
		ID:                id,
		CertBlob:          service.StringValue(plan.CertificateBody),
		IntermediatesBlob: service.StringValue(plan.IntermediatesBlob),
		AllowUntrusted:    service.BoolValue(plan.AllowUntrustedRoot),
	}
}
