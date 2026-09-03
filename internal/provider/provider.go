package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/actions/computepackageupload"
	"github.com/fastly/terraform-provider-fastly-beta/internal/actions/versionactivate"
	"github.com/fastly/terraform-provider-fastly-beta/internal/actions/versionclone"
	"github.com/fastly/terraform-provider-fastly-beta/internal/actions/versionstage"
	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/acls"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/configstores"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/dnszones"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/domains"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/kvstores"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafaccountrules"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwaflists"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafsignals"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacealertdatadogintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacealertjiraintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacealertmailinglistintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacealertmicrosoftteamsintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacealertopsgenieintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacealertpagerdutyintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacealertslackintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacealertwebhookintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacelists"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspaceredactions"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacerules"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspaces"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacesignals"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacethresholds"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacevirtualpatches"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/serviceversion"
	tlsactivationdatasource "github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlsactivation"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlsactivationids"
	tlscertificatedatasource "github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlscertificate"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlscertificateids"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlsconfiguration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlsconfigurationids"
	tlsplatformcertificatedatasource "github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlsplatformcertificate"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlsplatformcertificateids"
	tlsprivatekeydatasource "github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlsprivatekey"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/tlsprivatekeyids"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/vclsnippets"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/acl"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/aclentries"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/alert"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/apisecurityoperation"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/apisecurityoperationtag"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/backend"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/cdnacl"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/cdnaclentries"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/condition"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/configstore"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/customdashboard"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/dnszone"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/domain"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/domainmanagement"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/domainservicelink"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/dynamicsnippetcontent"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/dynamicvclsnippet"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/integration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/kvstore"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingbigquery"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingblobstorage"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingdatadog"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginggcs"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginghttps"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingnewrelic"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingnewrelicotlp"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggings3"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsplunk"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsumologic"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsyslog"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafcountrylist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafiplist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrequestrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafsignal"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafsignallist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafsignalrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafstringlist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafwildcardlist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspace"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertdatadogintegration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertjiraintegration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertmailinglistintegration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertmicrosoftteamsintegration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertopsgenieintegration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertpagerdutyintegration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertslackintegration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertwebhookintegration"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacecountrylist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspaceiplist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspaceratelimitrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspaceredaction"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacerequestrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacesignal"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacesignallist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacesignalrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacestringlist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacetemplatedsignalrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacethreshold"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacevirtualpatch"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacewildcardlist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/productenablement"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/resourcelink"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/servicecdn"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/servicecdnauto"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/servicecompute"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/servicecomputeauto"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/snippet"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/tlsactivation"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/tlscertificate"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/tlsmutualauthentication"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/tlsplatformcertificate"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/tlsprivatekey"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/vcl"
	"github.com/fastly/terraform-provider-fastly-beta/internal/version"
)

type fastlyProvider struct{}

type fastlyProviderModel struct {
	APIToken types.String `tfsdk:"api_token"`
}

func New() provider.Provider {
	return &fastlyProvider{}
}

func (p *fastlyProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "fastly"
}

func (p *fastlyProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "The Fastly API token. Can also be set via the FASTLY_API_TOKEN environment variable.",
			},
		},
	}
}

func (p *fastlyProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config fastlyProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiToken := os.Getenv("FASTLY_API_TOKEN")
	if !config.APIToken.IsNull() && config.APIToken.ValueString() != "" {
		apiToken = config.APIToken.ValueString()
	}

	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"An API token must be provided via the `api_token` provider configuration or FASTLY_API_TOKEN environment variable.",
		)
		return
	}

	client, err := fastly.NewClient(apiToken)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Fastly client", err.Error())
		return
	}

	userAgentPrefix := fmt.Sprintf("terraform-provider-fastly-beta/%s", version.Version)
	data := fastlyclient.NewData(client, userAgentPrefix)

	resp.ResourceData = data
	resp.DataSourceData = data
	resp.ActionData = data
	resp.ListResourceData = data
}

func (p *fastlyProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		acl.NewResource,
		aclentries.NewResource,
		alert.NewResource,
		apisecurityoperation.NewResource,
		apisecurityoperationtag.NewResource,
		backend.NewResource,
		cdnacl.NewResource,
		cdnaclentries.NewResource,
		condition.NewResource,
		configstore.NewResource,
		customdashboard.NewResource,
		dnszone.NewResource,
		domain.NewResource,
		domainmanagement.NewResource,
		domainservicelink.NewResource,
		loggingbigquery.NewResource,
		loggingblobstorage.NewResource,
		loggingdatadog.NewResource,
		logginggcs.NewResource,
		logginghttps.NewResource,
		loggingnewrelic.NewResource,
		loggingnewrelicotlp.NewResource,
		loggings3.NewResource,
		loggingsplunk.NewResource,
		loggingsumologic.NewResource,
		loggingsyslog.NewResource,
		integration.NewResource,
		kvstore.NewResource,
		vcl.NewResource,
		snippet.NewResource,
		dynamicvclsnippet.NewResource,
		dynamicsnippetcontent.NewResource,
		ngwafcountrylist.NewResource,
		ngwafiplist.NewResource,
		ngwafrequestrule.NewResource,
		ngwafsignal.NewResource,
		ngwafsignallist.NewResource,
		ngwafsignalrule.NewResource,
		ngwafstringlist.NewResource,
		ngwafwildcardlist.NewResource,
		ngwafworkspacealertdatadogintegration.NewResource,
		ngwafworkspacealertjiraintegration.NewResource,
		ngwafworkspacealertmailinglistintegration.NewResource,
		ngwafworkspacealertmicrosoftteamsintegration.NewResource,
		ngwafworkspacealertopsgenieintegration.NewResource,
		ngwafworkspacealertpagerdutyintegration.NewResource,
		ngwafworkspacealertslackintegration.NewResource,
		ngwafworkspacealertwebhookintegration.NewResource,
		ngwafworkspace.NewResource,
		ngwafworkspacecountrylist.NewResource,
		ngwafworkspaceiplist.NewResource,
		ngwafworkspaceratelimitrule.NewResource,
		ngwafworkspaceredaction.NewResource,
		ngwafworkspacerequestrule.NewResource,
		ngwafworkspacesignal.NewResource,
		ngwafworkspacesignallist.NewResource,
		ngwafworkspacesignalrule.NewResource,
		ngwafworkspacestringlist.NewResource,
		ngwafworkspacetemplatedsignalrule.NewResource,
		ngwafworkspacethreshold.NewResource,
		ngwafworkspacevirtualpatch.NewResource,
		ngwafworkspacewildcardlist.NewResource,
		productenablement.NewFanoutResource,
		productenablement.NewBrotliCompressionResource,
		productenablement.NewImageOptimizerResource,
		productenablement.NewOriginInspectorResource,
		productenablement.NewDomainInspectorResource,
		productenablement.NewWebsocketsResource,
		productenablement.NewLogExplorerInsightsResource,
		productenablement.NewAPIDiscoveryResource,
		productenablement.NewBotManagementResource,
		productenablement.NewDDoSProtectionResource,
		productenablement.NewNGWAFResource,
		resourcelink.NewResource,
		servicecdn.NewResource,
		servicecdnauto.NewResource,
		servicecompute.NewResource,
		servicecomputeauto.NewResource,
		tlsactivation.NewResource,
		tlscertificate.NewResource,
		tlsmutualauthentication.NewResource,
		tlsplatformcertificate.NewResource,
		tlsprivatekey.NewResource,
	}
}

func (p *fastlyProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		acls.NewDataSource,
		configstores.NewDataSource,
		dnszones.NewDataSource,
		domains.NewDataSource,
		kvstores.NewDataSource,
		ngwafaccountrules.NewDataSource,
		ngwaflists.NewDataSource,
		ngwafsignals.NewDataSource,
		ngwafworkspacealertdatadogintegrations.NewDataSource,
		ngwafworkspacealertjiraintegrations.NewDataSource,
		ngwafworkspacealertmailinglistintegrations.NewDataSource,
		ngwafworkspacealertmicrosoftteamsintegrations.NewDataSource,
		ngwafworkspacealertopsgenieintegrations.NewDataSource,
		ngwafworkspacealertpagerdutyintegrations.NewDataSource,
		ngwafworkspacealertslackintegrations.NewDataSource,
		ngwafworkspacealertwebhookintegrations.NewDataSource,
		ngwafworkspacelists.NewDataSource,
		ngwafworkspaceredactions.NewDataSource,
		ngwafworkspacerules.NewDataSource,
		ngwafworkspaces.NewDataSource,
		ngwafworkspacesignals.NewDataSource,
		ngwafworkspacethresholds.NewDataSource,
		ngwafworkspacevirtualpatches.NewDataSource,
		serviceversion.NewDataSource,
		tlsactivationdatasource.NewDataSource,
		tlsactivationids.NewDataSource,
		tlscertificatedatasource.NewDataSource,
		tlscertificateids.NewDataSource,
		tlsconfiguration.NewDataSource,
		tlsconfigurationids.NewDataSource,
		tlsplatformcertificatedatasource.NewDataSource,
		tlsplatformcertificateids.NewDataSource,
		tlsprivatekeydatasource.NewDataSource,
		tlsprivatekeyids.NewDataSource,
		vclsnippets.NewDataSource,
	}
}

func (p *fastlyProvider) ListResources(_ context.Context) []func() list.ListResource {
	return []func() list.ListResource{
		backend.NewListResource,
		cdnacl.NewListResource,
		cdnaclentries.NewListResource,
		condition.NewListResource,
		domain.NewListResource,
		loggingbigquery.NewListResource,
		loggingblobstorage.NewListResource,
		loggingdatadog.NewListResource,
		logginggcs.NewListResource,
		logginghttps.NewListResource,
		loggingnewrelic.NewListResource,
		loggingnewrelicotlp.NewListResource,
		vcl.NewListResource,
		loggings3.NewListResource,
		loggingsplunk.NewListResource,
		loggingsumologic.NewListResource,
		loggingsyslog.NewListResource,
		servicecdn.NewListResource,
		servicecompute.NewListResource,
	}
}

func (p *fastlyProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		versionclone.NewAction,
		versionactivate.NewAction,
		versionstage.NewAction,
		computepackageupload.NewAction,
	}
}
