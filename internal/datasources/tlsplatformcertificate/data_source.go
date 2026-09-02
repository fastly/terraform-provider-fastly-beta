package tlsplatformcertificate

import (
	"context"
	"slices"
	"time"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	Domains         types.Set    `tfsdk:"domains"`
	ConfigurationID types.String `tfsdk:"configuration_id"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	NotBefore       types.String `tfsdk:"not_before"`
	NotAfter        types.String `tfsdk:"not_after"`
	Replace         types.Bool   `tfsdk:"replace"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_platform_certificate"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to get information about a Fastly Platform TLS certificate for use with other resources.\n\n" +
			"The filters are applied using an AND boolean operator, so depending on the combination of filters they may " +
			"become mutually exclusive. `id` must not be specified in combination with any of the others. The query must " +
			"return exactly one certificate; a query that returns zero or more than one certificate is an error.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Unique ID assigned to the certificate by Fastly. Conflicts with all the other filters.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("domains")),
				},
			},
			"domains": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Domains that are listed in any certificate's Subject Alternative Names (SAN) list.",
				Validators: []validator.Set{
					setvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"configuration_id": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the TLS configuration used to terminate TLS traffic.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when the certificate was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when the certificate was last updated.",
			},
			"not_before": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when the certificate will become valid. Must be in the past for the certificate to terminate TLS traffic.",
			},
			"not_after": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when the certificate will expire. Must be in the future for the certificate to terminate TLS traffic.",
			},
			"replace": schema.BoolAttribute{
				Computed:    true,
				Description: "A recommendation from Fastly indicating the key associated with this certificate is in need of rotation.",
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

	var certificate *fastly.BulkCertificate

	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		id := config.ID.ValueString()
		tflog.Debug(ctx, "Reading Fastly TLS platform certificate", map[string]any{"id": id})

		cert, err := d.client.GetBulkCertificate(ctx, &fastly.GetBulkCertificateInput{ID: id})
		if err != nil {
			resp.Diagnostics.AddError("Error reading TLS platform certificate", err.Error())
			return
		}
		certificate = cert
	} else {
		wantDomains, diags := domainsFilter(ctx, config.Domains)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		certificates, err := listAllBulkCertificates(ctx, d.client)
		if err != nil {
			resp.Diagnostics.AddError("Error listing TLS platform certificates", err.Error())
			return
		}

		var matches []*fastly.BulkCertificate
		for _, c := range certificates {
			if matchesDomains(c, wantDomains) {
				matches = append(matches, c)
			}
		}

		if len(matches) == 0 {
			resp.Diagnostics.AddError(
				"No matching TLS platform certificate found",
				"Your query returned no results. Please change your search criteria and try again.",
			)
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError(
				"Multiple matching TLS platform certificates found",
				"Your query returned more than one result. Please change your search criteria to be more specific and try again.",
			)
			return
		}

		certificate = matches[0]
	}

	if certificate.Replace {
		resp.Diagnostics.AddWarning(
			"Certificate recommended for replacement",
			"Fastly recommends that this certificate ("+certificate.ID+") be replaced.",
		)
	}

	state, diags := flattenToDataSourceModel(ctx, certificate)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// domainsFilter extracts the requested domains filter from a Domains
// config value. A null or unknown Domains (the filter left unset) yields no
// filter rather than an error - ElementsAs cannot convert an unknown Set,
// so unknown must be checked explicitly rather than relying on it to behave
// like null.
func domainsFilter(ctx context.Context, domains types.Set) ([]string, diag.Diagnostics) {
	if domains.IsNull() || domains.IsUnknown() {
		return nil, nil
	}

	var wantDomains []string
	diags := domains.ElementsAs(ctx, &wantDomains, false)
	return wantDomains, diags
}

// matchesDomains reports whether any of wantDomains is present, verbatim, in
// certificate's Subject Alternative Names. An empty wantDomains matches
// every certificate, mirroring the legacy provider's behavior when the
// domains filter is left unset.
func matchesDomains(certificate *fastly.BulkCertificate, wantDomains []string) bool {
	if len(wantDomains) == 0 {
		return true
	}
	for _, d := range certificate.Domains {
		if d != nil && slices.Contains(wantDomains, d.ID) {
			return true
		}
	}
	return false
}

// listAllBulkCertificates pages through every bulk certificate. The Fastly
// API paginates this endpoint and go-fastly does not auto-paginate it.
func listAllBulkCertificates(ctx context.Context, client *fastly.Client) ([]*fastly.BulkCertificate, error) {
	var all []*fastly.BulkCertificate
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
		all = append(all, page...)
		pageNumber++
	}
	return all, nil
}

func flattenToDataSourceModel(ctx context.Context, certificate *fastly.BulkCertificate) (DataSourceModel, diag.Diagnostics) {
	var configurationID string
	if len(certificate.Configurations) > 0 && certificate.Configurations[0] != nil {
		configurationID = certificate.Configurations[0].ID
	}

	domainNames := make([]string, 0, len(certificate.Domains))
	for _, d := range certificate.Domains {
		if d != nil {
			domainNames = append(domainNames, d.ID)
		}
	}
	domains, diags := types.SetValueFrom(ctx, types.StringType, domainNames)

	m := DataSourceModel{
		ID:              types.StringValue(certificate.ID),
		Domains:         domains,
		ConfigurationID: types.StringValue(configurationID),
		CreatedAt:       formatTime(certificate.CreatedAt),
		UpdatedAt:       formatTime(certificate.UpdatedAt),
		NotBefore:       formatTime(certificate.NotBefore),
		NotAfter:        formatTime(certificate.NotAfter),
		Replace:         types.BoolValue(certificate.Replace),
	}

	return m, diags
}

func formatTime(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
