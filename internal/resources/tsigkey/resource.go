package tsigkey

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/dns/v1/tsigkeys"
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
	resp.TypeName = req.ProviderTypeName + "_tsig_key"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Fastly TSIG Key. TSIG keys are versionless and independent of any service-version lifecycle.",
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

	tflog.Debug(ctx, "Creating Fastly TSIG Key", map[string]any{
		"name": plan.Name.ValueString(),
	})

	key, err := tsigkeys.Create(ctx, r.client, BuildCreateInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating TSIG Key", err.Error())
		return
	}

	newState := FlattenToModel(key, plan.Secret)
	newState.Description = ReconcileDescription(newState.Description, plan.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyID := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly TSIG Key", map[string]any{
		"id": keyID,
	})

	key, err := tsigkeys.Get(ctx, r.client, &tsigkeys.GetInput{TSIGKeyID: &keyID})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "TSIG Key not found, removing from state", map[string]any{"id": keyID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading TSIG Key", err.Error())
		return
	}

	newState := FlattenToModel(key, state.Secret)
	newState.Description = ReconcileDescription(newState.Description, state.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyID := state.ID.ValueString()
	tflog.Debug(ctx, "Updating Fastly TSIG Key", map[string]any{
		"id": keyID,
	})

	key, err := tsigkeys.Update(ctx, r.client, BuildUpdateInput(keyID, plan, state))
	if err != nil {
		resp.Diagnostics.AddError("Error updating TSIG Key", err.Error())
		return
	}

	newState := FlattenToModel(key, plan.Secret)
	newState.Description = ReconcileDescription(newState.Description, plan.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyID := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly TSIG Key", map[string]any{
		"id": keyID,
	})

	err := tsigkeys.Delete(ctx, r.client, &tsigkeys.DeleteInput{TSIGKeyID: &keyID})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting TSIG Key", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
