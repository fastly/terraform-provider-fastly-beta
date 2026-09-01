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
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacelists"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacerules"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspaces"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacesignals"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafworkspacethresholds"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/serviceversion"
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
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspace"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacecountrylist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspaceiplist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspaceratelimitrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacerequestrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacesignal"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacesignallist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacesignalrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacestringlist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacetemplatedsignalrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacethreshold"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacewildcardlist"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/productenablement"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/resourcelink"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/servicecdn"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/servicecdnauto"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/servicecompute"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/servicecomputeauto"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/snippet"
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
		ngwafworkspace.NewResource,
		ngwafworkspacecountrylist.NewResource,
		ngwafworkspaceiplist.NewResource,
		ngwafworkspaceratelimitrule.NewResource,
		ngwafworkspacerequestrule.NewResource,
		ngwafworkspacesignal.NewResource,
		ngwafworkspacesignallist.NewResource,
		ngwafworkspacesignalrule.NewResource,
		ngwafworkspacestringlist.NewResource,
		ngwafworkspacetemplatedsignalrule.NewResource,
		ngwafworkspacethreshold.NewResource,
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
	}
}

func (p *fastlyProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		acls.NewDataSource,
		configstores.NewDataSource,
		dnszones.NewDataSource,
		domains.NewDataSource,
		kvstores.NewDataSource,
		ngwafworkspacelists.NewDataSource,
		ngwafworkspacerules.NewDataSource,
		ngwafworkspaces.NewDataSource,
		ngwafworkspacesignals.NewDataSource,
		ngwafworkspacethresholds.NewDataSource,
		serviceversion.NewDataSource,
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
