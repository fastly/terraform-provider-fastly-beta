package servicecomputeauto

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	legacyFastlyProviderAddress  = "registry.terraform.io/fastly/fastly"
	legacyServiceComputeTypeName = "fastly_service_compute"
	legacyServiceComputeSchemaV0 = int64(0)
)

var _ resource.ResourceWithMoveState = &Resource{}

// MoveState declares the supported legacy SDKv2 fastly_service_compute move.
//
// The migration CLI rewrites fastly_service_compute configuration to
// fastly_service_compute_auto and adds a Terraform moved block; Terraform Core
// then invokes this mover.
//
// Only state that Read cannot reconstruct is copied: the service ID, and the
// force_destroy/reuse lifecycle controls the Fastly API does not return. The
// source schema is deliberately omitted so the mover decodes just those fields
// from the raw JSON state, independent of the large legacy nested-block type.
// Legacy product_enablement is not copied; each product becomes a standalone
// fastly_service_product_* resource imported by service ID.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	return []resource.StateMover{{StateMover: moveLegacyServiceComputeState}}
}

type legacyServiceComputeState struct {
	ID                *string           `json:"id"`
	ForceDestroy      *bool             `json:"force_destroy"`
	Reuse             *bool             `json:"reuse"`
	ProductEnablement []json.RawMessage `json:"product_enablement"`
}

func moveLegacyServiceComputeState(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if req.SourceProviderAddress != legacyFastlyProviderAddress || req.SourceTypeName != legacyServiceComputeTypeName {
		return
	}

	if req.SourceSchemaVersion != legacyServiceComputeSchemaV0 {
		resp.Diagnostics.AddError(
			"Unsupported legacy Fastly service state",
			fmt.Sprintf("fastly_service_compute_auto can only move state from fastly_service_compute schema version 0; received schema version %d.", req.SourceSchemaVersion),
		)
		return
	}

	if req.SourceRawState == nil || len(req.SourceRawState.JSON) == 0 {
		resp.Diagnostics.AddError(
			"Unable to decode legacy Fastly service state",
			"Terraform did not provide raw source state for fastly_service_compute.",
		)
		return
	}

	var source legacyServiceComputeState
	if err := json.Unmarshal(req.SourceRawState.JSON, &source); err != nil {
		resp.Diagnostics.AddError("Unable to decode legacy Fastly service state", err.Error())
		return
	}

	if source.ID == nil || *source.ID == "" {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly service state",
			"The fastly_service_compute state does not contain a Fastly service ID.",
		)
		return
	}

	if len(source.ProductEnablement) != 0 {
		resp.Diagnostics.AddWarning(
			"Legacy product enablement requires standalone resources",
			"The fastly_service_compute product_enablement block is not copied into fastly_service_compute_auto state. Manage products as standalone fastly_service_product_* resources and import their existing service-scoped state using the Fastly service ID.",
		)
	}

	// SDKv2 optional booleans have false zero-value semantics; normalize absent
	// values to the Framework defaults to avoid a migration-only plan diff.
	forceDestroy := source.ForceDestroy != nil && *source.ForceDestroy
	reuse := source.Reuse != nil && *source.Reuse

	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("id"), types.StringValue(*source.ID))...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("force_destroy"), types.BoolValue(forceDestroy))...)
	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("reuse"), types.BoolValue(reuse))...)

	// Like an import, the sparse target state needs its package hash refreshed
	// on first Read.
	if resp.TargetPrivate != nil {
		resp.Diagnostics.Append(resp.TargetPrivate.SetKey(ctx, computePackageImportedPrivateKey, []byte("true"))...)
	}
}
