package ngwafsignal

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
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
	resp.TypeName = req.ProviderTypeName + "_ngwaf_signal"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly Next-Gen WAF custom signal defined at account scope.",
		Attributes:  resourceAttributes(),
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

	input, diags := BuildCreateInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly NGWAF account signal", map[string]any{
		"name": plan.Name.ValueString(),
	})

	signal, err := signals.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating NGWAF account signal", err.Error())
		return
	}

	state, err := FlattenToModel(ctx, signal)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening NGWAF account signal after create", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	signalID := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly NGWAF account signal", map[string]any{"id": signalID})

	signal, err := signals.Get(ctx, r.client, BuildGetInput(signalID))
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF account signal not found, removing from state", map[string]any{"id": signalID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF account signal", err.Error())
		return
	}

	newState, err := FlattenToModel(ctx, signal)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening NGWAF account signal after read", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	var state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	signalID := state.ID.ValueString()
	input, diags := BuildUpdateInput(ctx, signalID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly NGWAF account signal", map[string]any{"id": signalID})

	signal, err := signals.Update(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF account signal", err.Error())
		return
	}

	newState, err := FlattenToModel(ctx, signal)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening NGWAF account signal after update", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	signalID := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly NGWAF account signal", map[string]any{"id": signalID})

	err := signals.Delete(ctx, r.client, BuildDeleteInput(signalID))
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting NGWAF account signal", err.Error())
	}
}

// ImportState accepts a bare signal ID. The account endpoint addresses a
// signal by ID alone, and Read repopulates applies_to from the API.
func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
