package tlsplatformcertificate

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

// FlattenToModel populates the attributes the Fastly API actually returns
// for a bulk certificate. It deliberately leaves certificate_body,
// intermediates_blob, and allow_untrusted_root unset: GetBulkCertificate
// never returns them, so the resource carries those write-only values
// forward from the plan (create/update) or prior state (read) instead - see
// carryForwardWriteOnly.
func FlattenToModel(ctx context.Context, certificate *fastly.BulkCertificate) (Model, diag.Diagnostics) {
	var diags diag.Diagnostics

	var configurationID string
	if len(certificate.Configurations) > 0 && certificate.Configurations[0] != nil {
		configurationID = certificate.Configurations[0].ID
	}

	domainNames := make([]string, 0, len(certificate.Domains))
	for _, d := range certificate.Domains {
		if d != nil {
			domainNames = append(domainNames, d.ID)
		}
	}
	domains, domainDiags := types.SetValueFrom(ctx, types.StringType, domainNames)
	diags.Append(domainDiags...)

	m := Model{
		ID:              types.StringValue(certificate.ID),
		ConfigurationID: types.StringValue(configurationID),
		NotAfter:        formatTime(certificate.NotAfter),
		NotBefore:       formatTime(certificate.NotBefore),
		CreatedAt:       formatTime(certificate.CreatedAt),
		UpdatedAt:       formatTime(certificate.UpdatedAt),
		Replace:         types.BoolValue(certificate.Replace),
		Domains:         domains,
	}

	if certificate.Replace {
		diags.AddWarning(
			"Certificate recommended for replacement",
			fmt.Sprintf("Fastly recommends that this certificate (%s) be replaced.", certificate.ID),
		)
	}

	return m, diags
}

// carryForwardWriteOnly copies the fields the Fastly API never returns on
// read (certificate_body, intermediates_blob, allow_untrusted_root) from src
// into dst, so a refresh does not null them out.
func carryForwardWriteOnly(dst *Model, src Model) {
	dst.CertificateBody = src.CertificateBody
	dst.IntermediatesBlob = src.IntermediatesBlob
	dst.AllowUntrustedRoot = src.AllowUntrustedRoot
}

func formatTime(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
