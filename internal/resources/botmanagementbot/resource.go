package botmanagementbot

import (
	"context"
	"fmt"
	"strings"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
	_ resource.ResourceWithIdentity    = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bot_management_bot"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the action of a predefined bot in a Bot Management (ContentGuard) workspace. This resource doesn't create or delete bots, and destroying the resource sets the bot's action back to `inherit`.",
		Attributes:  ResourceAttributes(),
	}
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = resourceidentity.BotManagementBotSchema()
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

	tflog.Debug(ctx, "Adopting Fastly Bot Management bot", logFields(plan))
	r.write(ctx, plan, &resp.State, resp.Identity, &resp.Diagnostics)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	categoryID := state.CategoryID.ValueString()
	botID := state.BotID.ValueString()
	tflog.Debug(ctx, "Reading Fastly Bot Management bot", logFields(state))

	bot, err := policy.GetBot(ctx, r.client, &policy.GetBotInput{WorkspaceID: &workspaceID, CategoryID: &categoryID, BotID: &botID})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Bot Management bot not found, removing from state", logFields(state))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Bot Management bot", err.Error())
		return
	}

	newState := FlattenToModel(workspaceID, categoryID, bot)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
	resourceidentity.Set(ctx, resp.Identity, &resp.Diagnostics, resourceidentity.BotManagementBot(newState.WorkspaceID, newState.CategoryID, newState.BotID))
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly Bot Management bot", logFields(plan))
	r.write(ctx, plan, &resp.State, resp.Identity, &resp.Diagnostics)
}

// Delete returns the bot to its category's action.
func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Resetting Fastly Bot Management bot to inherit", logFields(state))

	if _, err := policy.UpdateBot(ctx, r.client, BuildUpdateInput(state, InheritAction)); err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error resetting Bot Management bot to inherit", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resourceidentity.ImportBotManagementBot(ctx, req, resp)
		return
	}

	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected import identifier of the form \"workspace_id/category_id/bot_id\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("category_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bot_id"), parts[2])...)
}

func (r *Resource) write(ctx context.Context, plan Model, state *tfsdk.State, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) {
	bot, err := policy.UpdateBot(ctx, r.client, BuildUpdateInput(plan, service.StringValue(plan.Action)))
	if err != nil {
		diags.AddError("Error updating Bot Management bot", err.Error())
		return
	}

	newState := FlattenToModel(plan.WorkspaceID.ValueString(), plan.CategoryID.ValueString(), bot)
	diags.Append(state.Set(ctx, &newState)...)
	resourceidentity.Set(ctx, identity, diags, resourceidentity.BotManagementBot(newState.WorkspaceID, newState.CategoryID, newState.BotID))
}

func logFields(m Model) map[string]any {
	return map[string]any{
		"workspace_id": m.WorkspaceID.ValueString(),
		"category_id":  m.CategoryID.ValueString(),
		"bot_id":       m.BotID.ValueString(),
	}
}
