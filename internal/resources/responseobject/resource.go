package responseobject

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

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

const resourceTypeName = "fastly_service_response_object"

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
	resp.TypeName = req.ProviderTypeName + "_service_response_object"
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = resourceidentity.NamedVersionedSchema()
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fastly service response object resource. Writes directly to the specified writable service version.",
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

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, resourceTypeName, service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	tflog.Debug(ctx, "Creating Fastly service response object", map[string]any{
		"service_id": serviceID,
		"version":    version,
		"name":       service.StringValue(plan.Name),
	})

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, serviceID, version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ro, err := ops{}.Create(ctx, r.providerData.Client, serviceID, version, plan.NestedModel)
	if err != nil {
		resp.Diagnostics.AddError("Error creating explicit service response object", err.Error())
		return
	}

	flatten(ctx, ro, &plan)
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

	serviceID := state.Service.ValueString()
	version := int(state.Version.ValueInt64())
	name := state.Name.ValueString()

	tflog.Debug(ctx, "Reading Fastly service response object from API", map[string]any{
		"service_id": serviceID,
		"version":    version,
		"name":       name,
	})

	ro, err := r.providerData.Client.GetResponseObject(ctx, &fastly.GetResponseObjectInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Service response object not found, removing from state", map[string]any{
				"service_id": serviceID,
				"name":       name,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading explicit service response object", err.Error())
		return
	}

	flatten(ctx, ro, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.NamedVersioned(state.Service, state.Name))...)
	}
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
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

	resp.Diagnostics.Append(r.providerData.VersionChecker.EnsureMutable(ctx, serviceID, version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly service response object", map[string]any{
		"service_id": serviceID,
		"version":    version,
		"name":       service.StringValue(plan.Name),
	})

	ro, err := ops{}.Update(ctx, r.providerData.Client, serviceID, version, plan.NestedModel)
	if err != nil {
		resp.Diagnostics.AddError("Error updating explicit service response object", err.Error())
		return
	}

	flatten(ctx, ro, &plan)
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

	serviceID := state.Service.ValueString()
	version := int(state.Version.ValueInt64())
	name := state.Name.ValueString()

	tflog.Debug(ctx, "Deleting Fastly service response object", map[string]any{
		"service_id": serviceID,
		"version":    version,
		"name":       name,
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

	err := r.providerData.Client.DeleteResponseObject(ctx, &fastly.DeleteResponseObjectInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting explicit service response object", err.Error())
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
				"For example: service123/3/my-response-object\n\n"+
				"Error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Importing response object", map[string]any{
		"service_id": serviceID,
		"version":    version,
		"name":       name,
	})

	ro, err := r.providerData.Client.GetResponseObject(ctx, &fastly.GetResponseObjectInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error importing response object", err.Error())
		return
	}

	var state Model
	state.Service = types.StringValue(serviceID)
	state.Version = types.Int64Value(int64(version))
	flatten(ctx, ro, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if !resp.Diagnostics.HasError() && resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, resourceidentity.NamedVersioned(state.Service, state.Name))...)
	}
}
