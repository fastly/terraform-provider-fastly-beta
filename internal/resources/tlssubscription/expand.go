package tlssubscription

import (
	"context"
	"fmt"
	"slices"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

// domainsToAPI reads plan/config domains into both the go-fastly relation shape and a plain
// string slice, the latter needed for the common_name-membership check and error messages.
func domainsToAPI(ctx context.Context, domains types.Set) ([]*fastly.TLSDomain, []string, diag.Diagnostics) {
	var domainStrings []string
	diags := domains.ElementsAs(ctx, &domainStrings, false)
	if diags.HasError() {
		return nil, nil, diags
	}

	apiDomains := make([]*fastly.TLSDomain, 0, len(domainStrings))
	for _, d := range domainStrings {
		apiDomains = append(apiDomains, &fastly.TLSDomain{ID: d})
	}
	return apiDomains, domainStrings, diags
}

func buildCreateInput(ctx context.Context, plan Model) (*fastly.CreateTLSSubscriptionInput, diag.Diagnostics) {
	domains, domainStrings, diags := domainsToAPI(ctx, plan.Domains)
	if diags.HasError() {
		return nil, diags
	}

	input := &fastly.CreateTLSSubscriptionInput{
		CertificateAuthority: service.StringValue(plan.CertificateAuthority),
		Domains:              domains,
	}

	if configurationID := service.StringValue(plan.ConfigurationID); configurationID != "" {
		input.Configuration = &fastly.TLSConfiguration{ID: configurationID}
	}

	if commonName := service.StringValue(plan.CommonName); commonName != "" {
		if !slices.Contains(domainStrings, commonName) {
			diags.AddAttributeError(path.Root("common_name"), "Invalid common_name",
				fmt.Sprintf("domain specified as common_name (%s) must also be in domains (%v)", commonName, domainStrings))
			return nil, diags
		}
		input.CommonName = &fastly.TLSDomain{ID: commonName}
	}

	return input, diags
}

// buildUpdateInput always sends domains, common_name, and configuration_id since the API needs
// all three on every update, even when only an unrelated attribute like force_update changed.
func buildUpdateInput(ctx context.Context, id string, plan Model) (*fastly.UpdateTLSSubscriptionInput, diag.Diagnostics) {
	domains, domainStrings, diags := domainsToAPI(ctx, plan.Domains)
	if diags.HasError() {
		return nil, diags
	}

	commonName := service.StringValue(plan.CommonName)
	if commonName != "" && !slices.Contains(domainStrings, commonName) {
		diags.AddAttributeError(path.Root("common_name"), "Invalid common_name",
			fmt.Sprintf("domain specified as common_name (%s) must also be in domains (%v)", commonName, domainStrings))
		return nil, diags
	}

	input := &fastly.UpdateTLSSubscriptionInput{
		ID:            id,
		Force:         service.BoolValue(plan.ForceUpdate),
		CommonName:    &fastly.TLSDomain{ID: commonName},
		Domains:       domains,
		Configuration: &fastly.TLSConfiguration{ID: service.StringValue(plan.ConfigurationID)},
	}
	return input, diags
}
