package tlssubscription

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

// includeTLSAuthorizations is passed to GetTLSSubscription so the response carries the
// challenge data needed to populate managed_dns_challenges/managed_http_challenges.
const includeTLSAuthorizations = "tls_authorizations"

func getSubscription(ctx context.Context, client *fastly.Client, id string) (*fastly.TLSSubscription, error) {
	include := includeTLSAuthorizations
	return client.GetTLSSubscription(ctx, &fastly.GetTLSSubscriptionInput{ID: id, Include: &include})
}

func flattenToModel(ctx context.Context, client *fastly.Client, subscription *fastly.TLSSubscription) (Model, diag.Diagnostics) {
	var diags diag.Diagnostics

	certificateID := ""
	if len(subscription.Certificates) > 0 {
		certificateID = subscription.Certificates[0].ID
	}

	commonName := ""
	if subscription.CommonName != nil {
		commonName = subscription.CommonName.ID
	}

	domainElems := make([]attr.Value, 0, len(subscription.Domains))
	for _, d := range subscription.Domains {
		if d == nil {
			continue
		}
		domainElems = append(domainElems, types.StringValue(d.ID))
	}
	domainsSet, setDiags := types.SetValue(types.StringType, domainElems)
	diags.Append(setDiags...)

	dnsChallenges, httpChallenges, challengeDiags := flattenChallenges(subscription.Authorizations)
	diags.Append(challengeDiags...)
	if diags.HasError() {
		return Model{}, diags
	}

	m := Model{
		ID:                    types.StringValue(subscription.ID),
		CertificateAuthority:  types.StringValue(subscription.CertificateAuthority),
		CertificateID:         types.StringValue(certificateID),
		CommonName:            types.StringValue(commonName),
		ConfigurationID:       types.StringValue(resolveConfigurationID(ctx, client, subscription, certificateID)),
		Domains:               domainsSet,
		ManagedDNSChallenges:  dnsChallenges,
		ManagedHTTPChallenges: httpChallenges,
		State:                 types.StringValue(subscription.State),
		CreatedAt:             types.StringNull(),
		UpdatedAt:             types.StringNull(),
	}
	if subscription.CreatedAt != nil {
		m.CreatedAt = types.StringValue(subscription.CreatedAt.Format(time.RFC3339))
	}
	if subscription.UpdatedAt != nil {
		m.UpdatedAt = types.StringValue(subscription.UpdatedAt.Format(time.RFC3339))
	}

	return m, diags
}

// resolveConfigurationID: a subscription's own tls_configuration relation stays fixed across a
// renewal, so once a certificate has been issued, the most recent activation's configuration (if
// any) is the authoritative value.
//
// The domain lookup is skipped when there's no issued certificate yet (a pending/processing
// subscription) - filtering by an empty certificate ID there would list every TLS domain on the
// account and match on an unrelated domain's activation.
func resolveConfigurationID(ctx context.Context, client *fastly.Client, subscription *fastly.TLSSubscription, certificateID string) string {
	configurationID := ""
	if subscription.Configuration != nil {
		configurationID = subscription.Configuration.ID
	}
	if certificateID == "" {
		return configurationID
	}

	domains, err := client.ListTLSDomains(ctx, &fastly.ListTLSDomainsInput{
		FilterTLSCertificateID: certificateID,
		Include:                "tls_activations",
		Sort:                   "tls_activations.created_at",
	})
	if err != nil {
		return configurationID
	}
	return latestActivationConfigurationID(domains, configurationID)
}

// latestActivationConfigurationID is the pure part of resolveConfigurationID, isolated so it's
// testable without a live API client.
func latestActivationConfigurationID(domains []*fastly.TLSDomain, fallback string) string {
	for _, domain := range domains {
		if domain.Activations == nil {
			break
		}
		if len(domain.Activations) > 0 {
			return domain.Activations[0].Configuration.ID
		}
	}
	return fallback
}

func flattenChallenges(authorizations []*fastly.TLSAuthorizations) (dns, http types.Set, diags diag.Diagnostics) {
	dnsElems := make([]attr.Value, 0)
	httpElems := make([]attr.Value, 0)

	for _, authorization := range authorizations {
		if authorization == nil {
			continue
		}
		for _, challenge := range authorization.Challenges {
			if challenge.Type == "managed-dns" {
				if len(challenge.Values) < 1 {
					diags.AddError("Error reading TLS subscription", "fastly API returned no record values for a managed DNS challenge")
					continue
				}
				obj, objDiags := types.ObjectValue(managedDNSChallengeAttrTypes, map[string]attr.Value{
					"record_name":  types.StringValue(challenge.RecordName),
					"record_type":  types.StringValue(challenge.RecordType),
					"record_value": types.StringValue(challenge.Values[0]),
				})
				diags.Append(objDiags...)
				dnsElems = append(dnsElems, obj)
				continue
			}

			valueElems := make([]attr.Value, 0, len(challenge.Values))
			for _, v := range challenge.Values {
				valueElems = append(valueElems, types.StringValue(v))
			}
			values, valuesDiags := types.SetValue(types.StringType, valueElems)
			diags.Append(valuesDiags...)

			obj, objDiags := types.ObjectValue(managedHTTPChallengeAttrTypes, map[string]attr.Value{
				"record_name":   types.StringValue(challenge.RecordName),
				"record_type":   types.StringValue(challenge.RecordType),
				"record_values": values,
			})
			diags.Append(objDiags...)
			httpElems = append(httpElems, obj)
		}
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: managedDNSChallengeAttrTypes}), types.SetNull(types.ObjectType{AttrTypes: managedHTTPChallengeAttrTypes}), diags
	}

	dns, dnsDiags := types.SetValue(types.ObjectType{AttrTypes: managedDNSChallengeAttrTypes}, dnsElems)
	diags.Append(dnsDiags...)
	http, httpDiags := types.SetValue(types.ObjectType{AttrTypes: managedHTTPChallengeAttrTypes}, httpElems)
	diags.Append(httpDiags...)

	return dns, http, diags
}
