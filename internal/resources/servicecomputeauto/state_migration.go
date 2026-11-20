package servicecomputeauto

import (
	"context"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/computepackage"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginghttps"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	legacyFastlyProviderAddress  = "registry.terraform.io/fastly/fastly"
	legacyServiceComputeTypeName = "fastly_service_compute"
	legacyServiceComputeSchemaV0 = int64(0)
)

// MoveState declares the supported legacy SDKv2 -> Framework resource move.
//
// The migration CLI is expected to rewrite fastly_service_compute configuration to
// fastly_service_compute_auto and add a Terraform moved block. Terraform Core then
// invokes this mover to translate the persisted source state.
//
// This mover copies only state that cannot be reconstructed safely
// by Read. The service ID preserves identity; force_destroy and reuse are
// Terraform lifecycle controls which are not returned by the Fastly API. Package
// content/filename are also local-only; preserve them so refreshing the package
// hash does not cause Terraform to upload an unchanged package. Read repopulates
// service metadata, managed/active versions, and API-readable configuration.
//
// HTTPS logging retains endpoint names and gzip-level hints, since refresh
// cannot distinguish all API defaults from explicitly configured zero values.
// Credentials and other endpoint attributes are still reconstructed by Read.
//
// Legacy product_enablement state, including the configurable bot_management,
// ddos_protection and ngwaf products, is intentionally not copied to the target
// service state. Each product is instead split into a standalone
// fastly_service_product_* resource whose configuration carries its settings,
// and imported by service ID.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyServiceComputeMoveStateSchema()

	return []resource.StateMover{
		{
			SourceSchema: &sourceSchema,
			StateMover:   moveLegacyServiceComputeState,
		},
	}
}

var legacyServiceComputeSimpleProductNames = []string{
	"api_discovery",
	"domain_inspector",
	"fanout",
	"log_explorer_insights",
	"websockets",
}

var legacyServiceComputeConfigurableProductNames = []string{
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
		if !legacyServiceComputeProductNameIn(name, legacyServiceComputeSimpleProductNames) &&
			!legacyServiceComputeProductNameIn(name, legacyServiceComputeConfigurableProductNames) {
			return false, false
		}
	}

	for _, name := range legacyServiceComputeSimpleProductNames {
		value, ok := attributes[name].(types.Bool)
		if !ok || value.IsUnknown() {
			return false, false
		}
		if !value.IsNull() {
			hasManagedProducts = true
		}
	}

	for _, name := range legacyServiceComputeConfigurableProductNames {
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

func legacyServiceComputeProductNameIn(name string, names []string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func moveLegacyServiceComputeState(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if req.SourceProviderAddress != legacyFastlyProviderAddress || req.SourceTypeName != legacyServiceComputeTypeName {
		return
	}

	if req.SourceSchemaVersion != legacyServiceComputeSchemaV0 {
		resp.Diagnostics.AddError(
			"Unsupported legacy Fastly service state",
			fmt.Sprintf("fastly_service_compute_auto can only move state from fastly_service_compute schema version 0; received schema version %d. This state mover does not support that persisted state schema and requires a migration path that explicitly supports it.", req.SourceSchemaVersion),
		)
		return
	}

	if req.SourceState == nil {
		resp.Diagnostics.AddError(
			"Unable to decode legacy Fastly service state",
			"Terraform Plugin Framework did not provide decoded source state for fastly_service_compute schema version 0. The legacy source schema declared by the state mover does not match the persisted SDKv2 state.",
		)
		return
	}

	var sourceID types.String
	var sourceForceDestroy types.Bool
	var sourceReuse types.Bool
	var sourceActivate types.Bool
	var sourceStage types.Bool
	var sourceProductEnablement types.Set
	var sourcePackage types.List
	var sourceHTTPS types.Set

	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &sourceID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("force_destroy"), &sourceForceDestroy)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("reuse"), &sourceReuse)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("activate"), &sourceActivate)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("stage"), &sourceStage)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("product_enablement"), &sourceProductEnablement)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("package"), &sourcePackage)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("logging_https"), &sourceHTTPS)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// fastly_service_compute_auto always activates and cannot stage. Moving a
	// service that opted out would silently discard that lifecycle behaviour.
	if !sourceActivate.IsNull() && !sourceActivate.IsUnknown() && !sourceActivate.ValueBool() {
		resp.Diagnostics.AddError(
			"Unsupported legacy lifecycle setting: activate = false",
			"fastly_service_compute_auto activates every change and cannot represent a fastly_service_compute with activate = false. Set activate = true and apply with the legacy provider before moving, or keep managing this service with fastly_service_compute.",
		)
	}
	if !sourceStage.IsNull() && !sourceStage.IsUnknown() && sourceStage.ValueBool() {
		resp.Diagnostics.AddError(
			"Unsupported legacy lifecycle setting: stage = true",
			"fastly_service_compute_auto does not support staging and cannot represent a fastly_service_compute with stage = true. Set stage = false and apply with the legacy provider before moving, or keep managing this service with fastly_service_compute.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	supportedProductEnablement, hasManagedProducts := inspectLegacyProductEnablement(sourceProductEnablement)
	if !supportedProductEnablement {
		resp.Diagnostics.AddError(
			"Unsupported legacy product enablement state",
			"fastly_service_compute product_enablement state can be migrated only when it contains known products in a well-formed shape. Unknown or malformed product state is blocked.",
		)
		return
	}

	if hasManagedProducts {
		resp.Diagnostics.AddWarning(
			"Legacy product enablement requires standalone resources",
			"The fastly_service_compute product_enablement block is not copied into fastly_service_compute_auto state. The migration configuration must represent managed products as standalone fastly_service_product_* resources and import their existing service-scoped state using the Fastly service ID.",
		)
	}

	if sourceID.IsNull() || sourceID.IsUnknown() || sourceID.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly service state",
			"The fastly_service_compute state does not contain a Fastly service ID.",
		)
		return
	}

	packages, packageDiags := legacyComputePackageState(ctx, sourcePackage)
	resp.Diagnostics.Append(packageDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	https, httpsDiags := logginghttps.LegacyComputeMoveState(sourceHTTPS)
	resp.Diagnostics.Append(httpsDiags...)
	if resp.Diagnostics.HasError() {
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
	// Keep only the prior endpoint names and gzip hints for the first Read.
	if len(https) != 0 {
		resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("logging_https"), https)...)
	}

	if len(packages) != 0 {
		resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("package"), packages)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Moving a service has the same first-read requirement as importing one:
	// refresh the package hash from the API, including when no package block
	// survived in the source state. Read retains copied local-only inputs.
	if resp.TargetPrivate != nil {
		resp.Diagnostics.Append(markComputePackageForRefresh(ctx, resp.TargetPrivate)...)
	}
}

// legacyComputePackageState preserves local-only package inputs. SDKv2 writes
// the unused content/filename as an empty string, while the Framework target
// represents an omitted attribute as null.
func legacyComputePackageState(ctx context.Context, source types.List) ([]computepackage.Model, diag.Diagnostics) {
	var diags diag.Diagnostics
	if source.IsNull() {
		return nil, diags
	}
	if source.IsUnknown() || len(source.Elements()) > 1 {
		diags.AddError(
			"Unsupported legacy Compute package state",
			"The legacy package state must contain at most one known package object.",
		)
		return nil, diags
	}
	if len(source.Elements()) == 0 {
		return nil, diags
	}

	object, ok := source.Elements()[0].(types.Object)
	if !ok || object.IsNull() || object.IsUnknown() {
		diags.AddError(
			"Unsupported legacy Compute package state",
			"The legacy package state contains an unknown or malformed package object.",
		)
		return nil, diags
	}

	var packages []computepackage.Model
	diags.Append(source.ElementsAs(ctx, &packages, false)...)
	if diags.HasError() {
		return nil, diags
	}
	pkg := &packages[0]
	for _, value := range []*types.String{&pkg.Filename, &pkg.Content, &pkg.SourceCodeHash} {
		if value.IsUnknown() {
			diags.AddError(
				"Unsupported legacy Compute package state",
				"The legacy package state contains an unknown local input or package hash.",
			)
			return nil, diags
		}
		if value.ValueString() == "" {
			*value = types.StringNull()
		}
	}
	if !pkg.Filename.IsNull() && !pkg.Content.IsNull() {
		diags.AddError(
			"Unsupported legacy Compute package state",
			"The legacy package state contains both filename and content; only one can be migrated.",
		)
		return nil, diags
	}

	// Imported legacy packages may have only an API hash. Do not invent
	// filename/content for them, and let the first Read refresh the hash.
	return packages, diags
}

type privateStateSetter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

func markComputePackageForRefresh(ctx context.Context, private privateStateSetter) diag.Diagnostics {
	return private.SetKey(ctx, computePackageImportedPrivateKey, []byte("true"))
}
