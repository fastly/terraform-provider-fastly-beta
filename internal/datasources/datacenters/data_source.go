package datacenters

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

var (
	_ datasource.DataSource              = &DataSource{}
	_ datasource.DataSourceWithConfigure = &DataSource{}
)

type DataSource struct {
	client *fastly.Client
}

type Model struct {
	ID   types.String `tfsdk:"id"`
	Pops types.Set    `tfsdk:"pops"`
}

var coordinatesAttrTypes = map[string]attr.Type{
	"latitude":  types.Float64Type,
	"longitude": types.Float64Type,
	"x":         types.Float64Type,
	"y":         types.Float64Type,
}

var popAttrTypes = map[string]attr.Type{
	"code":        types.StringType,
	"group":       types.StringType,
	"name":        types.StringType,
	"shield":      types.StringType,
	"coordinates": types.ObjectType{AttrTypes: coordinatesAttrTypes},
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datacenters"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly POPs (Points of Presence).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Stable Terraform data source identifier derived from the returned POP codes.",
			},
			"pops": schema.SetNestedAttribute{
				Computed:    true,
				Description: "A list of all Fastly POPs. Set semantics are used because POPs are not returned in a guaranteed order.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"code": schema.StringAttribute{
							Computed:    true,
							Description: "A code representing the POP location.",
						},
						"group": schema.StringAttribute{
							Computed:    true,
							Description: "A code representing the general region of the world in which the POP location resides.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the POP.",
						},
						"shield": schema.StringAttribute{
							Computed:    true,
							Description: "A code representing the shielding name of the POP. The value may be empty if the POP is not available for shielding.",
						},
						"coordinates": schema.SingleNestedAttribute{
							Computed:    true,
							Description: "The geographic coordinates of the POP.",
							Attributes: map[string]schema.Attribute{
								"latitude": schema.Float64Attribute{
									Computed:    true,
									Description: "The latitude of the POP.",
								},
								"longitude": schema.Float64Attribute{
									Computed:    true,
									Description: "The longitude of the POP.",
								},
								"x": schema.Float64Attribute{
									Computed:    true,
									Description: "The x coordinate of the POP, used for visualizing relative geographic distance.",
								},
								"y": schema.Float64Attribute{
									Computed:    true,
									Description: "The y coordinate of the POP, used for visualizing relative geographic distance.",
								},
							},
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

	tflog.Debug(ctx, "Reading Fastly Datacenters")

	datacenters, err := d.client.AllDatacenters(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Datacenters", err.Error())
		return
	}

	popSet, ids, diags := flattenDatacenters(datacenters)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Pops = popSet
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenDatacenters(datacenters []fastly.Datacenter) (types.Set, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := make([]string, 0, len(datacenters))
	elements := make([]attr.Value, 0, len(datacenters))

	for _, dc := range datacenters {
		ids = append(ids, fastly.ToValue(dc.Code))

		coordinates, coordinatesDiags := flattenCoordinates(dc.Coordinates)
		diags.Append(coordinatesDiags...)

		obj, objDiags := types.ObjectValue(popAttrTypes, map[string]attr.Value{
			"code":        service.StringPointerOrNull(dc.Code),
			"group":       service.StringPointerOrNull(dc.Group),
			"name":        service.StringPointerOrNull(dc.Name),
			"shield":      service.StringPointerOrNull(dc.Shield),
			"coordinates": coordinates,
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	setValue, setDiags := types.SetValue(
		types.ObjectType{AttrTypes: popAttrTypes},
		elements,
	)
	diags.Append(setDiags...)

	return setValue, ids, diags
}

func flattenCoordinates(c *fastly.Coordinates) (types.Object, diag.Diagnostics) {
	if c == nil {
		return types.ObjectNull(coordinatesAttrTypes), nil
	}

	return types.ObjectValue(coordinatesAttrTypes, map[string]attr.Value{
		"latitude":  service.Float64PointerOrNull(c.Latitude),
		"longitude": service.Float64PointerOrNull(c.Longitude),
		"x":         service.Float64PointerOrNull(c.X),
		"y":         service.Float64PointerOrNull(c.Y),
	})
}
