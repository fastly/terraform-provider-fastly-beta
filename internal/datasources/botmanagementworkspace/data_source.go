package botmanagementworkspace

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"
	ws "github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces"
)

var (
	_ datasource.DataSource                     = &DataSource{}
	_ datasource.DataSourceWithConfigure        = &DataSource{}
	_ datasource.DataSourceWithConfigValidators = &DataSource{}
)

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	ServiceID      types.String `tfsdk:"service_id"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	ProtectionMode types.String `tfsdk:"protection_mode"`
	Services       types.List   `tfsdk:"services"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bot_management_workspace"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to look up a Fastly Bot Management (ContentGuard) workspace, either by the ID of a service attached to it or by its workspace ID. Exactly one of `service_id` or `workspace_id` must be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the Bot Management workspace.",
			},
			"service_id": schema.StringAttribute{
				Optional:    true,
				Description: "The ID of a service attached to the Bot Management workspace. Conflicts with `workspace_id`.",
			},
			"workspace_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The ID of the Bot Management workspace. Conflicts with `service_id`.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "The display name of the Bot Management workspace.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "A description of the Bot Management workspace.",
			},
			"protection_mode": schema.StringAttribute{
				Computed:    true,
				Description: "The protection mode of the Bot Management workspace. One of `off`, `log`, or `block`.",
			},
			"services": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The IDs of the services attached to the Bot Management workspace.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "The date and time the Bot Management workspace was created, in RFC 3339 format.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "The date and time the Bot Management workspace was last updated, in RFC 3339 format.",
			},
		},
	}
}

func (d *DataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("service_id"),
			path.MatchRoot("workspace_id"),
		),
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
	var config DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var workspace *ws.Workspace
	if !config.WorkspaceID.IsNull() {
		workspaceID := config.WorkspaceID.ValueString()
		tflog.Debug(ctx, "Reading Fastly Bot Management workspace", map[string]any{"workspace_id": workspaceID})

		w, err := ws.Get(ctx, d.client, &ws.GetInput{WorkspaceID: &workspaceID})
		if err != nil {
			if errors.IsNotFound(err) {
				resp.Diagnostics.AddError("Bot Management workspace not found", fmt.Sprintf("No Bot Management workspace exists with ID %q.", workspaceID))
				return
			}
			resp.Diagnostics.AddError("Error reading Bot Management workspace", err.Error())
			return
		}
		workspace = w
	} else {
		serviceID := config.ServiceID.ValueString()
		tflog.Debug(ctx, "Looking up Fastly Bot Management workspace by service", map[string]any{"service_id": serviceID})

		workspaces, err := ws.List(ctx, d.client, &ws.ListInput{ServiceID: &serviceID})
		if err != nil {
			resp.Diagnostics.AddError("Error listing Bot Management workspaces", err.Error())
			return
		}
		switch len(workspaces) {
		case 0:
			resp.Diagnostics.AddError(
				"Bot Management workspace not found",
				fmt.Sprintf("Service %q isn't attached to a Bot Management workspace. A Bot Management workspace is created when ContentGuard is enabled on the service, for example with `fastly_service_product_bot_management` and `contentguard = \"on\"`. Enable it in an earlier apply before looking up the workspace.", serviceID),
			)
			return
		case 1:
			workspace = &workspaces[0]
		default:
			resp.Diagnostics.AddError(
				"Multiple Bot Management workspaces found",
				fmt.Sprintf("Expected service %q to be attached to one Bot Management workspace, found %d.", serviceID, len(workspaces)),
			)
			return
		}
	}

	services, diags := types.ListValueFrom(ctx, types.StringType, nonNil(workspace.Services))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := DataSourceModel{
		ID:             types.StringValue(workspace.ID),
		ServiceID:      config.ServiceID,
		WorkspaceID:    types.StringValue(workspace.ID),
		Name:           types.StringValue(workspace.Name),
		Description:    types.StringValue(workspace.Description),
		ProtectionMode: types.StringValue(workspace.ProtectionMode),
		Services:       services,
		CreatedAt:      types.StringValue(workspace.CreatedAt),
		UpdatedAt:      types.StringValue(workspace.UpdatedAt),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
