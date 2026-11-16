package servicecdnauto

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
// This proof of concept copies only state that cannot be reconstructed safely
// by Read. The service ID preserves identity; force_destroy and reuse are
// Terraform lifecycle controls which are not returned by the Fastly API. Read
// repopulates service metadata, managed/active versions, and API-readable nested
// service configuration.
//
// Legacy product_enablement state, including the configurable bot_management,
// ddos_protection and ngwaf products, is intentionally not copied to the target
// service state. Each product is instead split into a standalone
// fastly_service_product_* resource whose configuration carries its settings,
// and imported by service ID.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyServiceVCLMoveStateSchema()

	return []resource.StateMover{
		{
			SourceSchema: &sourceSchema,
			StateMover:   moveLegacyServiceVCLState,
		},
	}
}

var legacyServiceVCLSimpleProductNames = []string{
	"api_discovery",
	"brotli_compression",
	"domain_inspector",
	"image_optimizer",
	"log_explorer_insights",
	"origin_inspector",
	"websockets",
}

var legacyServiceVCLConfigurableProductNames = []string{
	"bot_management",
	"ddos_protection",
	"ngwaf",
}

// inspectLegacyProductEnablement reports whether persisted legacy
// product_enablement state can be safely omitted from the target service state.
//
// Every known product, configurable or not, can be split into a standalone
// fastly_service_product_* resource and imported by service ID.
func inspectLegacyProductEnablement(products types.Set) (supported bool, hasManagedProducts bool) {
	if products.IsNull() {
		return true, false
	}
	if products.IsUnknown() {
		return false, false
	}

	elements := products.Elements()
	if len(elements) == 0 {
		return true, false
	}
	if len(elements) != 1 {
		return false, false
	}

	product, ok := elements[0].(types.Object)
	if !ok || product.IsNull() || product.IsUnknown() {
		return false, false
	}

	return inspectLegacyProductAttributes(product.Attributes())
}

func inspectLegacyProductAttributes(attributes map[string]attr.Value) (supported bool, hasManagedProducts bool) {
	for name := range attributes {
		if name == "name" {
			continue
		}
		if !legacyServiceVCLProductNameIn(name, legacyServiceVCLSimpleProductNames) &&
			!legacyServiceVCLProductNameIn(name, legacyServiceVCLConfigurableProductNames) {
			return false, false
		}
	}

	for _, name := range legacyServiceVCLSimpleProductNames {
		value, ok := attributes[name].(types.Bool)
		if !ok || value.IsUnknown() {
			return false, false
		}
		if !value.IsNull() {
			hasManagedProducts = true
		}
	}

	for _, name := range legacyServiceVCLConfigurableProductNames {
		value, ok := attributes[name].(types.List)
		if !ok || value.IsUnknown() {
			return false, false
		}
		if value.IsNull() {
			continue
		}
		switch len(value.Elements()) {
		case 0:
		case 1:
			hasManagedProducts = true
		default:
			return false, false
		}
	}

	return true, hasManagedProducts
}

func legacyServiceVCLProductNameIn(name string, names []string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func moveLegacyServiceVCLState(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if req.SourceProviderAddress != legacyFastlyProviderAddress || req.SourceTypeName != legacyServiceVCLTypeName {
		return
	}

	if req.SourceSchemaVersion != legacyServiceVCLSchemaV1 {
		resp.Diagnostics.AddError(
			"Unsupported legacy Fastly service state",
			fmt.Sprintf("fastly_service_cdn_auto can only move state from fastly_service_vcl schema version 1; received schema version %d. This state mover does not support that persisted state schema and requires a migration path that explicitly supports it.", req.SourceSchemaVersion),
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
	var sourceProductEnablement types.Set

	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &sourceID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("force_destroy"), &sourceForceDestroy)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("reuse"), &sourceReuse)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("product_enablement"), &sourceProductEnablement)...)
	if resp.Diagnostics.HasError() {
		return
	}

	supportedProductEnablement, hasManagedProducts := inspectLegacyProductEnablement(sourceProductEnablement)
	if !supportedProductEnablement {
		resp.Diagnostics.AddError(
			"Unsupported legacy product enablement state",
			"fastly_service_vcl product_enablement state can be migrated only when it contains known products in a well-formed shape. Unknown or malformed product state is blocked.",
		)
		return
	}

	if hasManagedProducts {
		resp.Diagnostics.AddWarning(
			"Legacy product enablement requires standalone resources",
			"The fastly_service_vcl product_enablement block is not copied into fastly_service_cdn_auto state. The migration configuration must represent managed products as standalone fastly_service_product_* resources and import their existing service-scoped state using the Fastly service ID.",
		)
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
