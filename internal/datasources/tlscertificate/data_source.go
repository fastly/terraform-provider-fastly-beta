package tlscertificate

import (
	"context"
	"slices"
	"time"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

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
	ID                 types.String `tfsdk:"id"`
	CreatedAt          types.String `tfsdk:"created_at"`
	Domains            types.Set    `tfsdk:"domains"`
	IssuedTo           types.String `tfsdk:"issued_to"`
	Issuer             types.String `tfsdk:"issuer"`
	Name               types.String `tfsdk:"name"`
	Replace            types.Bool   `tfsdk:"replace"`
	SerialNumber       types.String `tfsdk:"serial_number"`
	SignatureAlgorithm types.String `tfsdk:"signature_algorithm"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_certificate"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to get the details of a TLS certificate, by ID or by other lookup criteria.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Unique ID assigned to certificate by Fastly.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("name"),
						path.MatchRoot("issued_to"),
						path.MatchRoot("domains"),
						path.MatchRoot("issuer"),
					),
				},
			},
			"domains": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Domains that are listed in any certificates' Subject Alternative Names (SAN) list.",
				Validators: []validator.Set{
					setvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"issued_to": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The hostname for which a certificate was issued.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"issuer": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The certificate authority that issued the certificate.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Human-readable name used to identify the certificate. Defaults to the certificate's Common Name or first Subject Alternative Name entry.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when the certificate was created.",
			},
			"replace": schema.BoolAttribute{
				Computed:    true,
				Description: "A recommendation from Fastly indicating the key associated with this certificate is in need of rotation.",
			},
			"serial_number": schema.StringAttribute{
				Computed:    true,
				Description: "A value assigned by the issuer that is unique to a certificate.",
			},
			"signature_algorithm": schema.StringAttribute{
				Computed:    true,
				Description: "The algorithm used to sign the certificate.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when the certificate was last updated.",
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

	var domains []string
	resp.Diagnostics.Append(config.Domains.ElementsAs(ctx, &domains, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var certificate *fastly.CustomTLSCertificate

	if id := service.StringValue(config.ID); id != "" {
		tflog.Debug(ctx, "Reading Fastly TLS certificate", map[string]any{"id": id})

		found, err := d.client.GetCustomTLSCertificate(ctx, &fastly.GetCustomTLSCertificateInput{ID: id})
		if err != nil {
			resp.Diagnostics.AddError("Error reading TLS certificate", err.Error())
			return
		}
		certificate = found
	} else {
		tflog.Debug(ctx, "Listing Fastly TLS certificates to find a match")

		certificates, err := listCertificates(ctx, d.client)
		if err != nil {
			resp.Diagnostics.AddError("Error listing TLS certificates", err.Error())
			return
		}

		matches := filterCertificates(certificates, config, domains)
		switch len(matches) {
		case 0:
			resp.Diagnostics.AddError("No matching TLS certificate found", "your query returned no results. Please change your search criteria and try again")
			return
		case 1:
			certificate = matches[0]
		default:
			resp.Diagnostics.AddError("Multiple matching TLS certificates found", "your query returned more than one result. Please change to a more specific search criteria")
			return
		}
	}

	if certificate.Replace {
		resp.Diagnostics.AddWarning("Certificate replacement recommended", "Fastly recommends that this certificate ("+certificate.ID+") be replaced")
	}

	state, diags := flattenToModel(ctx, certificate)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func listCertificates(ctx context.Context, client *fastly.Client) ([]*fastly.CustomTLSCertificate, error) {
	var certificates []*fastly.CustomTLSCertificate
	pageNumber := 1
	for {
		list, err := client.ListCustomTLSCertificates(ctx, &fastly.ListCustomTLSCertificatesInput{
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
		certificates = append(certificates, list...)
	}
	return certificates, nil
}

func filterCertificates(certificates []*fastly.CustomTLSCertificate, config DataSourceModel, domains []string) []*fastly.CustomTLSCertificate {
	name := service.StringValue(config.Name)
	issuedTo := service.StringValue(config.IssuedTo)
	issuer := service.StringValue(config.Issuer)

	var matches []*fastly.CustomTLSCertificate
	for _, c := range certificates {
		if name != "" && c.Name != name {
			continue
		}
		if issuedTo != "" && c.IssuedTo != issuedTo {
			continue
		}
		if issuer != "" && c.Issuer != issuer {
			continue
		}
		if len(domains) > 0 && !certificateHasAnyDomain(c, domains) {
			continue
		}
		matches = append(matches, c)
	}
	return matches
}

func certificateHasAnyDomain(c *fastly.CustomTLSCertificate, domains []string) bool {
	for _, d := range c.Domains {
		if slices.Contains(domains, d.ID) {
			return true
		}
	}
	return false
}

func flattenToModel(ctx context.Context, c *fastly.CustomTLSCertificate) (DataSourceModel, diag.Diagnostics) {
	domains := make([]string, len(c.Domains))
	for i, d := range c.Domains {
		domains[i] = d.ID
	}
	domainsSet, diags := types.SetValueFrom(ctx, types.StringType, domains)

	m := DataSourceModel{
		ID:                 types.StringValue(c.ID),
		Domains:            domainsSet,
		IssuedTo:           types.StringValue(c.IssuedTo),
		Issuer:             types.StringValue(c.Issuer),
		Name:               types.StringValue(c.Name),
		Replace:            types.BoolValue(c.Replace),
		SerialNumber:       types.StringValue(c.SerialNumber),
		SignatureAlgorithm: types.StringValue(c.SignatureAlgorithm),
		CreatedAt:          types.StringNull(),
		UpdatedAt:          types.StringNull(),
	}

	if c.CreatedAt != nil {
		m.CreatedAt = types.StringValue(c.CreatedAt.Format(time.RFC3339))
	}
	if c.UpdatedAt != nil {
		m.UpdatedAt = types.StringValue(c.UpdatedAt.Format(time.RFC3339))
	}

	return m, diags
}
