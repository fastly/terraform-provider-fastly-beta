package tlsdomain

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	Domain             types.String `tfsdk:"domain"`
	TLSActivationIDs   types.Set    `tfsdk:"tls_activation_ids"`
	TLSCertificateIDs  types.Set    `tfsdk:"tls_certificate_ids"`
	TLSSubscriptionIDs types.Set    `tfsdk:"tls_subscription_ids"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_domain"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to look up activations, certificates, and subscriptions associated with a TLS domain.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier. Mirrors domain.",
			},
			"domain": schema.StringAttribute{
				Required:    true,
				Description: "Domain name to look up activations, certificates and subscriptions for.",
			},
			"tls_activation_ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IDs of the activations associated with the domain.",
			},
			"tls_certificate_ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IDs of the certificates associated with the domain.",
			},
			"tls_subscription_ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IDs of the subscriptions associated with the domain.",
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

	domainName := service.StringValue(config.Domain)

	tflog.Debug(ctx, "Listing Fastly TLS domains to find a match", map[string]any{"domain": domainName})

	domains, err := listDomains(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing TLS domains", err.Error())
		return
	}

	var matches []*fastly.TLSDomain
	for _, dom := range domains {
		if dom.ID == domainName {
			matches = append(matches, dom)
		}
	}

	var domain *fastly.TLSDomain
	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError("No matching TLS domain found", "your query returned no results. Please change your search criteria and try again")
		return
	case 1:
		domain = matches[0]
	default:
		resp.Diagnostics.AddError("Multiple matching TLS domains found", "your query returned more than one result. Please change to a more specific search criteria")
		return
	}

	state, diags := flattenToModel(ctx, domain)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func listDomains(ctx context.Context, client *fastly.Client) ([]*fastly.TLSDomain, error) {
	var domains []*fastly.TLSDomain
	pageNumber := 1
	for {
		list, err := client.ListTLSDomains(ctx, &fastly.ListTLSDomainsInput{
			PageNumber: pageNumber,
		})
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}
		pageNumber++
		domains = append(domains, list...)
	}
	return domains, nil
}

func flattenToModel(ctx context.Context, domain *fastly.TLSDomain) (DataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	var activationIDs, certificateIDs, subscriptionIDs []string
	for _, activation := range domain.Activations {
		activationIDs = append(activationIDs, activation.ID)
	}
	for _, certificate := range domain.Certificates {
		certificateIDs = append(certificateIDs, certificate.ID)
	}
	for _, subscription := range domain.Subscriptions {
		subscriptionIDs = append(subscriptionIDs, subscription.ID)
	}

	activationSet, d := types.SetValueFrom(ctx, types.StringType, activationIDs)
	diags.Append(d...)
	certificateSet, d := types.SetValueFrom(ctx, types.StringType, certificateIDs)
	diags.Append(d...)
	subscriptionSet, d := types.SetValueFrom(ctx, types.StringType, subscriptionIDs)
	diags.Append(d...)

	return DataSourceModel{
		ID:                 types.StringValue(domain.ID),
		Domain:             types.StringValue(domain.ID),
		TLSActivationIDs:   activationSet,
		TLSCertificateIDs:  certificateSet,
		TLSSubscriptionIDs: subscriptionSet,
	}, diags
}
