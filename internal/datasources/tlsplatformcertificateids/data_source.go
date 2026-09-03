package tlsplatformcertificateids

import (
	"context"

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
	ID  types.String `tfsdk:"id"`
	IDs types.Set    `tfsdk:"ids"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_platform_certificate_ids"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to get the IDs of available Fastly Platform TLS certificates, for use with `fastly_tls_platform_certificate`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"ids": schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "IDs of every Platform TLS certificate.",
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

	tflog.Debug(ctx, "Listing Fastly TLS platform certificate IDs")

	ids, err := listBulkCertificateIDs(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing TLS platform certificates", err.Error())
		return
	}

	idSet, diags := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.IDs = idSet
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// listBulkCertificateIDs pages through every bulk certificate. The Fastly
// API paginates this endpoint and go-fastly does not auto-paginate it.
func listBulkCertificateIDs(ctx context.Context, client *fastly.Client) ([]string, error) {
	var ids []string
	pageNumber := 1
	for {
		page, err := client.ListBulkCertificates(ctx, &fastly.ListBulkCertificatesInput{
			PageNumber: pageNumber,
			PageSize:   50,
		})
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		for _, certificate := range page {
			ids = append(ids, certificate.ID)
		}
		pageNumber++
	}
	return ids, nil
}
