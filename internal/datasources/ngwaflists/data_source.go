package ngwaflists

import (
	"context"
	"sort"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	ID    types.String `tfsdk:"id"`
	Lists types.List   `tfsdk:"lists"`
}

var listAttrTypes = map[string]attr.Type{
	"created_at":   types.StringType,
	"description":  types.StringType,
	"id":           types.StringType,
	"name":         types.StringType,
	"reference_id": types.StringType,
	"type":         types.StringType,
	"updated_at":   types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_lists"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve Fastly Next-Gen WAF lists defined at account scope.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"lists": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The list of account-scoped NGWAF lists.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "The date and time in ISO 8601 format when the list was created.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "The description of the list.",
						},
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the list.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the list.",
						},
						"reference_id": schema.StringAttribute{
							Computed:    true,
							Description: "The reference ID of the list.",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "The type of the list.",
						},
						"updated_at": schema.StringAttribute{
							Computed:    true,
							Description: "The date and time in ISO 8601 format when the list was last updated.",
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

	tflog.Debug(ctx, "Reading Fastly NGWAF account lists")

	remote, err := lists.ListLists(ctx, d.client, accountListsListInput())
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF account lists", err.Error())
		return
	}

	var data []lists.List
	if remote != nil {
		data = remote.Data
	}

	listValue, ids, diags := flattenLists(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Lists = listValue
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func accountListsListInput() *lists.ListInput {
	return &lists.ListInput{
		Scope: &scope.Scope{Type: scope.ScopeTypeAccount},
	}
}

func flattenLists(data []lists.List) (types.List, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	sorted := append([]lists.List(nil), data...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].ListID == sorted[j].ListID {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].ListID < sorted[j].ListID
	})

	ids := make([]string, 0, len(sorted))
	elements := make([]attr.Value, 0, len(sorted))

	for _, list := range sorted {
		ids = append(ids, list.ListID)

		obj, objDiags := types.ObjectValue(listAttrTypes, map[string]attr.Value{
			"created_at":   types.StringValue(list.CreatedAt.Format("2006-01-02T15:04:05Z")),
			"description":  types.StringValue(list.Description),
			"id":           types.StringValue(list.ListID),
			"name":         types.StringValue(list.Name),
			"reference_id": types.StringValue(list.ReferenceID),
			"type":         types.StringValue(list.Type),
			"updated_at":   types.StringValue(list.UpdatedAt.Format("2006-01-02T15:04:05Z")),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	listValue, listDiags := types.ListValue(
		types.ObjectType{AttrTypes: listAttrTypes},
		elements,
	)
	diags.Append(listDiags...)

	return listValue, ids, diags
}
