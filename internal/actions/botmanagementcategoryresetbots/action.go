package botmanagementcategoryresetbots

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

const inheritAction = "inherit"

type Action struct {
	client *fastly.Client
}

var (
	_ action.Action              = &Action{}
	_ action.ActionWithConfigure = &Action{}
)

func NewAction() action.Action {
	return &Action{}
}

type Model struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CategoryID  types.String `tfsdk:"category_id"`
}

func (a *Action) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bot_management_category_reset_bots"
}

func (a *Action) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sets every bot in a Bot Management (ContentGuard) category to `inherit`, so they all follow the category's action. The category's own action is unchanged. Implemented as a Terraform Action (non-CRUD).",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the Bot Management workspace.",
			},
			"category_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the category whose bots are reset, such as `cat_ai_crawler`.",
			},
		},
	}
}

func (a *Action) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}
	a.client = data.Client
}

func (a *Action) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var cfg Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := cfg.WorkspaceID.ValueString()
	categoryID := cfg.CategoryID.ValueString()
	fields := map[string]any{"workspace_id": workspaceID, "category_id": categoryID}

	tflog.Info(ctx, "Resetting every bot in Fastly Bot Management category to inherit", fields)

	bots, err := policy.UpdateCategoryBots(ctx, a.client, &policy.UpdateCategoryBotsInput{
		WorkspaceID: &workspaceID,
		CategoryID:  &categoryID,
		Action:      &policy.Action{Type: inheritAction},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to reset Bot Management category bots", err.Error())
		return
	}

	fields["bot_count"] = len(bots)
	tflog.Info(ctx, "Reset every bot in Fastly Bot Management category to inherit", fields)
}
