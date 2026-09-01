package tlsactivation

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
	resp.TypeName = req.ProviderTypeName + "_tls_activation"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Enables TLS on a domain using a custom certificate. TLS activations are versionless and independent of any service-version lifecycle.",
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

	certificateID := service.StringValue(plan.CertificateID)
	if certificateID == "" {
		resp.Diagnostics.AddError(
			"Missing certificate_id",
			"certificate_id is empty: the certificate for the referenced TLS subscription has not been issued yet "+
				"(certificates are issued asynchronously after domain validation). "+
				"Reference `fastly_tls_subscription_validation.<name>.certificate_id` instead of "+
				"`fastly_tls_subscription.<name>.certificate_id` so the activation waits for issuance within a single apply.",
		)
		return
	}

	tflog.Debug(ctx, "Creating Fastly TLS activation", map[string]any{
		"certificate_id": certificateID,
		"domain":         service.StringValue(plan.Domain),
	})

	activation, err := r.client.CreateTLSActivation(ctx, buildCreateInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating TLS activation", err.Error())
		return
	}

	newState := flattenToModel(activation)

	// mutual_authentication_id can only be set via a follow-up PATCH: https://github.com/fastly/terraform-provider-fastly/issues/873
	if mtlsID := service.StringValue(plan.MutualAuthenticationID); mtlsID != "" {
		updated, err := r.client.UpdateTLSActivation(ctx, buildUpdateInput(newState.ID.ValueString(), plan))
		if err != nil {
			// Record the activation created above so a retry updates it instead of creating a duplicate.
			resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
			resp.Diagnostics.AddError("Error setting mutual_authentication_id on TLS activation", err.Error())
			return
		}
		newState = flattenToModel(updated)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly TLS activation", map[string]any{"id": id})

	activation, err := r.client.GetTLSActivation(ctx, &fastly.GetTLSActivationInput{ID: id})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "TLS activation not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading TLS activation", err.Error())
		return
	}

	newState := flattenToModel(activation)
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
	tflog.Debug(ctx, "Updating Fastly TLS activation", map[string]any{"id": id})

	activation, err := r.client.UpdateTLSActivation(ctx, buildUpdateInput(id, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating TLS activation", err.Error())
		return
	}

	newState := flattenToModel(activation)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly TLS activation", map[string]any{"id": id})

	err := r.client.DeleteTLSActivation(ctx, &fastly.DeleteTLSActivationInput{ID: id})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting TLS activation", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
