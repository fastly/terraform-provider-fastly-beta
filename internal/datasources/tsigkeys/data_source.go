package tsigkeys

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
	"github.com/fastly/go-fastly/v17/fastly/dns/v1/tsigkeys"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Total types.Int64  `tfsdk:"total"`
	Keys  types.Set    `tfsdk:"keys"`
}

var keyAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
	"algorithm":   types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tsig_keys"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly TSIG keys.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"total": schema.Int64Attribute{
				Computed:    true,
				Description: "The total number of TSIG keys returned.",
			},
			"keys": schema.SetNestedAttribute{
				Computed:    true,
				Description: "A list of TSIG keys.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "TSIG Key Identifier (UUID).",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the TSIG key.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "A freeform descriptive note.",
						},
						"algorithm": schema.StringAttribute{
							Computed:    true,
							Description: "The algorithm of the TSIG key.",
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

	tflog.Debug(ctx, "Reading Fastly TSIG Keys")

	keys, err := tsigkeys.List(ctx, d.client, &tsigkeys.ListInput{})
	if err != nil {
		resp.Diagnostics.AddError("Error listing TSIG Keys", err.Error())
		return
	}

	ids := make([]string, 0, len(keys))
	elements := make([]attr.Value, 0, len(keys))
	for _, key := range keys {
		ids = append(ids, fastly.ToValue(key.ID))

		obj, diags := types.ObjectValue(keyAttrTypes, map[string]attr.Value{
			"id":          service.StringPointerOrNull(key.ID),
			"name":        service.StringPointerOrNull(key.Name),
			"description": service.StringPointerOrNull(key.Description),
			"algorithm":   service.StringPointerOrNull(key.Algorithm),
		})
		resp.Diagnostics.Append(diags...)
		elements = append(elements, obj)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	setVal, diags := types.SetValue(types.ObjectType{AttrTypes: keyAttrTypes}, elements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Keys = setVal
	state.Total = types.Int64Value(int64(len(keys)))
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
