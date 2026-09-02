package tlsmutualauthentication

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

// linkActivations links each activation to the mTLS object in order, stopping at the first
// error. It returns the IDs successfully linked so far so a caller can persist partial progress.
func linkActivations(ctx context.Context, client *fastly.Client, activationIDs []string, mtlsID string) ([]string, error) {
	linked := make([]string, 0, len(activationIDs))
	for _, activationID := range activationIDs {
		if err := setActivationMTLS(ctx, client, activationID, mtlsID); err != nil {
			return linked, err
		}
		linked = append(linked, activationID)
	}
	return linked, nil
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

func stringsToSet(ids []string) types.Set {
	if len(ids) == 0 {
		return types.SetNull(types.StringType)
	}
	elems := make([]attr.Value, 0, len(ids))
	for _, id := range ids {
		elems = append(elems, types.StringValue(id))
	}
	return types.SetValueMust(types.StringType, elems)
}
