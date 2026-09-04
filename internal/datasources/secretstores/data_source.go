package secretstores

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
	resp.TypeName = req.ProviderTypeName + "_secretstores"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve all Fastly Secret Stores available to the account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Stable Terraform data source identifier derived from the returned Secret Store IDs.",
			},
			"stores": schema.SetNestedAttribute{
				Computed:    true,
				Description: "The Secret Stores available to the account. Set semantics are used because the Fastly API does not guarantee list ordering.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "An alphanumeric string identifying the Secret Store.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the Secret Store.",
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

	tflog.Debug(ctx, "Reading Fastly Secret Stores")

	stores, err := listAllSecretStores(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Secret Stores", err.Error())
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

// listAllSecretStores pages through every Secret Store; unlike Config/KV Stores, the API
// only returns this list a cursor-page at a time.
func listAllSecretStores(ctx context.Context, client *fastly.Client) ([]fastly.SecretStore, error) {
	var (
		cursor string
		stores []fastly.SecretStore
	)

	for {
		page, err := client.ListSecretStores(ctx, &fastly.ListSecretStoresInput{
			Cursor: cursor,
		})
		if err != nil {
			return nil, err
		}

		stores = append(stores, page.Data...)

		next := page.Meta.NextCursor
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}

	return stores, nil
}

func flattenStores(stores []fastly.SecretStore) (types.Set, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := make([]string, 0, len(stores))
	elements := make([]attr.Value, 0, len(stores))

	for _, store := range stores {
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
