package stagingips

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

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	providerData *fastlyclient.Data
}

type DataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	ServiceID      types.String `tfsdk:"service_id"`
	ServiceVersion types.Int64  `tfsdk:"service_version"`
	Domains        types.Set    `tfsdk:"domains"`
}

var domainAttrTypes = map[string]attr.Type{
	"name":       types.StringType,
	"staging_ip": types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_staging_ips"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve the staging IP addresses assigned to a Fastly service version's domains.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"service_id": schema.StringAttribute{
				Required:    true,
				Description: "Alphanumeric string identifying the service.",
			},
			"service_version": schema.Int64Attribute{
				Required:    true,
				Description: "Integer identifying a service version.",
			},
			"domains": schema.SetNestedAttribute{
				Computed:    true,
				Description: "List of domains with their staging IP addresses.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The domain name.",
						},
						"staging_ip": schema.StringAttribute{
							Computed:    true,
							Description: "The staging IP address for the domain.",
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

	d.providerData = data
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	version := int(state.ServiceVersion.ValueInt64())

	tflog.Debug(ctx, "Reading Fastly staging IPs", map[string]any{
		"service_id":      serviceID,
		"service_version": version,
	})

	domains, err := d.providerData.Client.ListDomains(ctx, &fastly.ListDomainsInput{
		ServiceID:         serviceID,
		ServiceVersion:    version,
		IncludeStagingIPs: true,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading staging IPs", err.Error())
		return
	}

	setVal, ids, diags := flattenStagingIPs(domains)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Domains = setVal
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenStagingIPs(domains []*fastly.Domain) (types.Set, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := make([]string, 0, len(domains))
	elements := make([]attr.Value, 0, len(domains))

	for _, dom := range domains {
		if dom == nil {
			continue
		}

		name := fastly.ToValue(dom.Name)
		stagingIP := fastly.ToValue(dom.StagingIP)

		ids = append(ids, name+"/"+stagingIP)

		obj, objDiags := types.ObjectValue(domainAttrTypes, map[string]attr.Value{
			"name":       types.StringValue(name),
			"staging_ip": types.StringValue(stagingIP),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: domainAttrTypes}), nil, diags
	}

	setVal, setDiags := types.SetValue(types.ObjectType{AttrTypes: domainAttrTypes}, elements)
	diags.Append(setDiags...)

	return setVal, ids, diags
}
