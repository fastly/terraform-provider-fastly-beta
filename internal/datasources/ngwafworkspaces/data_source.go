package ngwafworkspaces

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Workspaces types.Set    `tfsdk:"workspaces"`
}

var workspaceAttrTypes = map[string]attr.Type{
	"id":   types.StringType,
	"name": types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspaces"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly Next-Gen WAF workspaces.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspaces": schema.SetNestedAttribute{
				Computed:    true,
				Description: "List of all Next-Gen WAF workspaces.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Identifier of the workspace.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Name of the workspace.",
						},
					},
				},
			},
		},
	}
}

func (d *DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	d.client = data.Client
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Fastly NGWAF workspaces")

	workspaces, err := ws.List(ctx, d.client, &ws.ListInput{})
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF workspaces", err.Error())
		return
	}

	workspaceSet, ids, diags := flattenWorkspaces(workspaces)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Workspaces = workspaceSet
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenWorkspaces(workspaces *ws.Workspaces) (types.Set, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	var data []ws.Workspace
	if workspaces != nil {
		data = workspaces.Data
	}

	ids := make([]string, 0, len(data))
	elements := make([]attr.Value, 0, len(data))

	for _, workspace := range data {
		ids = append(ids, workspace.WorkspaceID)

		obj, objDiags := types.ObjectValue(workspaceAttrTypes, map[string]attr.Value{
			"id":   types.StringValue(workspace.WorkspaceID),
			"name": types.StringValue(workspace.Name),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	setValue, setDiags := types.SetValue(
		types.ObjectType{AttrTypes: workspaceAttrTypes},
		elements,
	)
	diags.Append(setDiags...)

	return setValue, ids, diags
}
