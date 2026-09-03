package ngwaflist

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

// Definition describes one concrete type-specific NGWAF list resource.
type Definition struct {
	ListType    string
	TypeSuffix  string
	Description string
	Scope       scope.Type
}

// Resource implements one concrete type-specific NGWAF list resource.
type Resource struct {
	client     *fastly.Client
	definition Definition
}

type resourceModel struct {
	ID          types.String
	WorkspaceID types.String
	Name        types.String
	Description types.String
	Entries     types.List
	ReferenceID types.String
}

// NewResource returns the NGWAF list resource described by definition.
func NewResource(definition Definition) resource.Resource {
	return &Resource{definition: definition}
}

// NewWorkspaceResource returns a workspace-scoped NGWAF list resource for one
// concrete API list type.
func NewWorkspaceResource(listType, typeSuffix, description string) resource.Resource {
	return NewResource(Definition{
		ListType:    listType,
		TypeSuffix:  typeSuffix,
		Description: description,
		Scope:       scope.ScopeTypeWorkspace,
	})
}

// NewAccountResource returns an account-scoped NGWAF list resource for one
// concrete API list type.
func NewAccountResource(listType, typeSuffix, description string) resource.Resource {
	return NewResource(Definition{
		ListType:    listType,
		TypeSuffix:  typeSuffix,
		Description: description,
		Scope:       scope.ScopeTypeAccount,
	})
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	scopePrefix := ""
	if r.definition.Scope == scope.ScopeTypeWorkspace {
		scopePrefix = "workspace_"
	}
	resp.TypeName = req.ProviderTypeName + "_ngwaf_" + scopePrefix + r.definition.TypeSuffix + "_list"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := AccountAttributes(r.definition.ListType)
	if r.definition.Scope == scope.ScopeTypeWorkspace {
		attributes = Attributes(r.definition.ListType)
	}

	resp.Schema = schema.Schema{
		Description: r.definition.Description,
		Attributes:  attributes,
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
	plan := r.readPlan(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var input *lists.CreateInput
	var diags diag.Diagnostics
	if r.definition.Scope == scope.ScopeTypeAccount {
		input, diags = BuildAccountCreateInput(ctx, r.definition.ListType, plan.accountModel())
	} else {
		input, diags = BuildCreateInput(ctx, r.definition.ListType, plan.workspaceModel())
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly NGWAF list", r.logFields(plan))

	list, err := lists.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError(r.operationError("creating"), err.Error())
		return
	}

	newState, err := r.flatten(plan.WorkspaceID.ValueString(), list)
	if err != nil {
		resp.Diagnostics.AddError(r.operationError("reading"), err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	state := r.readState(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var input *lists.GetInput
	if r.definition.Scope == scope.ScopeTypeAccount {
		input = BuildAccountGetInput(state.ID.ValueString())
	} else {
		input = BuildGetInput(state.WorkspaceID.ValueString(), state.ID.ValueString())
	}

	tflog.Debug(ctx, "Reading Fastly NGWAF list", r.logFields(state))

	list, err := lists.Get(ctx, r.client, input)
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF list not found, removing from state", r.logFields(state))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(r.operationError("reading"), err.Error())
		return
	}

	newState, err := r.flatten(state.WorkspaceID.ValueString(), list)
	if err != nil {
		resp.Diagnostics.AddError(r.operationError("reading"), err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan := r.readPlan(ctx, req.Plan, &resp.Diagnostics)
	state := r.readState(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var input *lists.UpdateInput
	var diags diag.Diagnostics
	if r.definition.Scope == scope.ScopeTypeAccount {
		input, diags = BuildAccountUpdateInput(ctx, state.ID.ValueString(), plan.accountModel())
	} else {
		input, diags = BuildUpdateInput(ctx, state.ID.ValueString(), plan.workspaceModel())
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly NGWAF list", r.logFields(resourceModel{
		ID:          state.ID,
		WorkspaceID: plan.WorkspaceID,
		Name:        plan.Name,
	}))

	list, err := lists.Update(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError(r.operationError("updating"), err.Error())
		return
	}

	newState, err := r.flatten(plan.WorkspaceID.ValueString(), list)
	if err != nil {
		resp.Diagnostics.AddError(r.operationError("reading"), err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	state := r.readState(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var input *lists.DeleteInput
	if r.definition.Scope == scope.ScopeTypeAccount {
		input = BuildAccountDeleteInput(state.ID.ValueString())
	} else {
		input = BuildDeleteInput(state.WorkspaceID.ValueString(), state.ID.ValueString())
	}

	tflog.Debug(ctx, "Deleting Fastly NGWAF list", r.logFields(state))

	err := lists.Delete(ctx, r.client, input)
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError(r.operationError("deleting"), err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.definition.Scope == scope.ScopeTypeAccount {
		ImportAccountState(ctx, req, resp)
		return
	}
	ImportState(ctx, req, resp)
}

func (r *Resource) readPlan(ctx context.Context, plan tfsdk.Plan, diags *diag.Diagnostics) resourceModel {
	if r.definition.Scope == scope.ScopeTypeAccount {
		var model AccountModel
		diags.Append(plan.Get(ctx, &model)...)
		return resourceModelFromAccount(model)
	}

	var model Model
	diags.Append(plan.Get(ctx, &model)...)
	return resourceModelFromWorkspace(model)
}

func (r *Resource) readState(ctx context.Context, state tfsdk.State, diags *diag.Diagnostics) resourceModel {
	if r.definition.Scope == scope.ScopeTypeAccount {
		var model AccountModel
		diags.Append(state.Get(ctx, &model)...)
		return resourceModelFromAccount(model)
	}

	var model Model
	diags.Append(state.Get(ctx, &model)...)
	return resourceModelFromWorkspace(model)
}

func (r *Resource) flatten(workspaceID string, list *lists.List) (any, error) {
	if r.definition.Scope == scope.ScopeTypeAccount {
		return FlattenAccountToModel(r.definition.ListType, list)
	}
	return FlattenToModel(r.definition.ListType, workspaceID, list)
}

func (r *Resource) operationError(operation string) string {
	return fmt.Sprintf("Error %s NGWAF %s %s list", operation, r.definition.Scope, r.definition.ListType)
}

func (r *Resource) logFields(model resourceModel) map[string]any {
	fields := map[string]any{
		"id":    model.ID.ValueString(),
		"type":  r.definition.ListType,
		"scope": r.definition.Scope,
	}
	if !model.Name.IsNull() && !model.Name.IsUnknown() {
		fields["name"] = model.Name.ValueString()
	}
	if r.definition.Scope == scope.ScopeTypeWorkspace {
		fields["workspace_id"] = model.WorkspaceID.ValueString()
	}
	return fields
}

func resourceModelFromWorkspace(model Model) resourceModel {
	return resourceModel(model)
}

func resourceModelFromAccount(model AccountModel) resourceModel {
	return resourceModel{
		ID:          model.ID,
		Name:        model.Name,
		Description: model.Description,
		Entries:     model.Entries,
		ReferenceID: model.ReferenceID,
	}
}

func (m resourceModel) workspaceModel() Model {
	return Model(m)
}

func (m resourceModel) accountModel() AccountModel {
	return AccountModel{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Entries:     m.Entries,
		ReferenceID: m.ReferenceID,
	}
}
