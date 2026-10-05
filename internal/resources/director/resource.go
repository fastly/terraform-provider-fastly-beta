package director

import (
	"context"

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

	"github.com/fastly/go-fastly/v17/fastly"
)

var (
	_ resource.Resource                   = &Resource{}
	_ resource.ResourceWithImportState    = &Resource{}
	_ resource.ResourceWithValidateConfig = &Resource{}
	_ resource.ResourceWithIdentity       = &Resource{}
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
	resp.TypeName = req.ProviderTypeName + "_service_director"
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = resourceidentity.NamedVersionedSchema()
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fastly service director resource. Writes directly to the specified writable service version.",
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

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := ValidateConfig([]NestedModel{config.NestedModel}); err != nil {
		resp.Diagnostics.AddError("Invalid director configuration", err.Error())
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, plan.Service.ValueString(), "fastly_service_director", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	tflog.Debug(ctx, "Creating Fastly service director", map[string]any{
		"service_id": plan.Service.ValueString(),
		"version":    plan.Version.ValueInt64(),
		"name":       service.StringValue(plan.Name),
	})

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, plan.Service.ValueString(), int(plan.Version.ValueInt64()))...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := (ops{}).Create(ctx, r.providerData.Client, plan.Service.ValueString(), int(plan.Version.ValueInt64()), plan.NestedModel); err != nil {
		resp.Diagnostics.AddError("Error creating explicit service director", err.Error())
		return
	}

	// CreateDirector's response predates the CreateDirectorBackend calls ops.Create makes
	// afterward, so it never reflects backend associations - re-fetch for the true state.
	d, err := r.providerData.Client.GetDirector(ctx, &fastly.GetDirectorInput{
		ServiceID:      plan.Service.ValueString(),
		ServiceVersion: int(plan.Version.ValueInt64()),
		Name:           service.StringValue(plan.Name),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating explicit service director", err.Error())
		return
	}

	flatten(ctx, d, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.NamedVersioned(plan.Service, plan.Name))...)
	}
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Fastly service director from API", map[string]any{
		"service_id": state.Service.ValueString(),
		"version":    state.Version.ValueInt64(),
		"name":       state.Name.ValueString(),
	})

	d, err := r.providerData.Client.GetDirector(ctx, &fastly.GetDirectorInput{
		ServiceID:      state.Service.ValueString(),
		ServiceVersion: int(state.Version.ValueInt64()),
		Name:           state.Name.ValueString(),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Service director not found, removing from state", map[string]any{
				"service_id": state.Service.ValueString(),
				"name":       state.Name.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading explicit service director", err.Error())
		return
	}

	flatten(ctx, d, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.NamedVersioned(state.Service, state.Name))...)
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

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, plan.Service.ValueString(), "fastly_service_director", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, plan.Service.ValueString(), int(plan.Version.ValueInt64()))...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly service director", map[string]any{
		"service_id": plan.Service.ValueString(),
		"version":    plan.Version.ValueInt64(),
		"name":       service.StringValue(plan.Name),
	})

	o := &ops{}
	if _, err := o.List(ctx, r.providerData.Client, plan.Service.ValueString(), int(plan.Version.ValueInt64())); err != nil {
		resp.Diagnostics.AddError("Error updating explicit service director", err.Error())
		return
	}

	if _, err := o.Update(ctx, r.providerData.Client, plan.Service.ValueString(), int(plan.Version.ValueInt64()), plan.NestedModel); err != nil {
		resp.Diagnostics.AddError("Error updating explicit service director", err.Error())
		return
	}

	// UpdateDirector's response (or the unchanged remote, if metadata didn't change) predates
	// o.Update's backend reconciliation, so it never reflects the diffed backend associations -
	// re-fetch for the true state.
	d, err := r.providerData.Client.GetDirector(ctx, &fastly.GetDirectorInput{
		ServiceID:      plan.Service.ValueString(),
		ServiceVersion: int(plan.Version.ValueInt64()),
		Name:           service.StringValue(plan.Name),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating explicit service director", err.Error())
		return
	}

	flatten(ctx, d, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.NamedVersioned(plan.Service, plan.Name))...)
	}
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Fastly service director", map[string]any{
		"service_id": state.Service.ValueString(),
		"version":    state.Version.ValueInt64(),
		"name":       state.Name.ValueString(),
	})

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, state.Service.ValueString(), "fastly_service_director", service.TypeVCL); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	notFound, diags := r.providerData.VersionChecker.EnsureMutableForDelete(ctx, state.Service.ValueString(), int(state.Version.ValueInt64()))
	resp.Diagnostics.Append(diags...)
	if notFound || resp.Diagnostics.HasError() {
		return
	}

	err := r.providerData.Client.DeleteDirector(ctx, &fastly.DeleteDirectorInput{
		ServiceID:      state.Service.ValueString(),
		ServiceVersion: int(state.Version.ValueInt64()),
		Name:           state.Name.ValueString(),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting explicit service director", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resourceidentity.ImportNamedVersioned(ctx, r.providerData.Client, req, resp)
		return
	}

	serviceID, version, name, err := importutil.ParseCompositeID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected import ID in format: service_id/version/name\n"+
				"For example: service123/3/my-director\n\n"+
				"Error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Importing director", map[string]any{
		"service_id": serviceID,
		"version":    version,
		"name":       name,
	})

	d, err := r.providerData.Client.GetDirector(ctx, &fastly.GetDirectorInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error importing director", err.Error())
		return
	}

	var state Model
	state.Service = types.StringValue(serviceID)
	state.Version = types.Int64Value(int64(version))
	flatten(ctx, d, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.NamedVersioned(state.Service, state.Name))...)
	}
}
