package ratelimiter

import (
	"context"
	"strconv"

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

	"github.com/fastly/go-fastly/v17/fastly"
)

var (
	_ resource.Resource                   = &Resource{}
	_ resource.ResourceWithImportState    = &Resource{}
	_ resource.ResourceWithIdentity       = &Resource{}
	_ resource.ResourceWithValidateConfig = &Resource{}
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
	resp.TypeName = req.ProviderTypeName + "_service_ratelimiter"
}

// IdentityModel deliberately excludes version: a resource identity can't
// safely contain a mutable field, since Terraform treats an identity change
// as a different resource and forces a replace.
type IdentityModel struct {
	ServiceID types.String `tfsdk:"service_id"`
	Name      types.String `tfsdk:"name"`
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"service_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Fastly service ID.",
			},
			"name": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Rate limiter name.",
			},
		},
	}
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fastly service rate limiter resource. Writes directly to the specified writable service version.",
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
		resp.Diagnostics.AddError("Invalid rate limiter configuration", err.Error())
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, plan.Service.ValueString(), "fastly_service_ratelimiter", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	tflog.Debug(ctx, "Creating Fastly service rate limiter", map[string]any{
		"service_id": plan.Service.ValueString(),
		"version":    plan.Version.ValueInt64(),
		"name":       service.StringValue(plan.Name),
	})

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, plan.Service.ValueString(), int(plan.Version.ValueInt64()))...)
	if resp.Diagnostics.HasError() {
		return
	}

	e, err := ops{}.Create(ctx, r.providerData.Client, plan.Service.ValueString(), int(plan.Version.ValueInt64()), plan.NestedModel)
	if err != nil {
		resp.Diagnostics.AddError("Error creating explicit service rate limiter", err.Error())
		return
	}

	flatten(ctx, e, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, &IdentityModel{
			ServiceID: plan.Service,
			Name:      plan.Name,
		})...)
	}
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Fastly service rate limiter from API", map[string]any{
		"service_id": state.Service.ValueString(),
		"version":    state.Version.ValueInt64(),
		"name":       state.Name.ValueString(),
	})

	e, err := findByName(ctx, r.providerData.Client, state.Service.ValueString(), int(state.Version.ValueInt64()), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading explicit service rate limiter", err.Error())
		return
	}
	if e == nil {
		tflog.Warn(ctx, "Service rate limiter not found, removing from state", map[string]any{
			"service_id": state.Service.ValueString(),
			"name":       state.Name.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	flatten(ctx, e, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, &IdentityModel{
			ServiceID: state.Service,
			Name:      state.Name,
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

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, plan.Service.ValueString(), "fastly_service_ratelimiter", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, plan.Service.ValueString(), int(plan.Version.ValueInt64()))...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly service rate limiter", map[string]any{
		"service_id": plan.Service.ValueString(),
		"version":    plan.Version.ValueInt64(),
		"name":       service.StringValue(plan.Name),
	})

	o := &ops{}
	if _, err := o.List(ctx, r.providerData.Client, plan.Service.ValueString(), int(plan.Version.ValueInt64())); err != nil {
		resp.Diagnostics.AddError("Error updating explicit service rate limiter", err.Error())
		return
	}

	e, err := o.Update(ctx, r.providerData.Client, plan.Service.ValueString(), int(plan.Version.ValueInt64()), plan.NestedModel)
	if err != nil {
		resp.Diagnostics.AddError("Error updating explicit service rate limiter", err.Error())
		return
	}

	flatten(ctx, e, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, &IdentityModel{
			ServiceID: plan.Service,
			Name:      plan.Name,
		})...)
	}
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Fastly service rate limiter", map[string]any{
		"service_id": state.Service.ValueString(),
		"version":    state.Version.ValueInt64(),
		"name":       state.Name.ValueString(),
	})

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, state.Service.ValueString(), "fastly_service_ratelimiter", service.TypeVCL); err != nil {
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

	o := &ops{}
	if _, err := o.List(ctx, r.providerData.Client, state.Service.ValueString(), int(state.Version.ValueInt64())); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting explicit service rate limiter", err.Error())
		return
	}

	if err := o.Delete(ctx, r.providerData.Client, state.Service.ValueString(), int(state.Version.ValueInt64()), state.Name.ValueString()); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting explicit service rate limiter", err.Error())
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
// (service_id/version/name). It doesn't set identity: a plain-ID import
// leaves the framework to derive it from the resulting state on the next
// Read, same as any other resource.
func (r *Resource) importByCompositeID(ctx context.Context, id string, resp *resource.ImportStateResponse) {
	serviceID, version, name, err := importutil.ParseCompositeID(id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected import ID in format: service_id/version/name\n"+
				"For example: service123/3/my-rate-limiter\n\n"+
				"Error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Importing rate limiter", map[string]any{
		"service_id": serviceID,
		"version":    version,
		"name":       name,
	})

	e, err := findByName(ctx, r.providerData.Client, serviceID, version, name)
	if err != nil {
		resp.Diagnostics.AddError("Error importing rate limiter", err.Error())
		return
	}
	if e == nil {
		resp.Diagnostics.AddError("Error importing rate limiter", "no rate limiter named \""+name+"\" was found in service "+serviceID+" version "+strconv.Itoa(version))
		return
	}

	var state Model
	flatten(ctx, e, &state)

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
	name := identity.Name.ValueString()

	version, _, err := service.SelectReadVersion(ctx, r.providerData.Client, serviceID)
	if err != nil {
		resp.Diagnostics.AddError("Error selecting service version for import", err.Error())
		return
	}

	tflog.Debug(ctx, "Importing rate limiter", map[string]any{
		"service_id": serviceID,
		"version":    version,
		"name":       name,
	})

	e, err := findByName(ctx, r.providerData.Client, serviceID, version, name)
	if err != nil {
		resp.Diagnostics.AddError("Error importing rate limiter", err.Error())
		return
	}
	if e == nil {
		resp.Diagnostics.AddError("Error importing rate limiter", "no rate limiter named \""+name+"\" was found in service "+serviceID+" version "+strconv.Itoa(version))
		return
	}

	var state Model
	flatten(ctx, e, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.Identity.Set(ctx, &IdentityModel{
		ServiceID: types.StringValue(serviceID),
		Name:      types.StringValue(name),
	})...)
}

// findByName returns the rate limiter matching name at the given service version, or nil if none
// exists. The Fastly API only supports fetching a rate limiter by its opaque, API-assigned ID -
// never by service/version/name - so lookups by name always go through ListERLs.
func findByName(ctx context.Context, client *fastly.Client, serviceID string, version int, name string) (*fastly.ERL, error) {
	erls, err := client.ListERLs(ctx, &fastly.ListERLsInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
	if err != nil {
		return nil, err
	}

	for _, e := range erls {
		if fastly.ToValue(e.Name) == name {
			return e, nil
		}
	}

	return nil, nil
}
