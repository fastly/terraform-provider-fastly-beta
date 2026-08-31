package dnszones

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/dns/v1/dnszones"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Total types.Int64  `tfsdk:"total"`
	Zones types.Set    `tfsdk:"zones"`
}

var zoneAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zones"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly DNS zones.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"total": schema.Int64Attribute{
				Computed:    true,
				Description: "The total number of DNS zones returned.",
			},
			"zones": schema.SetNestedAttribute{
				Computed:    true,
				Description: "A list of DNS zones.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Zone Identifier.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The domain name for the zone.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "A freeform descriptive note.",
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

	tflog.Debug(ctx, "Reading Fastly DNS Zones")

	zones, err := dnszones.List(ctx, d.client, &dnszones.ListInput{})
	if err != nil {
		resp.Diagnostics.AddError("Error listing DNS Zones", err.Error())
		return
	}

	ids := make([]string, 0, len(zones))
	elements := make([]attr.Value, 0, len(zones))
	for _, zone := range zones {
		ids = append(ids, fastly.ToValue(zone.ID))

		obj, diags := types.ObjectValue(zoneAttrTypes, map[string]attr.Value{
			"id":          service.StringPointerOrNull(zone.ID),
			"name":        service.StringPointerOrNull(zone.Name),
			"description": service.StringPointerOrNull(zone.Description),
		})
		resp.Diagnostics.Append(diags...)
		elements = append(elements, obj)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	setVal, diags := types.SetValue(types.ObjectType{AttrTypes: zoneAttrTypes}, elements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Zones = setVal
	state.Total = types.Int64Value(int64(len(zones)))
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
