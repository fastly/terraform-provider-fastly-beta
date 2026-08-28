package configstores

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"

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
	ID     types.String `tfsdk:"id"`
	Stores types.Set    `tfsdk:"stores"`
}

var storeAttrTypes = map[string]attr.Type{
	"id":   types.StringType,
	"name": types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_configstores"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve all Fastly Config Stores available to the account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Stable Terraform data source identifier derived from the returned Config Store IDs.",
			},
			"stores": schema.SetNestedAttribute{
				Computed:    true,
				Description: "The Config Stores available to the account. Set semantics are used because the Fastly API does not guarantee list ordering.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "An alphanumeric string identifying the Config Store.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the Config Store.",
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

	tflog.Debug(ctx, "Reading Fastly Config Stores")

	stores, err := d.client.ListConfigStores(ctx, &fastly.ListConfigStoresInput{})
	if err != nil {
		resp.Diagnostics.AddError("Error listing Config Stores", err.Error())
		return
	}

	storeSet, ids, diags := flattenStores(stores)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Stores = storeSet
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenStores(stores []*fastly.ConfigStore) (types.Set, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := make([]string, 0, len(stores))
	elements := make([]attr.Value, 0, len(stores))

	for _, store := range stores {
		if store == nil {
			continue
		}

		ids = append(ids, store.StoreID)

		obj, objDiags := types.ObjectValue(storeAttrTypes, map[string]attr.Value{
			"id":   types.StringValue(store.StoreID),
			"name": types.StringValue(store.Name),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	setValue, setDiags := types.SetValue(
		types.ObjectType{AttrTypes: storeAttrTypes},
		elements,
	)
	diags.Append(setDiags...)

	return setValue, ids, diags
}
