package tlsactivation

import (
	"context"
	"time"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	CertificateID   types.String `tfsdk:"certificate_id"`
	ConfigurationID types.String `tfsdk:"configuration_id"`
	CreatedAt       types.String `tfsdk:"created_at"`
	Domain          types.String `tfsdk:"domain"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_activation"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to get the ID of a TLS activation, or to look up the certificate, configuration, or domain of an existing activation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Fastly Activation ID. Conflicts with all other filters.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("certificate_id"),
						path.MatchRoot("configuration_id"),
						path.MatchRoot("domain"),
					),
				},
			},
			"certificate_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the TLS Certificate used.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"configuration_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the TLS Configuration used.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"domain": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Domain that TLS was enabled on.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when TLS was enabled.",
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
	var config DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var activation *fastly.TLSActivation

	if id := service.StringValue(config.ID); id != "" {
		tflog.Debug(ctx, "Reading Fastly TLS activation", map[string]any{"id": id})

		found, err := d.client.GetTLSActivation(ctx, &fastly.GetTLSActivationInput{ID: id})
		if err != nil {
			resp.Diagnostics.AddError("Error reading TLS activation", err.Error())
			return
		}
		activation = found
	} else {
		tflog.Debug(ctx, "Listing Fastly TLS activations to find a match")

		activations, err := listActivations(ctx, d.client)
		if err != nil {
			resp.Diagnostics.AddError("Error listing TLS activations", err.Error())
			return
		}

		matches := filterActivations(activations, config)
		switch len(matches) {
		case 0:
			resp.Diagnostics.AddError("No matching TLS activation found", "your query returned no results. Please change your search criteria and try again")
			return
		case 1:
			activation = matches[0]
		default:
			resp.Diagnostics.AddError("Multiple matching TLS activations found", "your query returned more than one result. Please change to a more specific search criteria")
			return
		}
	}

	state := flattenToModel(activation)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func listActivations(ctx context.Context, client *fastly.Client) ([]*fastly.TLSActivation, error) {
	var activations []*fastly.TLSActivation
	pageNumber := 1
	for {
		list, err := client.ListTLSActivations(ctx, &fastly.ListTLSActivationsInput{
			PageNumber: pageNumber,
			PageSize:   10,
		})
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}
		pageNumber++
		activations = append(activations, list...)
	}
	return activations, nil
}

func filterActivations(activations []*fastly.TLSActivation, config DataSourceModel) []*fastly.TLSActivation {
	certID := service.StringValue(config.CertificateID)
	configID := service.StringValue(config.ConfigurationID)
	domain := service.StringValue(config.Domain)

	var matches []*fastly.TLSActivation
	for _, a := range activations {
		if certID != "" && (a.Certificate == nil || a.Certificate.ID != certID) {
			continue
		}
		if configID != "" && (a.Configuration == nil || a.Configuration.ID != configID) {
			continue
		}
		if domain != "" && (a.Domain == nil || a.Domain.ID != domain) {
			continue
		}
		matches = append(matches, a)
	}
	return matches
}

func flattenToModel(a *fastly.TLSActivation) DataSourceModel {
	m := DataSourceModel{
		ID:              types.StringValue(a.ID),
		CertificateID:   types.StringNull(),
		ConfigurationID: types.StringNull(),
		CreatedAt:       types.StringNull(),
		Domain:          types.StringNull(),
	}

	if a.Certificate != nil {
		m.CertificateID = types.StringValue(a.Certificate.ID)
	}
	if a.Configuration != nil {
		m.ConfigurationID = types.StringValue(a.Configuration.ID)
	}
	if a.Domain != nil {
		m.Domain = types.StringValue(a.Domain.ID)
	}
	if a.CreatedAt != nil {
		m.CreatedAt = types.StringValue(a.CreatedAt.Format(time.RFC3339))
	}

	return m
}
