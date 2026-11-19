package botmanagementbots

import (
	"context"

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
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CategoryID  types.String `tfsdk:"category_id"`
	Bots        []BotModel   `tfsdk:"bots"`
}

type BotModel struct {
	BotID      types.String `tfsdk:"bot_id"`
	CategoryID types.String `tfsdk:"category_id"`
	Name       types.String `tfsdk:"name"`
	Action     types.String `tfsdk:"action"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bot_management_bots"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to list the predefined bots in a Bot Management (ContentGuard) workspace, optionally limited to one category.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the Bot Management workspace.",
			},
			"category_id": schema.StringAttribute{
				Optional:    true,
				Description: "Only list the bots in this category. When omitted, bots in every category are listed.",
			},
			"bots": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The bots in the Bot Management workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"bot_id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the bot.",
						},
						"category_id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the category the bot belongs to.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The display name of the bot.",
						},
						"action": schema.StringAttribute{
							Computed:    true,
							Description: "The bot's action. One of `allow`, `block`, `challenge`, or `inherit` (follows the category's action).",
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
	input := &policy.ListBotsInput{WorkspaceID: &workspaceID}
	if !state.CategoryID.IsNull() {
		input.CategoryID = new(state.CategoryID.ValueString())
	}
	tflog.Debug(ctx, "Reading Fastly Bot Management bots", map[string]any{"workspace_id": workspaceID, "category_id": state.CategoryID.ValueString()})

	bots, err := policy.ListBots(ctx, d.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Bot Management bots", err.Error())
		return
	}

	ids := []string{workspaceID, state.CategoryID.ValueString()}
	state.Bots = make([]BotModel, 0, len(bots))
	for _, b := range bots {
		// The category-scoped endpoint omits category_id from each bot.
		categoryID := b.CategoryID
		if categoryID == "" {
			categoryID = state.CategoryID.ValueString()
		}
		ids = append(ids, categoryID+"/"+b.BotID)
		state.Bots = append(state.Bots, BotModel{
			BotID:      types.StringValue(b.BotID),
			CategoryID: types.StringValue(categoryID),
			Name:       types.StringValue(b.Name),
			Action:     types.StringValue(b.Action.Type),
		})
	}
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
