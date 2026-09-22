package settings

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/importutil"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

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

// IdentityModel deliberately excludes version: a resource identity can't
// safely contain a mutable field, since Terraform treats an identity change
// as a different resource and forces a replace. Settings has no name
// component, so service_id alone identifies it.
type IdentityModel struct {
	ServiceID types.String `tfsdk:"service_id"`
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_settings"
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"service_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Fastly service ID.",
			},
		},
	}
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fastly service general settings resource. Writes directly to the specified writable service version. CDN services only.",
		Attributes:  ResourceAttributes(),
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

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, "fastly_service_settings", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	tflog.Debug(ctx, "Creating Fastly service settings", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, serviceID, version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := Reconcile(ctx, r.providerData.Client, serviceID, version, nil, []NestedModel{plan.NestedModel})
	if err != nil {
		resp.Diagnostics.AddError("Error creating explicit service settings", err.Error())
		return
	}

	flattenModel(&plan, result[0], serviceID, version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, &IdentityModel{
			ServiceID: plan.Service,
		})...)
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

	tflog.Debug(ctx, "Reading Fastly service settings from API", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	m, err := readCurrent(ctx, r.providerData.Client, serviceID, version)
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Service settings not found, removing from state", map[string]any{
				"service_id": serviceID,
				"version":    version,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading explicit service settings", err.Error())
		return
	}

	flattenModel(&state, m, serviceID, version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, &IdentityModel{
			ServiceID: state.Service,
		})...)
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

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, "fastly_service_settings", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, serviceID, version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly service settings", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	result, err := Reconcile(ctx, r.providerData.Client, serviceID, version, []NestedModel{state.NestedModel}, []NestedModel{plan.NestedModel})
	if err != nil {
		resp.Diagnostics.AddError("Error updating explicit service settings", err.Error())
		return
	}

	flattenModel(&plan, result[0], serviceID, version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	if resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, &IdentityModel{
			ServiceID: plan.Service,
		})...)
	}
}

// Delete resets the general settings back to their API defaults rather than actually deleting
// anything remotely - unlike most resources, settings always exist server-side for every service
// version, so there is nothing to delete, only reset. This mirrors NestedBlockSchema's documented
// behavior for removing the settings block from an _auto resource.
func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.Service.ValueString()
	version := int(state.Version.ValueInt64())

	tflog.Debug(ctx, "Deleting Fastly service settings", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, "fastly_service_settings", service.TypeVCL); err != nil {
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

	if _, err := Reconcile(ctx, r.providerData.Client, serviceID, version, []NestedModel{state.NestedModel}, nil); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error resetting service settings to defaults", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "" {
		r.importByCompositeID(ctx, req.ID, resp)
		return
	}
	r.importByIdentity(ctx, req, resp)
}

// importByCompositeID handles the legacy `terraform import` string ID format
// (service_id/version). It doesn't set identity: a plain-ID import leaves
// the framework to derive it from the resulting state on the next Read,
// same as any other resource.
func (r *Resource) importByCompositeID(ctx context.Context, id string, resp *resource.ImportStateResponse) {
	serviceID, version, err := importutil.ParseServiceVersionID(id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected import ID in format: service_id/version\n"+
				"For example: service123/3\n\n"+
				"Error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Importing service settings", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	m, err := readCurrent(ctx, r.providerData.Client, serviceID, version)
	if err != nil {
		resp.Diagnostics.AddError("Error importing service settings", err.Error())
		return
	}

	var state Model
	flattenModel(&state, m, serviceID, version)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// importByIdentity handles `terraform query`-generated `import { identity =
// {...} }` blocks. Identity carries no version, so the version to read is
// selected the same way query itself picks one: active, else latest.
func (r *Resource) importByIdentity(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var identity IdentityModel
	resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := identity.ServiceID.ValueString()

	version, _, err := service.SelectReadVersion(ctx, r.providerData.Client, serviceID)
	if err != nil {
		resp.Diagnostics.AddError("Error selecting service version for import", err.Error())
		return
	}

	tflog.Debug(ctx, "Importing service settings by identity", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	m, err := readCurrent(ctx, r.providerData.Client, serviceID, version)
	if err != nil {
		resp.Diagnostics.AddError("Error importing service settings", err.Error())
		return
	}

	var state Model
	flattenModel(&state, m, serviceID, version)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.Identity.Set(ctx, &IdentityModel{
		ServiceID: types.StringValue(serviceID),
	})...)
}
