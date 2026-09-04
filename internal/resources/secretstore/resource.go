package secretstore

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

type Model struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secretstore"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Secret Store, a persistent, globally distributed store for secrets that is accessible to Compute services during request processing.",
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

	tflog.Debug(ctx, "Creating Fastly Secret Store", map[string]any{
		"name": plan.Name.ValueString(),
	})

	store, err := r.client.CreateSecretStore(ctx, &fastly.CreateSecretStoreInput{
		Name: plan.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Secret Store", err.Error())
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

	tflog.Debug(ctx, "Reading Fastly Secret Store", map[string]any{
		"id": storeID,
	})

	store, err := r.client.GetSecretStore(ctx, &fastly.GetSecretStoreInput{
		StoreID: storeID,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Secret Store not found, removing from state", map[string]any{
				"id": storeID,
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading Secret Store", err.Error())
		return
	}

	flatten(&state, store)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update never actually runs against a changed name, since name requires replacement and
// there is no other mutable attribute; it only needs to persist the plan back into state.
func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	storeID := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly Secret Store", map[string]any{
		"id": storeID,
	})

	// Deleting a Secret Store also deletes all secrets it contains,
	// so there is no need to empty it first.
	err := r.client.DeleteSecretStore(ctx, &fastly.DeleteSecretStoreInput{
		StoreID: storeID,
	})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Secret Store", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flatten(m *Model, store *fastly.SecretStore) {
	if store == nil {
		return
	}

	m.ID = types.StringValue(store.StoreID)
	m.Name = types.StringValue(store.Name)
}
