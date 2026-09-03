package tlsprivatekey

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	resp.TypeName = req.ProviderTypeName + "_tls_private_key"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a private key for use with Fastly's TLS/SSL support. Private keys are versionless and independent of any service-version lifecycle. The resource is immutable: any change to `name` or `private_key` destroys and recreates it.",
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

	tflog.Debug(ctx, "Creating Fastly TLS private key", map[string]any{"name": plan.Name.ValueString()})

	created, err := r.client.CreatePrivateKey(ctx, buildCreateInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating TLS private key", err.Error())
		return
	}

	// Record the ID immediately so the key isn't orphaned in Fastly with no
	// Terraform state if the follow-up read below fails.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), created.ID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The create response omits fields such as created_at that only the get
	// endpoint populates, so re-fetch before storing state.
	key, err := r.client.GetPrivateKey(ctx, &fastly.GetPrivateKeyInput{ID: created.ID})
	if err != nil {
		resp.Diagnostics.AddError("Error reading newly created TLS private key", err.Error())
		return
	}

	newState := flattenToModel(key, plan.PrivateKey)
	warnIfReplaceRecommended(&resp.Diagnostics, key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly TLS private key", map[string]any{"id": id})

	key, err := r.client.GetPrivateKey(ctx, &fastly.GetPrivateKeyInput{ID: id})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "TLS private key not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading TLS private key", err.Error())
		return
	}

	newState := flattenToModel(key, state.PrivateKey)
	warnIfReplaceRecommended(&resp.Diagnostics, key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Update is unreachable in practice: name and private_key both require
// replacement, so Terraform never plans an in-place update for this resource.
// Implemented only to satisfy resource.Resource.
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

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly TLS private key", map[string]any{"id": id})

	err := r.client.DeletePrivateKey(ctx, &fastly.DeletePrivateKeyInput{ID: id})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting TLS private key", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// warnIfReplaceRecommended surfaces the API's replace recommendation as a non-fatal warning.
func warnIfReplaceRecommended(diags *diag.Diagnostics, key *fastly.PrivateKey) {
	if !key.Replace {
		return
	}
	diags.AddWarning(
		"Fastly recommends replacing this private key",
		fmt.Sprintf("Fastly recommends that private key %q (id: %s) and all associated certificates be replaced.", key.Name, key.ID),
	)
}
