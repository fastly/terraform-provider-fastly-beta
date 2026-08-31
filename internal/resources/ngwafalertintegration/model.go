package ngwafalertintegration

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

// Model is the shared Terraform state model for all workspace-scoped NGWAF
// alert integration resources. Type-specific resources expose only the config
// attributes that apply to their integration type.
type Model struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Description types.String `tfsdk:"description"`

	Address   types.String `tfsdk:"address"`
	Host      types.String `tfsdk:"host"`
	IssueType types.String `tfsdk:"issue_type"`
	Key       types.String `tfsdk:"key"`
	Project   types.String `tfsdk:"project"`
	Site      types.String `tfsdk:"site"`
	Username  types.String `tfsdk:"username"`
	Webhook   types.String `tfsdk:"webhook"`
}

type RemoteAlert struct {
	ID          string
	Type        string
	Description string
	Config      map[string]string
}

type ConfigAttribute struct {
	Name        string
	Description string
	Sensitive   bool
}

type Definition struct {
	Type        string
	TypeSuffix  string
	DisplayName string
	Description string
	ConfigAttrs []ConfigAttribute
	Operations  Operations
}

type Operations interface {
	Create(ctx context.Context, client *fastly.Client, plan Model) (*RemoteAlert, error)
	Get(ctx context.Context, client *fastly.Client, workspaceID, alertID string) (*RemoteAlert, error)
	Update(ctx context.Context, client *fastly.Client, alertID string, plan Model) (*RemoteAlert, error)
	Delete(ctx context.Context, client *fastly.Client, workspaceID, alertID string) error
	List(ctx context.Context, client *fastly.Client, workspaceID string) ([]RemoteAlert, error)
}

func ModelFromPlan(ctx context.Context, plan tfsdk.Plan, def Definition) (Model, diag.Diagnostics) {
	var m Model
	var diags diag.Diagnostics

	diags.Append(plan.GetAttribute(ctx, path.Root("workspace_id"), &m.WorkspaceID)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("description"), &m.Description)...)
	for _, attr := range def.ConfigAttrs {
		diags.Append(readPlanAttribute(ctx, plan, attr.Name, &m)...)
	}

	return m, diags
}

func ModelFromState(ctx context.Context, state tfsdk.State, def Definition) (Model, diag.Diagnostics) {
	var m Model
	var diags diag.Diagnostics

	diags.Append(state.GetAttribute(ctx, path.Root("id"), &m.ID)...)
	diags.Append(state.GetAttribute(ctx, path.Root("workspace_id"), &m.WorkspaceID)...)
	diags.Append(state.GetAttribute(ctx, path.Root("description"), &m.Description)...)
	for _, attr := range def.ConfigAttrs {
		diags.Append(readStateAttribute(ctx, state, attr.Name, &m)...)
	}

	return m, diags
}

func readPlanAttribute(ctx context.Context, plan tfsdk.Plan, name string, m *Model) diag.Diagnostics {
	var value types.String
	diags := plan.GetAttribute(ctx, path.Root(name), &value)
	if diags.HasError() {
		return diags
	}
	setModelAttribute(m, name, value)
	return diags
}

func readStateAttribute(ctx context.Context, state tfsdk.State, name string, m *Model) diag.Diagnostics {
	var value types.String
	diags := state.GetAttribute(ctx, path.Root(name), &value)
	if diags.HasError() {
		return diags
	}
	setModelAttribute(m, name, value)
	return diags
}

func SetModelState(ctx context.Context, state *tfsdk.State, def Definition, m Model) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Append(state.SetAttribute(ctx, path.Root("id"), m.ID)...)
	diags.Append(state.SetAttribute(ctx, path.Root("workspace_id"), m.WorkspaceID)...)
	diags.Append(state.SetAttribute(ctx, path.Root("description"), m.Description)...)

	for _, attr := range def.ConfigAttrs {
		diags.Append(state.SetAttribute(ctx, path.Root(attr.Name), modelAttribute(m, attr.Name))...)
	}

	return diags
}

func setModelAttribute(m *Model, name string, value types.String) {
	switch name {
	case "address":
		m.Address = value
	case "host":
		m.Host = value
	case "issue_type":
		m.IssueType = value
	case "key":
		m.Key = value
	case "project":
		m.Project = value
	case "site":
		m.Site = value
	case "username":
		m.Username = value
	case "webhook":
		m.Webhook = value
	}
}

func modelAttribute(m Model, name string) types.String {
	switch name {
	case "address":
		return m.Address
	case "host":
		return m.Host
	case "issue_type":
		return m.IssueType
	case "key":
		return m.Key
	case "project":
		return m.Project
	case "site":
		return m.Site
	case "username":
		return m.Username
	case "webhook":
		return m.Webhook
	default:
		return types.StringNull()
	}
}
