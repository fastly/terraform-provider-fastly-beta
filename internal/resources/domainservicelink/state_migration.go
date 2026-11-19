package domainservicelink

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	legacyFastlyProviderAddress = "registry.terraform.io/fastly/fastly"
	legacyLinkV1TypeName        = "fastly_domain_v1_service_link"
	legacyLinkSchemaV0          = int64(0)
)

var _ resource.ResourceWithMoveState = &Resource{}

// legacyLinkMoveStateSchema is decode-only. The SDKv2 fastly_domain_service_link
// and its deprecated fastly_domain_v1_service_link alias share one version 0 schema.
func legacyLinkMoveStateSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"domain_id":  schema.StringAttribute{Computed: true},
			"id":         schema.StringAttribute{Computed: true},
			"service_id": schema.StringAttribute{Computed: true},
		},
	}
}

// MoveState moves fastly_domain_v1_service_link state from the legacy SDKv2
// provider. fastly_domain_service_link keeps its name and schema, so it needs no mover.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyLinkMoveStateSchema()
	return []resource.StateMover{{
		SourceSchema: &sourceSchema,
		StateMover:   moveLegacyLinkV1State,
	}}
}

func moveLegacyLinkV1State(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if req.SourceProviderAddress != legacyFastlyProviderAddress || req.SourceTypeName != legacyLinkV1TypeName {
		return
	}

	if req.SourceSchemaVersion != legacyLinkSchemaV0 {
		resp.Diagnostics.AddError(
			"Unsupported legacy Fastly domain service link state",
			fmt.Sprintf("fastly_domain_service_link can only move state from fastly_domain_v1_service_link schema version 0; received schema version %d.", req.SourceSchemaVersion),
		)
		return
	}
	if req.SourceState == nil {
		resp.Diagnostics.AddError(
			"Unable to decode legacy Fastly domain service link state",
			"Terraform Plugin Framework did not provide decoded source state for fastly_domain_v1_service_link schema version 0.",
		)
		return
	}

	var id, domainID, serviceID types.String
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("domain_id"), &domainID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("service_id"), &serviceID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !hasValue(domainID) || !hasValue(serviceID) {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly domain service link state",
			"The fastly_domain_v1_service_link state must contain both a domain_id and a service_id.",
		)
		return
	}
	if hasValue(id) && id.ValueString() != domainID.ValueString() {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly domain service link state",
			"The fastly_domain_v1_service_link state has different id and domain_id values, so the link identity is ambiguous.",
		)
		return
	}

	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("id"), domainID)...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("domain_id"), domainID)...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("service_id"), serviceID)...)
}

func hasValue(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueString() != ""
}
