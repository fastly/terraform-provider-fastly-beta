package auditlogeventmapping

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v18/fastly"
	"github.com/fastly/go-fastly/v18/fastly/notifications/v1/eventmappings"
)

var (
	_ resource.Resource                   = &Resource{}
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
	resp.TypeName = req.ProviderTypeName + "_audit_log_event_mapping"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Fastly Audit Log Event Mapping, which routes audit log events of the given types to one or more `fastly_integration` notification channels. Event mappings are versionless and independent of any service-version lifecycle.",
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

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.ScopeType.ValueString() != eventmappings.ScopeTypeAccount || config.ScopeIDs.IsNull() || config.ScopeIDs.IsUnknown() {
		return
	}
	if len(config.ScopeIDs.Elements()) > 0 {
		resp.Diagnostics.AddAttributeError(path.Root("scope_ids"),
			"Invalid scope_ids",
			"`scope_ids` must be empty when `scope_type` is `account`.")
	}
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

	tflog.Debug(ctx, "Creating Fastly Audit Log Event Mapping", map[string]any{"name": plan.Name.ValueString()})

	created, err := eventmappings.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Audit Log Event Mapping", err.Error())
		return
	}

	state, diags := FlattenToModel(ctx, created, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
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

	id := state.ID.ValueString()

	tflog.Debug(ctx, "Reading Fastly Audit Log Event Mapping", map[string]any{"id": id})

	m, err := eventmappings.Get(ctx, r.client, &eventmappings.GetInput{MappingID: new(id)})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Audit Log Event Mapping not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading Audit Log Event Mapping", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, m, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
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

	id := state.ID.ValueString()

	input, diags := BuildUpdateInput(ctx, id, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly Audit Log Event Mapping", map[string]any{"id": id})

	updated, err := eventmappings.Update(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Audit Log Event Mapping", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, updated, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
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

	id := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly Audit Log Event Mapping", map[string]any{"id": id})

	err := eventmappings.Delete(ctx, r.client, &eventmappings.DeleteInput{MappingID: new(id)})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Audit Log Event Mapping", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
