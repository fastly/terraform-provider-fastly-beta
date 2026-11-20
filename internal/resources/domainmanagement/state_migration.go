package domainmanagement

import (
	"context"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/legacystate"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	legacyDomainV1TypeName = "fastly_domain_v1"
	legacyDomainSchemaV0   = int64(0)
)

var (
	_ resource.ResourceWithMoveState    = &Resource{}
	_ resource.ResourceWithUpgradeState = &Resource{}
)

// currentSchemaVersion is 1 because version 0 state, written by the legacy
// SDKv2 provider, stores an unset description as "" and carries domain_id.
const currentSchemaVersion = int64(1)

// legacyDomainMoveStateSchema is decode-only. The SDKv2 fastly_domain and the
// deprecated fastly_domain_v1 alias share one schema at version 0; unlike the
// target, it persists the domain UUID twice, as id and as domain_id.
func legacyDomainMoveStateSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{Computed: true},
			"domain_id":   schema.StringAttribute{Computed: true},
			"fqdn":        schema.StringAttribute{Computed: true},
			"id":          schema.StringAttribute{Computed: true},
			"service_id":  schema.StringAttribute{Computed: true},
		},
	}
}

// UpgradeState upgrades version 0 fastly_domain state, which is what the
// legacy SDKv2 provider persisted under the same type name.
func (r *Resource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	priorSchema := legacyDomainMoveStateSchema()
	return map[int64]resource.StateUpgrader{
		legacyDomainSchemaV0: {
			PriorSchema:   &priorSchema,
			StateUpgrader: upgradeLegacyDomainState,
		},
	}
}

func upgradeLegacyDomainState(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	if req.State == nil {
		resp.Diagnostics.AddError(
			"Unable to decode legacy Fastly domain state",
			"Terraform Plugin Framework did not provide decoded prior state for fastly_domain schema version 0.",
		)
		return
	}
	state, ok := decodeLegacyDomainState(ctx, req.State.GetAttribute, "fastly_domain", &resp.Diagnostics)
	if !ok {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// MoveState moves fastly_domain_v1 state from the legacy SDKv2 provider.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyDomainMoveStateSchema()
	return []resource.StateMover{{
		SourceSchema: &sourceSchema,
		StateMover:   moveLegacyDomainV1State,
	}}
}

func moveLegacyDomainV1State(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if req.SourceProviderAddress != legacystate.ProviderAddress || req.SourceTypeName != legacyDomainV1TypeName {
		return
	}

	if req.SourceSchemaVersion != legacyDomainSchemaV0 {
		resp.Diagnostics.AddError(
			"Unsupported legacy Fastly domain state",
			fmt.Sprintf("fastly_domain can only move state from fastly_domain_v1 schema version 0; received schema version %d.", req.SourceSchemaVersion),
		)
		return
	}
	if req.SourceState == nil {
		resp.Diagnostics.AddError(
			"Unable to decode legacy Fastly domain state",
			"Terraform Plugin Framework did not provide decoded source state for fastly_domain_v1 schema version 0.",
		)
		return
	}
	state, ok := decodeLegacyDomainState(ctx, req.SourceState.GetAttribute, legacyDomainV1TypeName, &resp.Diagnostics)
	if !ok {
		return
	}
	resp.Diagnostics.Append(resp.TargetState.Set(ctx, state)...)
}

type getAttributeFunc func(ctx context.Context, p path.Path, target any) diag.Diagnostics

// decodeLegacyDomainState validates legacy state and returns the target model.
// An unset description is persisted as "" by SDKv2 but the target plans null for
// an omitted description, so "" becomes null to avoid a spurious update. An
// explicit `description = ""` then converges with one empty-description update.
func decodeLegacyDomainState(ctx context.Context, get getAttributeFunc, typeName string, diags *diag.Diagnostics) (Model, bool) {
	var id, domainID, fqdn, description, serviceID types.String
	diags.Append(get(ctx, path.Root("id"), &id)...)
	diags.Append(get(ctx, path.Root("domain_id"), &domainID)...)
	diags.Append(get(ctx, path.Root("fqdn"), &fqdn)...)
	diags.Append(get(ctx, path.Root("description"), &description)...)
	diags.Append(get(ctx, path.Root("service_id"), &serviceID)...)
	if diags.HasError() {
		return Model{}, false
	}

	if !legacystate.HasValue(id) {
		diags.AddError("Unable to migrate legacy Fastly domain state", fmt.Sprintf("The %s state does not contain a domain ID.", typeName))
		return Model{}, false
	}
	if !legacystate.HasValue(fqdn) {
		diags.AddError("Unable to migrate legacy Fastly domain state", fmt.Sprintf("The %s state does not contain an fqdn.", typeName))
		return Model{}, false
	}
	// The legacy provider always wrote both attributes from the same API value.
	// A disagreement means the state was edited or is not a domain state.
	if legacystate.HasValue(domainID) && domainID.ValueString() != id.ValueString() {
		diags.AddError(
			"Unable to migrate legacy Fastly domain state",
			fmt.Sprintf("The %s state has different id and domain_id values, so the domain identity is ambiguous.", typeName),
		)
		return Model{}, false
	}
	if !description.IsNull() && description.ValueString() == "" {
		description = types.StringNull()
	}
	return Model{ID: id, FQDN: fqdn, Description: description, ServiceID: serviceID}, true
}
