package botmanagementcategory

import (
	"context"
	"fmt"
	"strings"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"

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
	resp.TypeName = req.ProviderTypeName + "_bot_management_category"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the action of a predefined bot category in a Bot Management (ContentGuard) workspace. This resource doesn't create or delete categories, and destroying the resource only removes it from Terraform state.",
		Attributes:  ResourceAttributes(),
	}
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = resourceidentity.BotManagementCategorySchema()
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

	tflog.Debug(ctx, "Adopting Fastly Bot Management category", logFields(plan))
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
	tflog.Debug(ctx, "Reading Fastly Bot Management category", logFields(state))

	category, err := policy.GetCategory(ctx, r.client, &policy.GetCategoryInput{WorkspaceID: &workspaceID, CategoryID: &categoryID})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Bot Management category not found, removing from state", logFields(state))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Bot Management category", err.Error())
		return
	}

	newState := FlattenToModel(workspaceID, category)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
	resourceidentity.Set(ctx, resp.Identity, &resp.Diagnostics, resourceidentity.BotManagementCategory(newState.WorkspaceID, newState.CategoryID))
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly Bot Management category", logFields(plan))
	r.write(ctx, plan, &resp.State, resp.Identity, &resp.Diagnostics)
}

// Delete only removes the category from state. Categories can't be deleted
// and have no neutral action to reset to.
func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, _ *resource.DeleteResponse) {
	var state Model
	if diags := req.State.Get(ctx, &state); !diags.HasError() {
		tflog.Debug(ctx, "Removing Fastly Bot Management category from state; its action is left unchanged", logFields(state))
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resourceidentity.ImportBotManagementCategory(ctx, req, resp)
		return
	}

	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected import identifier of the form \"workspace_id/category_id\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("category_id"), parts[1])...)
}

func (r *Resource) write(ctx context.Context, plan Model, state *tfsdk.State, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) {
	category, err := policy.UpdateCategory(ctx, r.client, BuildUpdateInput(plan))
	if err != nil {
		diags.AddError("Error updating Bot Management category", err.Error())
		return
	}

	newState := FlattenToModel(plan.WorkspaceID.ValueString(), category)
	diags.Append(state.Set(ctx, &newState)...)
	resourceidentity.Set(ctx, identity, diags, resourceidentity.BotManagementCategory(newState.WorkspaceID, newState.CategoryID))
}

func logFields(m Model) map[string]any {
	return map[string]any{"workspace_id": m.WorkspaceID.ValueString(), "category_id": m.CategoryID.ValueString()}
}
