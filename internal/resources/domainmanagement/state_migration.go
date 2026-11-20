package domainmanagement

import (
	"context"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/legacystate"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	legacyDomainV1TypeName = "fastly_domain_v1"
	legacyDomainSchemaV0   = int64(0)
)

var _ resource.ResourceWithMoveState = &Resource{}

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

// MoveState moves fastly_domain_v1 state from the legacy SDKv2 provider.
//
// fastly_domain keeps its name, so it needs no mover: Terraform decodes the
// persisted state directly and drops the legacy-only domain_id attribute.
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

	var id, domainID, fqdn, description, serviceID types.String
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("domain_id"), &domainID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("fqdn"), &fqdn)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("description"), &description)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("service_id"), &serviceID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !legacystate.HasValue(id) {
		resp.Diagnostics.AddError("Unable to migrate legacy Fastly domain state", "The fastly_domain_v1 state does not contain a domain ID.")
		return
	}
	if !legacystate.HasValue(fqdn) {
		resp.Diagnostics.AddError("Unable to migrate legacy Fastly domain state", "The fastly_domain_v1 state does not contain an fqdn.")
		return
	}
	// The legacy provider always wrote both attributes from the same API value.
	// A disagreement means the state was edited or is not a domain state.
	if legacystate.HasValue(domainID) && domainID.ValueString() != id.ValueString() {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly domain state",
			"The fastly_domain_v1 state has different id and domain_id values, so the domain identity is ambiguous.",
		)
		return
	}

	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("fqdn"), fqdn)...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("description"), description)...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("service_id"), serviceID)...)
}
