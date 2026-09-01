package ngwafworkspaceredactions

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
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/redactions"
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
	Redactions  types.List   `tfsdk:"redactions"`
}

var redactionAttrTypes = map[string]attr.Type{
	"field": types.StringType,
	"id":    types.StringType,
	"type":  types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_redactions"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve field redactions scoped to a single Fastly Next-Gen WAF workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the workspace.",
			},
			"redactions": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The list of field redactions scoped to the workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the field that is being redacted.",
						},
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the redaction.",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "The type of field being redacted. One of `request_parameter`, `request_header`, or `response_header`.",
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

	tflog.Debug(ctx, "Reading Fastly NGWAF workspace redactions", map[string]any{"workspace_id": workspaceID})

	remote, err := redactions.List(ctx, d.client, &redactions.ListInput{
		WorkspaceID: &workspaceID,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF workspace redactions", err.Error())
		return
	}

	var data []redactions.Redaction
	if remote != nil {
		data = remote.Data
	}

	redactionList, ids, diags := flattenRedactions(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Redactions = redactionList
	state.ID = types.StringValue(idhash.HashIDs(append([]string{workspaceID}, ids...)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenRedactions(data []redactions.Redaction) (types.List, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	sorted := append([]redactions.Redaction(nil), data...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].RedactionID == sorted[j].RedactionID {
			return sorted[i].Field < sorted[j].Field
		}
		return sorted[i].RedactionID < sorted[j].RedactionID
	})

	ids := make([]string, 0, len(sorted))
	elements := make([]attr.Value, 0, len(sorted))

	for _, r := range sorted {
		ids = append(ids, r.RedactionID)

		obj, objDiags := types.ObjectValue(redactionAttrTypes, map[string]attr.Value{
			"field": types.StringValue(r.Field),
			"id":    types.StringValue(r.RedactionID),
			"type":  types.StringValue(r.Type),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	listValue, listDiags := types.ListValue(
		types.ObjectType{AttrTypes: redactionAttrTypes},
		elements,
	)
	diags.Append(listDiags...)

	return listValue, ids, diags
}
