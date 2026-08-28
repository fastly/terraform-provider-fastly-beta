package ngwafworkspacesignals

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Signals     types.List   `tfsdk:"signals"`
}

var signalAttrTypes = map[string]attr.Type{
	"description":  types.StringType,
	"id":           types.StringType,
	"name":         types.StringType,
	"reference_id": types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_signals"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve custom signals scoped to a single Fastly Next-Gen WAF workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the workspace.",
			},
			"signals": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The list of custom signals scoped to the workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "The description of the signal.",
						},
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the signal.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the signal.",
						},
						"reference_id": schema.StringAttribute{
							Computed:    true,
							Description: "The generated reference ID of the signal.",
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

	tflog.Debug(ctx, "Reading Fastly NGWAF workspace signals", map[string]any{"workspace_id": workspaceID})

	remote, err := signals.List(ctx, d.client, &signals.ListInput{
		Scope: &scope.Scope{
			Type:      scope.ScopeTypeWorkspace,
			AppliesTo: []string{workspaceID},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF workspace signals", err.Error())
		return
	}

	var data []signals.Signal
	if remote != nil {
		data = remote.Data
	}

	signalList, ids, diags := flattenSignals(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Signals = signalList
	state.ID = types.StringValue(idhash.HashIDs(append([]string{workspaceID}, ids...)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenSignals(data []signals.Signal) (types.List, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	sorted := append([]signals.Signal(nil), data...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SignalID == sorted[j].SignalID {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].SignalID < sorted[j].SignalID
	})

	ids := make([]string, 0, len(sorted))
	elements := make([]attr.Value, 0, len(sorted))

	for _, signal := range sorted {
		ids = append(ids, signal.SignalID)

		obj, objDiags := types.ObjectValue(signalAttrTypes, map[string]attr.Value{
			"description":  types.StringValue(signal.Description),
			"id":           types.StringValue(signal.SignalID),
			"name":         types.StringValue(signal.Name),
			"reference_id": types.StringValue(signal.ReferenceID),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	listValue, listDiags := types.ListValue(
		types.ObjectType{AttrTypes: signalAttrTypes},
		elements,
	)
	diags.Append(listDiags...)

	return listValue, ids, diags
}
