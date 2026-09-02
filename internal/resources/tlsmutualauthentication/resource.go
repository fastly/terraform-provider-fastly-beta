package tlsmutualauthentication

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	resp.TypeName = req.ProviderTypeName + "_tls_mutual_authentication"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Enables client-to-server mutual TLS. Mutual authentications are versionless and independent of any service-version lifecycle.",
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

	tflog.Debug(ctx, "Creating Fastly TLS mutual authentication", map[string]any{"name": service.StringValue(plan.Name)})

	mtls, err := r.client.CreateTLSMutualAuthentication(ctx, buildCreateInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating TLS mutual authentication", err.Error())
		return
	}

	activationIDs := setToStringSlice(ctx, plan.ActivationIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, activationID := range activationIDs {
		if err := setActivationMTLS(ctx, r.client, activationID, mtls.ID); err != nil {
			resp.Diagnostics.AddError("Error linking TLS activation to mutual authentication", err.Error())
			return
		}
	}

	newState, err := fetchAndFlatten(ctx, r.client, mtls.ID, service.StringValue(plan.Include))
	if err != nil {
		resp.Diagnostics.AddError("Error reading TLS mutual authentication", err.Error())
		return
	}
	newState.ActivationIDs = plan.ActivationIDs
	newState.CertBundle = plan.CertBundle
	newState.Include = plan.Include

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly TLS mutual authentication", map[string]any{"id": id})

	newState, err := fetchAndFlatten(ctx, r.client, id, service.StringValue(state.Include))
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "TLS mutual authentication not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading TLS mutual authentication", err.Error())
		return
	}
	// cert_bundle is write-only and activation_ids is desired-state only; neither is refreshed by a read.
	newState.ActivationIDs = state.ActivationIDs
	newState.CertBundle = state.CertBundle
	newState.Include = state.Include

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
	tflog.Debug(ctx, "Updating Fastly TLS mutual authentication", map[string]any{"id": id})

	if _, err := r.client.UpdateTLSMutualAuthentication(ctx, buildUpdateInput(id, plan, state)); err != nil {
		resp.Diagnostics.AddError("Error updating TLS mutual authentication", err.Error())
		return
	}

	if !plan.ActivationIDs.Equal(state.ActivationIDs) {
		oldIDs := setToStringSlice(ctx, state.ActivationIDs, &resp.Diagnostics)
		newIDs := setToStringSlice(ctx, plan.ActivationIDs, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		// Unsetting old activations is best-effort so one failure doesn't block the rest.
		for _, activationID := range oldIDs {
			_ = unsetActivationMTLS(ctx, r.client, activationID)
		}

		for _, activationID := range newIDs {
			if err := setActivationMTLS(ctx, r.client, activationID, id); err != nil {
				resp.Diagnostics.AddError("Error linking TLS activation to mutual authentication", err.Error())
				return
			}
		}
	}

	newState, err := fetchAndFlatten(ctx, r.client, id, service.StringValue(plan.Include))
	if err != nil {
		resp.Diagnostics.AddError("Error reading TLS mutual authentication", err.Error())
		return
	}
	newState.ActivationIDs = plan.ActivationIDs
	newState.CertBundle = plan.CertBundle
	newState.Include = plan.Include

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly TLS mutual authentication", map[string]any{"id": id})

	// Can't delete mTLS with active domains: unset it from each activation first.
	activationIDs := setToStringSlice(ctx, state.ActivationIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, activationID := range activationIDs {
		if err := unsetActivationMTLS(ctx, r.client, activationID); err != nil {
			resp.Diagnostics.AddError("Error unlinking TLS activation from mutual authentication", err.Error())
			return
		}
	}

	err := r.client.DeleteTLSMutualAuthentication(ctx, &fastly.DeleteTLSMutualAuthenticationInput{ID: id})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting TLS mutual authentication", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func fetchAndFlatten(ctx context.Context, client *fastly.Client, id, include string) (Model, error) {
	mtls, err := client.GetTLSMutualAuthentication(ctx, &fastly.GetTLSMutualAuthenticationInput{
		ID:      id,
		Include: include,
	})
	if err != nil {
		return Model{}, err
	}
	return flattenToModel(mtls), nil
}
