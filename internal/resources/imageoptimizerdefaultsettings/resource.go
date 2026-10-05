package imageoptimizerdefaultsettings

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/importutil"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

const resourceTypeName = "fastly_service_image_optimizer_default_settings"

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
	_ resource.ResourceWithIdentity    = &Resource{}
)

type Resource struct {
	providerData *fastlyclient.Data
}

func NewResource() resource.Resource {
	return &Resource{}
}

type Model struct {
	NestedModel
	ID      types.String `tfsdk:"id"`
	Service types.String `tfsdk:"service_id"`
	Version types.Int64  `tfsdk:"version"`
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_image_optimizer_default_settings"
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = resourceidentity.ServiceScopedVersionedSchema()
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fastly service Image Optimizer default settings resource. Writes directly to the specified writable service version. CDN services only. " +
			"Image Optimizer must already be enabled on the service (e.g. via `fastly_service_product_image_optimizer`).",
		Attributes: ResourceAttributes(),
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	r.providerData = data
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := plan.Service.ValueString()
	version := int(plan.Version.ValueInt64())

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, resourceTypeName, service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	tflog.Debug(ctx, "Creating Fastly Image Optimizer default settings", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, serviceID, version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := Reconcile(ctx, r.providerData.Client, serviceID, version, nil, []NestedModel{plan.NestedModel}); err != nil {
		resp.Diagnostics.AddError("Error creating explicit Image Optimizer default settings", err.Error())
		return
	}

	flattenModel(&plan, plan.NestedModel, serviceID, version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.ServiceScopedVersioned(plan.Service))...)
	}
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.Service.ValueString()
	version := int(state.Version.ValueInt64())

	tflog.Debug(ctx, "Reading Fastly Image Optimizer default settings from API", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	remote, err := r.providerData.Client.GetImageOptimizerDefaultSettings(ctx, &fastly.GetImageOptimizerDefaultSettingsInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading explicit Image Optimizer default settings", err.Error())
		return
	}
	// The client maps a 404 to (nil, nil), which happens when Image Optimizer has been disabled
	// on the service or the version no longer exists.
	if remote == nil {
		tflog.Warn(ctx, "Image Optimizer default settings not found, removing from state", map[string]any{
			"service_id": serviceID,
			"version":    version,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	flattenModel(&state, FlattenToNestedModel(remote), serviceID, version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.ServiceScopedVersioned(state.Service))...)
	}
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	var state Model

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := plan.Service.ValueString()
	version := int(plan.Version.ValueInt64())

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, resourceTypeName, service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, serviceID, version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly Image Optimizer default settings", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	if err := Reconcile(ctx, r.providerData.Client, serviceID, version, []NestedModel{state.NestedModel}, []NestedModel{plan.NestedModel}); err != nil {
		resp.Diagnostics.AddError("Error updating explicit Image Optimizer default settings", err.Error())
		return
	}

	flattenModel(&plan, plan.NestedModel, serviceID, version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.ServiceScopedVersioned(plan.Service))...)
	}
}

// Delete resets the settings back to their API defaults rather than deleting anything remotely,
// since they always exist server-side while Image Optimizer is enabled. This mirrors
// NestedBlockSchema's behavior for removing the block from an _auto resource.
func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.Service.ValueString()
	version := int(state.Version.ValueInt64())

	tflog.Debug(ctx, "Deleting Fastly Image Optimizer default settings", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, resourceTypeName, service.TypeVCL); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	notFound, diags := r.providerData.VersionChecker.EnsureMutableForDelete(ctx, serviceID, version)
	resp.Diagnostics.Append(diags...)
	if notFound || resp.Diagnostics.HasError() {
		return
	}

	if err := Reconcile(ctx, r.providerData.Client, serviceID, version, []NestedModel{state.NestedModel}, nil); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error resetting Image Optimizer default settings to defaults", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resourceidentity.ImportServiceScopedVersioned(ctx, r.providerData.Client, req, resp)
		return
	}

	serviceID, version, err := importutil.ParseServiceVersionID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected import ID in format: service_id/version\n"+
				"For example: service123/3\n\n"+
				"Error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Importing Image Optimizer default settings", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	remote, err := r.providerData.Client.GetImageOptimizerDefaultSettings(ctx, &fastly.GetImageOptimizerDefaultSettingsInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error importing Image Optimizer default settings", err.Error())
		return
	}
	if remote == nil {
		resp.Diagnostics.AddError(
			"Error importing Image Optimizer default settings",
			fmt.Sprintf("No Image Optimizer default settings found for service %q version %d. Image Optimizer must be enabled on the service.", serviceID, version),
		)
		return
	}

	var state Model
	flattenModel(&state, FlattenToNestedModel(remote), serviceID, version)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.ServiceScopedVersioned(state.Service))...)
	}
}
