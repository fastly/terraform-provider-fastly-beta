package configstore

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/errors"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}

type Resource struct {
	client *fastly.Client
}

type Model struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_configstore"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Config Store, a versionless container for key-value data that is accessible to Compute services during request processing.",
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

	tflog.Debug(ctx, "Creating Fastly Config Store", map[string]any{
		"name": plan.Name.ValueString(),
	})

	store, err := r.client.CreateConfigStore(ctx, &fastly.CreateConfigStoreInput{
		Name: plan.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Config Store", err.Error())
		return
	}

	flatten(&plan, store)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	storeID := state.ID.ValueString()

	tflog.Debug(ctx, "Reading Fastly Config Store", map[string]any{
		"id": storeID,
	})

	store, err := r.client.GetConfigStore(ctx, &fastly.GetConfigStoreInput{
		StoreID: storeID,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Config Store not found, removing from state", map[string]any{
				"id": storeID,
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading Config Store", err.Error())
		return
	}

	flatten(&state, store)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	var state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	storeID := state.ID.ValueString()

	// Unlike the legacy SDKv2 implementation, the current Config Store API has an
	// update endpoint. Renaming therefore preserves the Fastly store and its ID.
	tflog.Debug(ctx, "Updating Fastly Config Store", map[string]any{
		"id":   storeID,
		"name": plan.Name.ValueString(),
	})

	store, err := r.client.UpdateConfigStore(ctx, &fastly.UpdateConfigStoreInput{
		StoreID: storeID,
		Name:    plan.Name.ValueString(),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error updating Config Store", err.Error())
		return
	}

	flatten(&plan, store)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	storeID := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly Config Store", map[string]any{
		"id": storeID,
	})

	err := r.client.DeleteConfigStore(ctx, &fastly.DeleteConfigStoreInput{
		StoreID: storeID,
	})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Config Store", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flatten(m *Model, store *fastly.ConfigStore) {
	if store == nil {
		return
	}

	m.ID = types.StringValue(store.StoreID)
	m.Name = types.StringValue(store.Name)
}
