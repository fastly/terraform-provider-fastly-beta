package botmanagementcategories

import (
	"cmp"
	"context"
	"slices"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

var (
	_ datasource.DataSource              = &DataSource{}
	_ datasource.DataSourceWithConfigure = &DataSource{}
)

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID          types.String    `tfsdk:"id"`
	WorkspaceID types.String    `tfsdk:"workspace_id"`
	Categories  []CategoryModel `tfsdk:"categories"`
}

type CategoryModel struct {
	CategoryID  types.String `tfsdk:"category_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Action      types.String `tfsdk:"action"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bot_management_categories"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to list the predefined bot categories in a Bot Management (ContentGuard) workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the Bot Management workspace.",
			},
			"categories": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The bot categories in the Bot Management workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"category_id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the category.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The display name of the category.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "A description of the bots the category contains.",
						},
						"action": schema.StringAttribute{
							Computed:    true,
							Description: "The category's action. One of `allow`, `block`, or `challenge`.",
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

	workspaceID := state.WorkspaceID.ValueString()
	tflog.Debug(ctx, "Reading Fastly Bot Management categories", map[string]any{"workspace_id": workspaceID})

	categories, err := policy.ListCategories(ctx, d.client, &policy.ListCategoriesInput{WorkspaceID: &workspaceID})
	if err != nil {
		resp.Diagnostics.AddError("Error listing Bot Management categories", err.Error())
		return
	}
	slices.SortFunc(categories, func(a, b policy.Category) int {
		return cmp.Compare(a.CategoryID, b.CategoryID)
	})

	ids := []string{workspaceID}
	state.Categories = make([]CategoryModel, 0, len(categories))
	for _, c := range categories {
		ids = append(ids, c.CategoryID)
		state.Categories = append(state.Categories, CategoryModel{
			CategoryID:  types.StringValue(c.CategoryID),
			Name:        types.StringValue(c.Name),
			Description: types.StringValue(c.Description),
			Action:      types.StringValue(c.Action.Type),
		})
	}
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
