// Package ngwafsignals implements the fastly_ngwaf_signals data source.
package ngwafsignals

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
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

var _ datasource.DataSource = &DataSource{}

// listLimit is the maximum accepted by GET /ngwaf/v1/signals. Using the
// endpoint maximum avoids silently truncating Terraform state at the API's
// smaller default page size and matches the account-rules data-source pattern.
const listLimit = 1000

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Signals types.List   `tfsdk:"signals"`
}

var signalAttrTypes = map[string]attr.Type{
	"applies_to":   types.SetType{ElemType: types.StringType},
	"description":  types.StringType,
	"id":           types.StringType,
	"name":         types.StringType,
	"reference_id": types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_signals"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve Fastly Next-Gen WAF custom signals defined at account scope.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"signals": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The list of custom signals defined at account scope.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"applies_to": schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "The workspaces the signal applies to: a set of workspace IDs, or the single entry `*` for every workspace in the account.",
						},
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

	tflog.Debug(ctx, "Reading Fastly NGWAF account signals")

	remote, err := signals.List(ctx, d.client, accountSignalsListInput())
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF account signals", err.Error())
		return
	}

	var data []signals.Signal
	if remote != nil {
		data = remote.Data
	}

	signalList, ids, diags := flattenSignals(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Signals = signalList
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func accountSignalsListInput() *signals.ListInput {
	return &signals.ListInput{
		Limit: new(listLimit),
		Scope: &scope.Scope{Type: scope.ScopeTypeAccount},
	}
}

func flattenSignals(ctx context.Context, data []signals.Signal) (types.List, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Account lists are shared and the API does not promise ordering. Sort a
	// copy so both the visible list and idhash identifier are deterministic.
	sorted := append([]signals.Signal(nil), data...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].SignalID < sorted[j].SignalID
	})

	ids := make([]string, 0, len(sorted))
	elements := make([]attr.Value, 0, len(sorted))

	for _, signal := range sorted {
		ids = append(ids, signal.SignalID)

		appliesTo, appliesToDiags := types.SetValueFrom(ctx, types.StringType, signal.Scope.AppliesTo)
		diags.Append(appliesToDiags...)

		obj, objDiags := types.ObjectValue(signalAttrTypes, map[string]attr.Value{
			"applies_to":   appliesTo,
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
