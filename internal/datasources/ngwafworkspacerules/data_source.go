package ngwafworkspacerules

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Rules       types.List   `tfsdk:"rules"`
}

var ruleAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"type":        types.StringType,
	"description": types.StringType,
	"enabled":     types.BoolType,
	"created_at":  types.StringType,
	"updated_at":  types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_rules"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly Next-Gen WAF rules scoped to a single workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the workspace.",
			},
			"rules": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The list of rules.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the rule.",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "The type of the rule.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "The description of the rule.",
						},
						"enabled": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the rule is currently enabled.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "The date and time in ISO 8601 format when the rule was created.",
						},
						"updated_at": schema.StringAttribute{
							Computed:    true,
							Description: "The date and time in ISO 8601 format when the rule was last updated.",
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

	workspaceID := service.StringValue(state.WorkspaceID)
	tflog.Debug(ctx, "Reading Fastly NGWAF workspace rules", map[string]any{"workspace_id": workspaceID})

	remoteState, err := rules.List(ctx, d.client, &rules.ListInput{
		Scope: &scope.Scope{
			Type:      scope.ScopeTypeWorkspace,
			AppliesTo: []string{workspaceID},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF workspace rules", err.Error())
		return
	}

	ids := make([]string, 0, len(remoteState.Data))
	elements := make([]attr.Value, 0, len(remoteState.Data))
	for _, rule := range remoteState.Data {
		ids = append(ids, rule.RuleID)

		obj, diags := types.ObjectValue(ruleAttrTypes, map[string]attr.Value{
			"id":          types.StringValue(rule.RuleID),
			"type":        types.StringValue(rule.Type),
			"description": types.StringValue(rule.Description),
			"enabled":     types.BoolValue(rule.Enabled),
			"created_at":  types.StringValue(rule.CreatedAt.Format("2006-01-02T15:04:05Z")),
			"updated_at":  types.StringValue(rule.UpdatedAt.Format("2006-01-02T15:04:05Z")),
		})
		resp.Diagnostics.Append(diags...)
		elements = append(elements, obj)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	listVal, diags := types.ListValue(types.ObjectType{AttrTypes: ruleAttrTypes}, elements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Rules = listVal
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
