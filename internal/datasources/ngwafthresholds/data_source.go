package ngwafthresholds

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/datasources/idhash"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	th "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/thresholds"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Thresholds  types.Set    `tfsdk:"thresholds"`
}

var thresholdAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"action":      types.StringType,
	"dont_notify": types.BoolType,
	"duration":    types.Int64Type,
	"enabled":     types.BoolType,
	"interval":    types.Int64Type,
	"limit":       types.Int64Type,
	"name":        types.StringType,
	"signal":      types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_thresholds"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly Next-Gen WAF thresholds for a workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the workspace.",
			},
			"thresholds": schema.SetNestedAttribute{
				Computed:    true,
				Description: "List of all thresholds for the workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the threshold.",
						},
						"action": schema.StringAttribute{
							Computed:    true,
							Description: "Action to take when threshold is exceeded.",
						},
						"dont_notify": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether to silence notifications when action is taken.",
						},
						"duration": schema.Int64Attribute{
							Computed:    true,
							Description: "Duration the action is in place, in seconds.",
						},
						"enabled": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether this threshold is active.",
						},
						"interval": schema.Int64Attribute{
							Computed:    true,
							Description: "Threshold interval in seconds.",
						},
						"limit": schema.Int64Attribute{
							Computed:    true,
							Description: "Threshold limit.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the threshold.",
						},
						"signal": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the signal this threshold is acting on.",
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
	tflog.Debug(ctx, "Reading Fastly NGWAF thresholds", map[string]any{"workspace_id": workspaceID})

	thresholds, err := th.List(ctx, d.client, &th.ListInput{WorkspaceID: &workspaceID})
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF thresholds", err.Error())
		return
	}

	thresholdSet, ids, diags := flattenThresholds(thresholds)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Thresholds = thresholdSet
	state.ID = types.StringValue(idhash.HashIDs(append([]string{workspaceID}, ids...)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenThresholds(thresholds *th.Thresholds) (types.Set, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	var data []th.Threshold
	if thresholds != nil {
		data = thresholds.Data
	}

	ids := make([]string, 0, len(data))
	elements := make([]attr.Value, 0, len(data))

	for _, threshold := range data {
		ids = append(ids, threshold.ThresholdID)

		obj, objDiags := types.ObjectValue(thresholdAttrTypes, map[string]attr.Value{
			"id":          types.StringValue(threshold.ThresholdID),
			"action":      types.StringValue(threshold.Action),
			"dont_notify": types.BoolValue(threshold.DontNotify),
			"duration":    types.Int64Value(int64(threshold.Duration)),
			"enabled":     types.BoolValue(threshold.Enabled),
			"interval":    types.Int64Value(int64(threshold.Interval)),
			"limit":       types.Int64Value(int64(threshold.Limit)),
			"name":        types.StringValue(threshold.Name),
			"signal":      types.StringValue(threshold.Signal),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	setValue, setDiags := types.SetValue(
		types.ObjectType{AttrTypes: thresholdAttrTypes},
		elements,
	)
	diags.Append(setDiags...)

	return setValue, ids, diags
}
