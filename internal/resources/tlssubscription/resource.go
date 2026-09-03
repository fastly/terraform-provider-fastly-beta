package tlssubscription

import (
	"context"
	"fmt"
	"slices"

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
	_ resource.Resource                   = &Resource{}
	_ resource.ResourceWithConfigure      = &Resource{}
	_ resource.ResourceWithImportState    = &Resource{}
	_ resource.ResourceWithValidateConfig = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_subscription"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Enables the [TLS Subscription](https://www.fastly.com/documentation/reference/api/tls/subscriptions/) " +
			"flow, allowing users to add and manage TLS certificates through a Fastly-managed CA. TLS subscriptions " +
			"are versionless and independent of any service-version lifecycle.",
		Attributes: ResourceAttributes(),
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

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.CommonName.IsNull() || config.CommonName.IsUnknown() || config.Domains.IsUnknown() {
		return
	}
	commonName := config.CommonName.ValueString()
	if commonName == "" {
		return
	}

	var domains []string
	resp.Diagnostics.Append(config.Domains.ElementsAs(ctx, &domains, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !slices.Contains(domains, commonName) {
		resp.Diagnostics.AddAttributeError(
			path.Root("common_name"),
			"Invalid common_name",
			fmt.Sprintf("domain specified as common_name (%s) must also be in domains (%v)", commonName, domains),
		)
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly TLS subscription", map[string]any{"certificate_authority": service.StringValue(plan.CertificateAuthority)})

	input, diags := buildCreateInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscription, err := r.client.CreateTLSSubscription(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating TLS subscription", err.Error())
		return
	}

	// Persist state from the create response before the follow-up GET below, so a
	// transient failure to refresh doesn't orphan the subscription Fastly already created.
	initialState, diags := flattenToModel(ctx, r.client, subscription)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	initialState.ForceDestroy = plan.ForceDestroy
	initialState.ForceUpdate = plan.ForceUpdate
	resp.Diagnostics.Append(resp.State.Set(ctx, &initialState)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fetched, err := getSubscription(ctx, r.client, subscription.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading TLS subscription", err.Error())
		return
	}

	newState, diags := flattenToModel(ctx, r.client, fetched)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	newState.ForceDestroy = plan.ForceDestroy
	newState.ForceUpdate = plan.ForceUpdate

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly TLS subscription", map[string]any{"id": id})

	fetched, err := getSubscription(ctx, r.client, id)
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "TLS subscription not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading TLS subscription", err.Error())
		return
	}

	newState, diags := flattenToModel(ctx, r.client, fetched)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	newState.ForceDestroy = state.ForceDestroy
	newState.ForceUpdate = state.ForceUpdate

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
	tflog.Debug(ctx, "Updating Fastly TLS subscription", map[string]any{"id": id})

	// Only call the API when domains/common_name/configuration_id actually changed: other
	// attributes (e.g. force_update) have no effect on the upstream data model, and the API
	// call sends all three regardless of which one triggered it.
	if !plan.Domains.Equal(state.Domains) || !plan.CommonName.Equal(state.CommonName) || !plan.ConfigurationID.Equal(state.ConfigurationID) {
		input, diags := buildUpdateInput(ctx, id, plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		if _, err := r.client.UpdateTLSSubscription(ctx, input); err != nil {
			resp.Diagnostics.AddError("Error updating TLS subscription", err.Error())
			return
		}
	}

	fetched, err := getSubscription(ctx, r.client, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading TLS subscription", err.Error())
		return
	}

	newState, diags := flattenToModel(ctx, r.client, fetched)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	newState.ForceDestroy = plan.ForceDestroy
	newState.ForceUpdate = plan.ForceUpdate

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly TLS subscription", map[string]any{"id": id})

	err := r.client.DeleteTLSSubscription(ctx, &fastly.DeleteTLSSubscriptionInput{
		ID:    id,
		Force: service.BoolValue(state.ForceDestroy),
	})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting TLS subscription", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
