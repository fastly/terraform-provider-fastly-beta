package servicecdnauto

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	legacyFastlyProviderAddress = "registry.terraform.io/fastly/fastly"
	legacyServiceVCLTypeName    = "fastly_service_vcl"
	legacyServiceVCLSchemaV1    = int64(1)
)

// MoveState declares the supported legacy SDKv2 -> Framework resource move.
//
// The migration CLI is expected to rewrite fastly_service_vcl configuration to
// fastly_service_cdn_auto and add a Terraform moved block. Terraform Core then
// invokes this mover to translate the persisted source state.
//
// Only values which cannot be reconstructed safely by Read are copied. The
// service ID preserves identity; force_destroy and reuse are Terraform lifecycle
// controls which are not returned by the Fastly API. Read repopulates the service
// metadata, managed version, active version, and nested service configuration.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyServiceVCLMoveStateSchema()

	return []resource.StateMover{
		{
			SourceSchema: &sourceSchema,
			StateMover:   moveLegacyServiceVCLState,
		},
	}
}

func moveLegacyServiceVCLState(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if req.SourceProviderAddress != legacyFastlyProviderAddress || req.SourceTypeName != legacyServiceVCLTypeName {
		return
	}

	if req.SourceSchemaVersion != legacyServiceVCLSchemaV1 {
		resp.Diagnostics.AddError(
			"Unsupported legacy Fastly service state",
			"fastly_service_cdn_auto can only move state from fastly_service_vcl schema version 1. Upgrade the legacy Fastly provider and refresh the state before migrating.",
		)
		return
	}

	if req.SourceState == nil {
		resp.Diagnostics.AddError(
			"Unable to decode legacy Fastly service state",
			"Terraform Plugin Framework did not provide decoded source state for fastly_service_vcl schema version 1. The legacy source schema declared by the state mover does not match the persisted SDKv2 state.",
		)
		return
	}

	var sourceID types.String
	var sourceForceDestroy types.Bool
	var sourceReuse types.Bool

	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &sourceID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("force_destroy"), &sourceForceDestroy)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("reuse"), &sourceReuse)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if sourceID.IsNull() || sourceID.IsUnknown() || sourceID.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly service state",
			"The fastly_service_vcl state does not contain a Fastly service ID.",
		)
		return
	}

	// SDKv2 optional booleans have false zero-value semantics. Normalize an
	// absent persisted value so it matches the defaults on the Framework resource
	// and does not introduce a migration-only plan diff.
	if sourceForceDestroy.IsNull() || sourceForceDestroy.IsUnknown() {
		sourceForceDestroy = types.BoolValue(false)
	}
	if sourceReuse.IsNull() || sourceReuse.IsUnknown() {
		sourceReuse = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("id"), sourceID)...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("force_destroy"), sourceForceDestroy)...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("reuse"), sourceReuse)...)

	// Moving a service has the same first-read requirement as importing one:
	// force-refresh Image Optimizer default settings instead of inferring absence
	// from the intentionally sparse target state.
	if resp.TargetPrivate != nil {
		resp.Diagnostics.Append(resp.TargetPrivate.SetKey(ctx, imageOptimizerImportedPrivateKey, []byte("true"))...)
	}
}
