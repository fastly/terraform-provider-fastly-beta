package ngwafworkspacelists

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwaflists"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Lists       types.List   `tfsdk:"lists"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_lists"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly Next-Gen WAF lists scoped to a single workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the workspace.",
			},
			"lists": ngwaflists.ListsAttribute("The list of workspace-scoped NGWAF lists."),
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

	workspaceID := state.WorkspaceID.ValueString()

	tflog.Debug(ctx, "Reading Fastly NGWAF workspace lists", map[string]any{"workspace_id": workspaceID})

	remote, err := lists.ListLists(ctx, d.client, &lists.ListInput{
		Scope: &scope.Scope{
			Type:      scope.ScopeTypeWorkspace,
			AppliesTo: []string{workspaceID},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF workspace lists", err.Error())
		return
	}

	var data []lists.List
	if remote != nil {
		data = remote.Data
	}

	listValue, ids, diags := ngwaflists.FlattenLists(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Lists = listValue
	state.ID = types.StringValue(idhash.HashIDs(append([]string{workspaceID}, ids...)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
