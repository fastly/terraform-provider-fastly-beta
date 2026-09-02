package tlsmutualauthentication

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

func buildCreateInput(plan Model) *fastly.CreateTLSMutualAuthenticationInput {
	input := &fastly.CreateTLSMutualAuthenticationInput{
		CertBundle: service.StringValue(plan.CertBundle),
	}
	if !plan.Enforced.IsNull() && !plan.Enforced.IsUnknown() {
		input.Enforced = plan.Enforced.ValueBool()
	}
	if name := service.StringValue(plan.Name); name != "" {
		input.Name = name
	}
	return input
}

// Enforced is always sent: the API field can't distinguish false from unset.
func buildUpdateInput(id string, plan, state Model) *fastly.UpdateTLSMutualAuthenticationInput {
	input := &fastly.UpdateTLSMutualAuthenticationInput{
		ID:         id,
		CertBundle: service.StringValue(plan.CertBundle),
		Enforced:   service.BoolValue(plan.Enforced),
	}
	if !plan.Name.Equal(state.Name) {
		input.Name = service.StringValue(plan.Name)
	}
	return input
}

func setActivationMTLS(ctx context.Context, client *fastly.Client, activationID, mtlsID string) error {
	_, err := client.UpdateTLSActivation(ctx, &fastly.UpdateTLSActivationInput{
		ID:                   activationID,
		MutualAuthentication: &fastly.TLSMutualAuthentication{ID: mtlsID},
	})
	return err
}

// unsetActivationMTLS clears mTLS via an empty ID, the reliable way to clear this relation.
func unsetActivationMTLS(ctx context.Context, client *fastly.Client, activationID string) error {
	_, err := client.UpdateTLSActivation(ctx, &fastly.UpdateTLSActivationInput{
		ID:                   activationID,
		MutualAuthentication: &fastly.TLSMutualAuthentication{ID: ""},
	})
	return err
}

func setToStringSlice(ctx context.Context, s types.Set, diags *diag.Diagnostics) []string {
	if s.IsNull() || s.IsUnknown() {
		return []string{}
	}
	values := make([]string, 0, len(s.Elements()))
	diags.Append(s.ElementsAs(ctx, &values, false)...)
	return values
}
