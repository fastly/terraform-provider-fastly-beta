package tlsconfiguration

import (
	"context"
	"fmt"
	"slices"
	"time"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

const (
	tlsServicePlatform = "PLATFORM"
	tlsServiceCustom   = "CUSTOM"
)

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	TLSProtocols  types.Set    `tfsdk:"tls_protocols"`
	HTTPProtocols types.Set    `tfsdk:"http_protocols"`
	TLSService    types.String `tfsdk:"tls_service"`
	Default       types.Bool   `tfsdk:"default"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
	DNSRecords    types.Set    `tfsdk:"dns_records"`
}

var dnsRecordAttrTypes = map[string]attr.Type{
	"record_type":  types.StringType,
	"record_value": types.StringType,
	"region":       types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_configuration"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to get the ID of a TLS configuration for use with other resources.\n\n" +
			"The data source's filters are applied using an AND boolean operator, so depending on the combination " +
			"of filters, they may become mutually exclusive. The exception to this is `id`, which must not be " +
			"specified in combination with any of the others.\n\n" +
			"If more or less than a single match is returned by the search, an error is raised.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the TLS configuration obtained from the Fastly API or another data source. Conflicts with all the other filters.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("name"),
						path.MatchRoot("tls_protocols"),
						path.MatchRoot("http_protocols"),
						path.MatchRoot("tls_service"),
						path.MatchRoot("default"),
					),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Custom name of the TLS configuration.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"tls_protocols": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "TLS protocols available on the TLS configuration.",
				Validators: []validator.Set{
					setvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"http_protocols": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "HTTP protocols available on the TLS configuration.",
				Validators: []validator.Set{
					setvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"tls_service": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: fmt.Sprintf("Whether the configuration should support the `%s` or `%s` TLS service.", tlsServicePlatform, tlsServiceCustom),
				Validators: []validator.String{
					stringvalidator.OneOf(tlsServicePlatform, tlsServiceCustom),
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"default": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Signifies whether Fastly will use this configuration as a default when creating a new TLS activation.",
				Validators: []validator.Bool{
					boolvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when the configuration was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (GMT) when the configuration was last updated.",
			},
			"dns_records": schema.SetNestedAttribute{
				Computed: true,
				Description: "The available DNS addresses that can be used to enable TLS for a domain. DNS must be " +
					"configured for a domain for TLS handshakes to succeed. If enabling TLS on an apex domain (e.g. " +
					"`example.com`) you must create four A records (or four AAAA records for IPv6 support) using the " +
					"displayed global A record's IP addresses with your DNS provider. For subdomains and wildcard " +
					"domains (e.g. `www.example.com` or `*.example.com`) you will need to create a relevant CNAME record.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"record_type": schema.StringAttribute{
							Computed:    true,
							Description: "Type of DNS record to set, e.g. A, AAAA, or CNAME.",
						},
						"record_value": schema.StringAttribute{
							Computed:    true,
							Description: "The IP address or hostname of the DNS record.",
						},
						"region": schema.StringAttribute{
							Computed: true,
							Description: "The regions that will be used to route traffic. Select DNS records with a " +
								"`global` region to route traffic to the most performant point of presence (POP) " +
								"worldwide (global pricing will apply). Select DNS records with a `us-eu` region to " +
								"exclusively land traffic on North American and European POPs.",
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

	tflog.Debug(ctx, "Reading Fastly TLS configuration")

	var configuration *fastly.CustomTLSConfiguration

	if !state.ID.IsNull() && !state.ID.IsUnknown() && state.ID.ValueString() != "" {
		config, err := d.client.GetCustomTLSConfiguration(ctx, &fastly.GetCustomTLSConfigurationInput{
			ID:      state.ID.ValueString(),
			Include: "dns_records",
		})
		if err != nil {
			resp.Diagnostics.AddError("Error reading Fastly TLS configuration", err.Error())
			return
		}
		configuration = config
	} else {
		configurations, err := listCustomTLSConfigurations(ctx, d.client)
		if err != nil {
			resp.Diagnostics.AddError("Error listing Fastly TLS configurations", err.Error())
			return
		}

		matches, diags := filterConfigurations(ctx, configurations, &state)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		if len(matches) == 0 {
			resp.Diagnostics.AddError(
				"No matching Fastly TLS configuration found",
				"Your query returned no results. Please change your search criteria and try again.",
			)
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError(
				"Multiple matching Fastly TLS configurations found",
				"Your query returned more than one result. Please use more specific search criteria and try again.",
			)
			return
		}

		configuration = matches[0]
	}

	resp.Diagnostics.Append(flattenConfiguration(ctx, configuration, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func listCustomTLSConfigurations(ctx context.Context, client *fastly.Client) ([]*fastly.CustomTLSConfiguration, error) {
	var configurations []*fastly.CustomTLSConfiguration

	for page := 1; ; page++ {
		list, err := client.ListCustomTLSConfigurations(ctx, &fastly.ListCustomTLSConfigurationsInput{
			PageNumber: page,
			Include:    "dns_records",
		})
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}
		configurations = append(configurations, list...)
	}

	return configurations, nil
}

func filterConfigurations(ctx context.Context, configurations []*fastly.CustomTLSConfiguration, state *DataSourceModel) ([]*fastly.CustomTLSConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics

	var tlsProtocols, httpProtocols []string
	if !state.TLSProtocols.IsNull() && !state.TLSProtocols.IsUnknown() {
		diags.Append(state.TLSProtocols.ElementsAs(ctx, &tlsProtocols, false)...)
	}
	if !state.HTTPProtocols.IsNull() && !state.HTTPProtocols.IsUnknown() {
		diags.Append(state.HTTPProtocols.ElementsAs(ctx, &httpProtocols, false)...)
	}
	if diags.HasError() {
		return nil, diags
	}

	hasName := !state.Name.IsNull() && !state.Name.IsUnknown()
	name := state.Name.ValueString()
	hasTLSService := !state.TLSService.IsNull() && !state.TLSService.IsUnknown()
	tlsService := state.TLSService.ValueString()
	hasDefault := !state.Default.IsNull() && !state.Default.IsUnknown()
	defaultVal := state.Default.ValueBool()

	var matches []*fastly.CustomTLSConfiguration
	for _, c := range configurations {
		if hasName && c.Name != name {
			continue
		}
		if len(tlsProtocols) > 0 && !containsAll(c.TLSProtocols, tlsProtocols) {
			continue
		}
		if len(httpProtocols) > 0 && !containsAll(c.HTTPProtocols, httpProtocols) {
			continue
		}
		if hasTLSService && c.Bulk != (tlsService == tlsServicePlatform) {
			continue
		}
		if hasDefault && c.Default != defaultVal {
			continue
		}
		matches = append(matches, c)
	}

	return matches, diags
}

func containsAll(haystack, needles []string) bool {
	for _, needle := range needles {
		if !slices.Contains(haystack, needle) {
			return false
		}
	}
	return true
}

func flattenConfiguration(ctx context.Context, configuration *fastly.CustomTLSConfiguration, state *DataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	tlsService := tlsServiceCustom
	if configuration.Bulk {
		tlsService = tlsServicePlatform
	}

	elements := make([]attr.Value, 0, len(configuration.DNSRecords))
	for _, record := range configuration.DNSRecords {
		if record == nil {
			continue
		}
		obj, objDiags := types.ObjectValue(dnsRecordAttrTypes, map[string]attr.Value{
			"record_type":  types.StringValue(record.RecordType),
			"record_value": types.StringValue(record.ID),
			"region":       types.StringValue(record.Region),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	dnsRecordsSet, setDiags := types.SetValue(types.ObjectType{AttrTypes: dnsRecordAttrTypes}, elements)
	diags.Append(setDiags...)

	tlsProtocolsSet, tlsDiags := types.SetValueFrom(ctx, types.StringType, configuration.TLSProtocols)
	diags.Append(tlsDiags...)

	httpProtocolsSet, httpDiags := types.SetValueFrom(ctx, types.StringType, configuration.HTTPProtocols)
	diags.Append(httpDiags...)

	if diags.HasError() {
		return diags
	}

	state.ID = types.StringValue(configuration.ID)
	state.Name = types.StringValue(configuration.Name)
	state.TLSProtocols = tlsProtocolsSet
	state.HTTPProtocols = httpProtocolsSet
	state.TLSService = types.StringValue(tlsService)
	state.Default = types.BoolValue(configuration.Default)
	state.DNSRecords = dnsRecordsSet

	state.CreatedAt = types.StringNull()
	if configuration.CreatedAt != nil {
		state.CreatedAt = types.StringValue(configuration.CreatedAt.Format(time.RFC3339))
	}

	state.UpdatedAt = types.StringNull()
	if configuration.UpdatedAt != nil {
		state.UpdatedAt = types.StringValue(configuration.UpdatedAt.Format(time.RFC3339))
	}

	return diags
}
