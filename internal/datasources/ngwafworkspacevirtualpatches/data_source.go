package ngwafworkspacevirtualpatches

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

	"github.com/fastly/go-fastly/v17/fastly"
	vp "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/virtualpatches"
)

var _ datasource.DataSource = &DataSource{}

// listLimit is the largest page the virtual patches endpoint accepts.
const listLimit = 1000

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
	VirtualPatches types.List   `tfsdk:"virtual_patches"`
}

var virtualPatchAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"mode":        types.StringType,
	"description": types.StringType,
	"enabled":     types.BoolType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_virtual_patches"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly Next-Gen WAF virtual patches for a workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the workspace.",
			},
			"virtual_patches": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of all virtual patches for the workspace.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the virtual patch.",
						},
						"mode": schema.StringAttribute{
							Computed:    true,
							Description: "Action to take when a signal for virtual patch is detected.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "Description of the virtual patch.",
						},
						"enabled": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the virtual patch is enabled.",
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
	tflog.Debug(ctx, "Reading Fastly NGWAF workspace virtual patches", map[string]any{"workspace_id": workspaceID})

	limit := listLimit
	virtualPatches, err := vp.List(ctx, d.client, &vp.ListInput{WorkspaceID: &workspaceID, Limit: &limit})
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF virtual patches", err.Error())
		return
	}

	listValue, ids, diags := flattenVirtualPatches(virtualPatches)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.VirtualPatches = listValue
	state.ID = types.StringValue(idhash.HashIDs(append([]string{workspaceID}, ids...)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenVirtualPatches(virtualPatches *vp.VirtualPatches) (types.List, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	var data []vp.VirtualPatch
	if virtualPatches != nil {
		data = virtualPatches.Data
	}

	sorted := append([]vp.VirtualPatch(nil), data...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	ids := make([]string, 0, len(sorted))
	elements := make([]attr.Value, 0, len(sorted))

	for _, virtualPatch := range sorted {
		ids = append(ids, virtualPatch.ID)

		obj, objDiags := types.ObjectValue(virtualPatchAttrTypes, map[string]attr.Value{
			"id":          types.StringValue(virtualPatch.ID),
			"mode":        types.StringValue(virtualPatch.Mode),
			"description": types.StringValue(virtualPatch.Description),
			"enabled":     types.BoolValue(virtualPatch.Enabled),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	listValue, listDiags := types.ListValue(
		types.ObjectType{AttrTypes: virtualPatchAttrTypes},
		elements,
	)
	diags.Append(listDiags...)

	return listValue, ids, diags
}
