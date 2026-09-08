package domains

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
	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Total   types.Int64  `tfsdk:"total"`
	Domains types.Set    `tfsdk:"domains"`
}

var domainAttrTypes = map[string]attr.Type{
	"id":         types.StringType,
	"fqdn":       types.StringType,
	"service_id": types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly domains.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"total": schema.Int64Attribute{
				Computed:    true,
				Description: "The total number of domains returned.",
			},
			"domains": schema.SetNestedAttribute{
				Computed:    true,
				Description: "A domain represents the domain name through which visitors will retrieve content. There can be multiple domains for a service.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Domain Identifier (UUID).",
						},
						"fqdn": schema.StringAttribute{
							Computed:    true,
							Description: "The fully-qualified domain name for your domain.",
						},
						"service_id": schema.StringAttribute{
							Computed:    true,
							Description: "The `service_id` associated with your domain or `null` if there is no association.",
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

	tflog.Debug(ctx, "Reading Fastly Domains")

	var all []domains.Data
	var cursor *string
	limit := 100
	for {
		page, err := domains.List(ctx, d.client, &domains.ListInput{Cursor: cursor, Limit: &limit})
		if err != nil {
			resp.Diagnostics.AddError("Error listing Domains", err.Error())
			return
		}
		all = append(all, page.Data...)

		if page.Meta.NextCursor == "" {
			break
		}
		cursor = &page.Meta.NextCursor
	}

	ids := make([]string, 0, len(all))
	elements := make([]attr.Value, 0, len(all))
	for _, dom := range all {
		if dom.FQDN == "" {
			continue
		}
		ids = append(ids, dom.DomainID)

		obj, diags := types.ObjectValue(domainAttrTypes, map[string]attr.Value{
			"id":         types.StringValue(dom.DomainID),
			"fqdn":       types.StringValue(dom.FQDN),
			"service_id": service.StringPointerOrNull(dom.ServiceID),
		})
		resp.Diagnostics.Append(diags...)
		elements = append(elements, obj)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	setVal, diags := types.SetValue(types.ObjectType{AttrTypes: domainAttrTypes}, elements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Domains = setVal
	state.Total = types.Int64Value(int64(len(elements)))
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
