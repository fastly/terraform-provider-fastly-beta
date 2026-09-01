package ngwafalertintegrations

import (
	"context"
	"fmt"
	"sort"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
	def    ngwafalertintegration.Definition
}

type Model struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Alerts      types.List   `tfsdk:"alerts"`
}

var alertAttrTypes = map[string]attr.Type{
	"description": types.StringType,
	"id":          types.StringType,
}

func NewWorkspaceDataSource(def ngwafalertintegration.Definition) datasource.DataSource {
	return &DataSource{def: def}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_alert_" + d.def.TypeSuffix + "_integrations"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: fmt.Sprintf("Use this data source to retrieve %s alert integrations scoped to a single Fastly Next-Gen WAF workspace.", d.def.Type),
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the workspace.",
			},
			"alerts": schema.ListNestedAttribute{
				Computed:    true,
				Description: fmt.Sprintf("The list of %s alert integrations scoped to the workspace.", d.def.Type),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "The description of the alert integration.",
						},
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the alert integration.",
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
	var state Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	tflog.Debug(ctx, "Reading Fastly NGWAF workspace alert integrations", map[string]any{
		"workspace_id": workspaceID,
		"type":         d.def.Type,
	})

	remote, err := d.def.Operations.List(ctx, d.client, workspaceID)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error listing NGWAF workspace %s alert integrations", d.def.Type), err.Error())
		return
	}

	alertList, ids, diags := FlattenAlerts(remote)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Alerts = alertList
	state.ID = types.StringValue(idhash.HashIDs(append([]string{workspaceID}, ids...)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func FlattenAlerts(remote []ngwafalertintegration.RemoteAlert) (types.List, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	sorted := append([]ngwafalertintegration.RemoteAlert(nil), remote...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].ID == sorted[j].ID {
			return sorted[i].Description < sorted[j].Description
		}
		return sorted[i].ID < sorted[j].ID
	})

	ids := make([]string, 0, len(sorted))
	elements := make([]attr.Value, 0, len(sorted))

	for _, alert := range sorted {
		ids = append(ids, alert.ID)

		obj, objDiags := types.ObjectValue(alertAttrTypes, map[string]attr.Value{
			"description": types.StringValue(alert.Description),
			"id":          types.StringValue(alert.ID),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	listValue, listDiags := types.ListValue(
		types.ObjectType{AttrTypes: alertAttrTypes},
		elements,
	)
	diags.Append(listDiags...)

	return listValue, ids, diags
}
