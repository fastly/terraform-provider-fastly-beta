package tlsactivationids

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

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
	ID            types.String `tfsdk:"id"`
	CertificateID types.String `tfsdk:"certificate_id"`
	IDs           types.Set    `tfsdk:"ids"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_activation_ids"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to get the IDs of available TLS activations, optionally filtered by certificate.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"certificate_id": schema.StringAttribute{
				Optional:    true,
				Description: "ID of TLS certificate used to filter activations.",
			},
			"ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of IDs of the TLS Activations.",
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

	certificateID := service.StringValue(state.CertificateID)
	tflog.Debug(ctx, "Listing Fastly TLS activation IDs", map[string]any{"certificate_id": certificateID})

	var ids []string
	pageNumber := 1
	for {
		list, err := d.client.ListTLSActivations(ctx, &fastly.ListTLSActivationsInput{
			FilterTLSCertificateID: certificateID,
			PageNumber:             pageNumber,
			PageSize:               10,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error listing TLS activations", err.Error())
			return
		}
		if len(list) == 0 {
			break
		}
		pageNumber++

		for _, activation := range list {
			ids = append(ids, activation.ID)
		}
	}

	idsSet, diags := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.IDs = idsSet
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
