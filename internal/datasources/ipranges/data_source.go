package ipranges

import (
	"context"
	"sort"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	CIDRBlocks     types.List   `tfsdk:"cidr_blocks"`
	IPv6CIDRBlocks types.List   `tfsdk:"ipv6_cidr_blocks"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_ranges"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to get the list of Fastly IP ranges.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"cidr_blocks": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The lexically ordered list of ipv4 CIDR blocks.",
			},
			"ipv6_cidr_blocks": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The lexically ordered list of ipv6 CIDR blocks.",
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

	tflog.Debug(ctx, "Reading Fastly IP ranges")

	ipv4, ipv6, err := d.client.AllIPs(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing IP ranges", err.Error())
		return
	}

	cidrBlocks := append([]string(nil), ipv4...)
	ipv6CIDRBlocks := append([]string(nil), ipv6...)
	sort.Strings(cidrBlocks)
	sort.Strings(ipv6CIDRBlocks)

	cidrBlocksList, diags := types.ListValueFrom(ctx, types.StringType, cidrBlocks)
	resp.Diagnostics.Append(diags...)
	ipv6CIDRBlocksList, diags := types.ListValueFrom(ctx, types.StringType, ipv6CIDRBlocks)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(idhash.HashIDs(append(cidrBlocks, ipv6CIDRBlocks...)))
	state.CIDRBlocks = cidrBlocksList
	state.IPv6CIDRBlocks = ipv6CIDRBlocksList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
