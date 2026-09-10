package userserviceauthorization

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_service_authorization"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a user permissions on a service. User service authorizations are versionless and independent of any service-version lifecycle.",
		Attributes:  ResourceAttributes(),
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}
	r.client = data.Client
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly User Service Authorization", map[string]any{
		"service_id": plan.ServiceID.ValueString(),
		"user_id":    plan.UserID.ValueString(),
	})

	sa, err := r.client.CreateServiceAuthorization(ctx, buildCreateInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating User Service Authorization", err.Error())
		return
	}

	newState := flattenToModel(sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly User Service Authorization", map[string]any{"id": id})

	sa, err := r.client.GetServiceAuthorization(ctx, &fastly.GetServiceAuthorizationInput{ID: id})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "User Service Authorization not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading User Service Authorization", err.Error())
		return
	}

	newState := flattenToModel(sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Updating Fastly User Service Authorization", map[string]any{"id": id})

	sa, err := r.client.UpdateServiceAuthorization(ctx, buildUpdateInput(id, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating User Service Authorization", err.Error())
		return
	}

	// Unlike GET/POST, the PATCH response only includes attributes, not the service/user
	// relationships - service_id and user_id are RequiresReplace, so carry them over from the
	// plan instead of reading them off the (nil) response relationships.
	newState := plan
	newState.ID = types.StringValue(sa.ID)
	newState.Permission = types.StringValue(sa.Permission)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly User Service Authorization", map[string]any{"id": id})

	err := r.client.DeleteServiceAuthorization(ctx, &fastly.DeleteServiceAuthorizationInput{ID: id})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting User Service Authorization", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
