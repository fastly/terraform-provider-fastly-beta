package domainmanagement

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
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
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Fastly Domain. Domains are versionless and independent of any service-version lifecycle.",
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

	tflog.Debug(ctx, "Creating Fastly Domain", map[string]any{
		"fqdn": plan.FQDN.ValueString(),
	})

	data, err := domains.Create(ctx, r.client, BuildCreateInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating Domain", err.Error())
		return
	}

	newState := FlattenToModel(data)
	newState.Description = ReconcileDescription(newState.Description, plan.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly Domain", map[string]any{
		"id": domainID,
	})

	data, err := domains.Get(ctx, r.client, &domains.GetInput{DomainID: &domainID})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Domain not found, removing from state", map[string]any{"id": domainID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Domain", err.Error())
		return
	}

	newState := FlattenToModel(data)
	newState.Description = ReconcileDescription(newState.Description, state.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := plan.ID.ValueString()
	tflog.Debug(ctx, "Updating Fastly Domain", map[string]any{
		"id": domainID,
	})

	data, err := domains.Update(ctx, r.client, BuildUpdateInput(domainID, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating Domain", err.Error())
		return
	}

	newState := FlattenToModel(data)
	newState.Description = ReconcileDescription(newState.Description, plan.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly Domain", map[string]any{
		"id": domainID,
	})

	if err := domains.Delete(ctx, r.client, &domains.DeleteInput{DomainID: &domainID}); err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Domain", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
