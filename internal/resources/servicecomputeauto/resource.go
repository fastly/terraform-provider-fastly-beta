package servicecomputeauto

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/computepackage"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/backend"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/dictionary"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/domain"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/healthcheck"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingbigquery"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingblobstorage"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingcloudfiles"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingdatadog"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingdigitalocean"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingelasticsearch"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingftp"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginggcs"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginggooglepubsub"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginggrafanacloudlogs"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingheroku"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginghoneycomb"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginghttps"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingkafka"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingkinesis"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingloggly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/logginglogshuttle"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingnewrelic"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingnewrelicotlp"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingopenstack"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingpapertrail"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggings3"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingscalyr"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsftp"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsplunk"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsumologic"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/loggingsyslog"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/resourcelink"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

type Resource struct {
	providerData *fastlyclient.Data
}

// computePackageImportedPrivateKey marks, via private state, that the current Read follows an
// import. ImportStatePassthroughID leaves Package empty in state (only "id" is set), so without
// this marker a just-imported service's package source_code_hash is never refreshed from the API.
const computePackageImportedPrivateKey = "compute_package_imported"

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

func NewResource() resource.Resource {
	return &Resource{}
}

type Model struct {
	ID                      types.String                                 `tfsdk:"id"`
	Name                    types.String                                 `tfsdk:"name"`
	Comment                 types.String                                 `tfsdk:"comment"`
	ForceDestroy            types.Bool                                   `tfsdk:"force_destroy"`
	Reuse                   types.Bool                                   `tfsdk:"reuse"`
	ActiveVersion           types.Int64                                  `tfsdk:"active_version"`
	ManagedVersion          types.Int64                                  `tfsdk:"managed_version"`
	Domain                  []domain.NestedModel                         `tfsdk:"domain"`
	HealthCheck             []healthcheck.NestedModel                    `tfsdk:"healthcheck"`
	Backend                 []backend.NestedModel                        `tfsdk:"backend"`
	Dictionary              []dictionary.NestedModel                     `tfsdk:"dictionary"`
	ResourceLink            []resourcelink.NestedModel                   `tfsdk:"resource_link"`
	Package                 []computepackage.Model                       `tfsdk:"package"`
	LoggingBlobStorage      []loggingblobstorage.ComputeNestedModel      `tfsdk:"logging_blobstorage"`
	LoggingCloudfiles       []loggingcloudfiles.ComputeNestedModel       `tfsdk:"logging_cloudfiles"`
	LoggingOpenStack        []loggingopenstack.ComputeNestedModel        `tfsdk:"logging_openstack"`
	LoggingDigitalOcean     []loggingdigitalocean.ComputeNestedModel     `tfsdk:"logging_digitalocean"`
	LoggingElasticsearch    []loggingelasticsearch.ComputeNestedModel    `tfsdk:"logging_elasticsearch"`
	LoggingFTP              []loggingftp.ComputeNestedModel              `tfsdk:"logging_ftp"`
	LoggingS3               []loggings3.ComputeNestedModel               `tfsdk:"logging_s3"`
	LoggingScalyr           []loggingscalyr.ComputeNestedModel           `tfsdk:"logging_scalyr"`
	LoggingSFTP             []loggingsftp.ComputeNestedModel             `tfsdk:"logging_sftp"`
	LoggingNewRelicOTLP     []loggingnewrelicotlp.ComputeNestedModel     `tfsdk:"logging_newrelicotlp"`
	LoggingNewRelic         []loggingnewrelic.ComputeNestedModel         `tfsdk:"logging_newrelic"`
	LoggingHeroku           []loggingheroku.ComputeNestedModel           `tfsdk:"logging_heroku"`
	LoggingDatadog          []loggingdatadog.ComputeNestedModel          `tfsdk:"logging_datadog"`
	LoggingHoneycomb        []logginghoneycomb.ComputeNestedModel        `tfsdk:"logging_honeycomb"`
	LoggingBigQuery         []loggingbigquery.ComputeNestedModel         `tfsdk:"logging_bigquery"`
	LoggingGCS              []logginggcs.ComputeNestedModel              `tfsdk:"logging_gcs"`
	LoggingGooglePubSub     []logginggooglepubsub.ComputeNestedModel     `tfsdk:"logging_googlepubsub"`
	LoggingGrafanaCloudLogs []logginggrafanacloudlogs.ComputeNestedModel `tfsdk:"logging_grafanacloudlogs"`
	LoggingSplunk           []loggingsplunk.ComputeNestedModel           `tfsdk:"logging_splunk"`
	LoggingHTTPS            []logginghttps.ComputeNestedModel            `tfsdk:"logging_https"`
	LoggingSumologic        []loggingsumologic.ComputeNestedModel        `tfsdk:"logging_sumologic"`
	LoggingSyslog           []loggingsyslog.ComputeNestedModel           `tfsdk:"logging_syslog"`
	LoggingKafka            []loggingkafka.ComputeNestedModel            `tfsdk:"logging_kafka"`
	LoggingKinesis          []loggingkinesis.ComputeNestedModel          `tfsdk:"logging_kinesis"`
	LoggingLoggly           []loggingloggly.ComputeNestedModel           `tfsdk:"logging_loggly"`
	LoggingLogshuttle       []logginglogshuttle.ComputeNestedModel       `tfsdk:"logging_logshuttle"`
	LoggingPapertrail       []loggingpapertrail.ComputeNestedModel       `tfsdk:"logging_papertrail"`
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_compute_auto"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Automatic-lifecycle Fastly Compute service resource with nested versioned configuration. The provider automatically clones, validates, and activates changed versions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The Fastly service ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The service name.",
			},
			"comment": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Managed by Terraform"),
				Description: "Optional service comment.",
			},
			"force_destroy": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Deactivate the active version before deleting the service. Default `false`.",
			},
			"reuse": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Deactivate the active version but do not delete the service, allowing it to be reused/imported elsewhere. Default `false`.",
			},
			"active_version": schema.Int64Attribute{
				Computed:    true,
				Description: "The currently active service version.",
			},
			"managed_version": schema.Int64Attribute{
				Computed:    true,
				Description: "The latest service version selected and managed by this resource.",
			},
		},
		Blocks: map[string]schema.Block{
			"domain":                   domain.NestedBlockSchema(),
			"healthcheck":              healthcheck.NestedBlockSchema(),
			"backend":                  backend.NestedBlockSchema(),
			"dictionary":               dictionary.NestedBlockSchema(),
			"resource_link":            resourcelink.NestedBlockSchema(),
			"package":                  computepackage.NestedBlockSchema(),
			"logging_blobstorage":      loggingblobstorage.ComputeNestedBlockSchema(),
			"logging_cloudfiles":       loggingcloudfiles.ComputeNestedBlockSchema(),
			"logging_openstack":        loggingopenstack.ComputeNestedBlockSchema(),
			"logging_digitalocean":     loggingdigitalocean.ComputeNestedBlockSchema(),
			"logging_elasticsearch":    loggingelasticsearch.ComputeNestedBlockSchema(),
			"logging_ftp":              loggingftp.ComputeNestedBlockSchema(),
			"logging_s3":               loggings3.ComputeNestedBlockSchema(),
			"logging_scalyr":           loggingscalyr.ComputeNestedBlockSchema(),
			"logging_sftp":             loggingsftp.ComputeNestedBlockSchema(),
			"logging_newrelicotlp":     loggingnewrelicotlp.ComputeNestedBlockSchema(),
			"logging_newrelic":         loggingnewrelic.ComputeNestedBlockSchema(),
			"logging_heroku":           loggingheroku.ComputeNestedBlockSchema(),
			"logging_datadog":          loggingdatadog.ComputeNestedBlockSchema(),
			"logging_honeycomb":        logginghoneycomb.ComputeNestedBlockSchema(),
			"logging_bigquery":         loggingbigquery.ComputeNestedBlockSchema(),
			"logging_gcs":              logginggcs.ComputeNestedBlockSchema(),
			"logging_googlepubsub":     logginggooglepubsub.ComputeNestedBlockSchema(),
			"logging_grafanacloudlogs": logginggrafanacloudlogs.ComputeNestedBlockSchema(),
			"logging_splunk":           loggingsplunk.ComputeNestedBlockSchema(),
			"logging_https":            logginghttps.ComputeNestedBlockSchema(),
			"logging_sumologic":        loggingsumologic.ComputeNestedBlockSchema(),
			"logging_syslog":           loggingsyslog.ComputeNestedBlockSchema(),
			"logging_kafka":            loggingkafka.ComputeNestedBlockSchema(),
			"logging_kinesis":          loggingkinesis.ComputeNestedBlockSchema(),
			"logging_loggly":           loggingloggly.ComputeNestedBlockSchema(),
			"logging_logshuttle":       logginglogshuttle.ComputeNestedBlockSchema(),
			"logging_papertrail":       loggingpapertrail.ComputeNestedBlockSchema(),
		},
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	r.providerData = data
}

// partialCreateState is recorded once CreateService succeeds but before the remaining
// reconcile/validate/activate steps complete, so a failure partway through Create leaves the
// service trackable instead of orphaned. Nested blocks are left empty - Update reconciles and
// reads them back from the live API on the next apply regardless.
func partialCreateState(serviceID string, version int, plan *Model) *Model {
	return &Model{
		ID:             types.StringValue(serviceID),
		Name:           plan.Name,
		Comment:        plan.Comment,
		ForceDestroy:   plan.ForceDestroy,
		Reuse:          plan.Reuse,
		ManagedVersion: types.Int64Value(int64(version)),
		ActiveVersion:  types.Int64Null(),
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(plan.Package) == 0 {
		resp.Diagnostics.AddError(
			"Missing Compute package",
			"`fastly_service_compute_auto` automatically validates and activates service versions, so a package block is required when creating a Compute service.",
		)
		return
	}

	if err := computepackage.ValidateInput(plan.Package); err != nil {
		resp.Diagnostics.AddError("Invalid Compute package", err.Error())
		return
	}

	created, err := r.providerData.AutoClient().CreateService(ctx, &fastly.CreateServiceInput{
		Name:    new(plan.Name.ValueString()),
		Comment: new(plan.Comment.ValueString()),
		Type:    new(service.TypeCompute),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Fastly Compute service", err.Error())
		return
	}

	serviceID := fastly.ToValue(created.ServiceID)
	version := 1

	tflog.Info(ctx, "Created Fastly Compute service", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	// Once CreateService succeeds, every subsequent failure must record the service ID (and what
	// else is known) before returning - see CDTOOL-1731.
	recordOrphanSafeState := func() {
		resp.Diagnostics.Append(resp.State.Set(ctx, partialCreateState(serviceID, version, &plan))...)
	}

	if err := domain.Reconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.Domain); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling domains", err.Error())
		return
	}

	domains, err := domain.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading service domains", err.Error())
		return
	}
	plan.Domain = domain.MatchOrder(domains, plan.Domain)

	// Health checks must be reconciled before backends: a backend can reference a health check
	// by name, and the Fastly API rejects a backend create that names a health check which
	// doesn't exist yet in this version.
	if err := healthcheck.Reconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.HealthCheck); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling health checks", err.Error())
		return
	}

	healthChecks, err := healthcheck.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading service health checks", err.Error())
		return
	}
	plan.HealthCheck = healthcheck.MatchOrder(healthChecks, plan.HealthCheck)

	if err := backend.Reconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.Backend); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling backends", err.Error())
		return
	}

	backends, err := backend.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading service backends", err.Error())
		return
	}
	plan.Backend = backend.MatchOrder(backends, plan.Backend)

	if err := dictionary.ReconcileWithPrevious(ctx, r.providerData.AutoClient(), serviceID, version, nil, plan.Dictionary); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling dictionaries", err.Error())
		return
	}

	dictionaries, err := dictionary.ReadForVersionWithPlan(ctx, r.providerData.AutoClient(), serviceID, version, plan.Dictionary)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading service dictionaries", err.Error())
		return
	}
	plan.Dictionary = dictionary.MatchOrder(dictionaries, plan.Dictionary)

	if err := resourcelink.Reconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.ResourceLink); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling resource links", err.Error())
		return
	}

	resourceLinks, err := resourcelink.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading service resource links", err.Error())
		return
	}
	plan.ResourceLink = resourcelink.MatchOrder(resourceLinks, plan.ResourceLink)

	if err := loggingblobstorage.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingBlobStorage); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Blob Storage logging endpoints", err.Error())
		return
	}

	loggingBlobStorages, err := loggingblobstorage.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Blob Storage logging endpoints", err.Error())
		return
	}
	plan.LoggingBlobStorage = loggingblobstorage.ComputeMatchOrder(loggingBlobStorages, plan.LoggingBlobStorage)

	if err := loggingcloudfiles.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingCloudfiles); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Cloud Files logging endpoints", err.Error())
		return
	}

	loggingCloudfiless, err := loggingcloudfiles.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Cloud Files logging endpoints", err.Error())
		return
	}
	plan.LoggingCloudfiles = loggingcloudfiles.ComputeMatchOrder(loggingCloudfiless, plan.LoggingCloudfiles)

	if err := loggingopenstack.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingOpenStack); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling OpenStack logging endpoints", err.Error())
		return
	}

	loggingOpenStacks, err := loggingopenstack.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading OpenStack logging endpoints", err.Error())
		return
	}
	plan.LoggingOpenStack = loggingopenstack.ComputeMatchOrder(loggingOpenStacks, plan.LoggingOpenStack)

	if err := loggingdigitalocean.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingDigitalOcean); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling DigitalOcean logging endpoints", err.Error())
		return
	}

	loggingDigitalOceans, err := loggingdigitalocean.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading DigitalOcean logging endpoints", err.Error())
		return
	}
	plan.LoggingDigitalOcean = loggingdigitalocean.ComputeMatchOrder(loggingDigitalOceans, plan.LoggingDigitalOcean)

	if err := loggingelasticsearch.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingElasticsearch); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Elasticsearch logging endpoints", err.Error())
		return
	}

	loggingElasticsearches, err := loggingelasticsearch.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Elasticsearch logging endpoints", err.Error())
		return
	}
	plan.LoggingElasticsearch = loggingelasticsearch.ComputeMatchOrder(loggingElasticsearches, plan.LoggingElasticsearch)

	if err := loggingftp.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingFTP); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling FTP logging endpoints", err.Error())
		return
	}

	loggingFTPs, err := loggingftp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading FTP logging endpoints", err.Error())
		return
	}
	plan.LoggingFTP = loggingftp.ComputeMatchOrder(loggingFTPs, plan.LoggingFTP)

	if err := loggings3.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingS3); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling S3 logging endpoints", err.Error())
		return
	}

	loggingS3s, err := loggings3.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading S3 logging endpoints", err.Error())
		return
	}
	plan.LoggingS3 = loggings3.ComputeMatchOrder(loggingS3s, plan.LoggingS3)

	if err := loggingscalyr.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingScalyr); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Scalyr logging endpoints", err.Error())
		return
	}

	loggingScalyrs, err := loggingscalyr.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Scalyr logging endpoints", err.Error())
		return
	}
	plan.LoggingScalyr = loggingscalyr.ComputeMatchOrder(loggingScalyrs, plan.LoggingScalyr)

	if err := loggingsftp.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingSFTP); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling SFTP logging endpoints", err.Error())
		return
	}

	loggingSFTPs, err := loggingsftp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading SFTP logging endpoints", err.Error())
		return
	}
	plan.LoggingSFTP = loggingsftp.ComputeMatchOrder(loggingSFTPs, plan.LoggingSFTP)

	if err := loggingnewrelicotlp.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingNewRelicOTLP); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling New Relic OTLP logging endpoints", err.Error())
		return
	}

	loggingNewRelicOTLPs, err := loggingnewrelicotlp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading New Relic OTLP logging endpoints", err.Error())
		return
	}
	plan.LoggingNewRelicOTLP = loggingnewrelicotlp.ComputeMatchOrder(loggingNewRelicOTLPs, plan.LoggingNewRelicOTLP)

	if err := loggingnewrelic.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingNewRelic); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling New Relic logging endpoints", err.Error())
		return
	}

	loggingNewRelics, err := loggingnewrelic.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading New Relic logging endpoints", err.Error())
		return
	}
	plan.LoggingNewRelic = loggingnewrelic.ComputeMatchOrder(loggingNewRelics, plan.LoggingNewRelic)

	if err := loggingheroku.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingHeroku); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Heroku logging endpoints", err.Error())
		return
	}

	loggingHerokus, err := loggingheroku.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Heroku logging endpoints", err.Error())
		return
	}
	plan.LoggingHeroku = loggingheroku.ComputeMatchOrder(loggingHerokus, plan.LoggingHeroku)

	if err := loggingdatadog.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingDatadog); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Datadog logging endpoints", err.Error())
		return
	}

	loggingDatadogs, err := loggingdatadog.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Datadog logging endpoints", err.Error())
		return
	}
	plan.LoggingDatadog = loggingdatadog.ComputeMatchOrder(loggingDatadogs, plan.LoggingDatadog)

	if err := logginghoneycomb.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingHoneycomb); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Honeycomb logging endpoints", err.Error())
		return
	}

	loggingHoneycombs, err := logginghoneycomb.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Honeycomb logging endpoints", err.Error())
		return
	}
	plan.LoggingHoneycomb = logginghoneycomb.ComputeMatchOrder(loggingHoneycombs, plan.LoggingHoneycomb)

	if err := loggingbigquery.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingBigQuery); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling BigQuery logging endpoints", err.Error())
		return
	}

	loggingBigQueries, err := loggingbigquery.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading BigQuery logging endpoints", err.Error())
		return
	}
	plan.LoggingBigQuery = loggingbigquery.ComputeMatchOrder(loggingBigQueries, plan.LoggingBigQuery)

	if err := logginggcs.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingGCS); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling GCS logging endpoints", err.Error())
		return
	}

	loggingGCSs, err := logginggcs.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading GCS logging endpoints", err.Error())
		return
	}
	plan.LoggingGCS = logginggcs.ComputeMatchOrder(loggingGCSs, plan.LoggingGCS)

	if err := logginggooglepubsub.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingGooglePubSub); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Pub/Sub logging endpoints", err.Error())
		return
	}

	loggingGooglePubSubs, err := logginggooglepubsub.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Pub/Sub logging endpoints", err.Error())
		return
	}
	plan.LoggingGooglePubSub = logginggooglepubsub.ComputeMatchOrder(loggingGooglePubSubs, plan.LoggingGooglePubSub)

	if err := logginggrafanacloudlogs.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingGrafanaCloudLogs); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling GrafanaCloudLogs logging endpoints", err.Error())
		return
	}

	loggingGrafanaCloudLogss, err := logginggrafanacloudlogs.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading GrafanaCloudLogs logging endpoints", err.Error())
		return
	}
	plan.LoggingGrafanaCloudLogs = logginggrafanacloudlogs.ComputeMatchOrder(loggingGrafanaCloudLogss, plan.LoggingGrafanaCloudLogs)

	if err := loggingsplunk.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingSplunk); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Splunk logging endpoints", err.Error())
		return
	}

	loggingSplunks, err := loggingsplunk.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Splunk logging endpoints", err.Error())
		return
	}
	plan.LoggingSplunk = loggingsplunk.ComputeMatchOrder(loggingSplunks, plan.LoggingSplunk)

	if err := logginghttps.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingHTTPS); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling HTTPS logging endpoints", err.Error())
		return
	}

	loggingHTTPS, err := logginghttps.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading HTTPS logging endpoints", err.Error())
		return
	}
	plan.LoggingHTTPS = logginghttps.ComputeMatchOrder(loggingHTTPS, plan.LoggingHTTPS)

	if err := loggingsumologic.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingSumologic); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Sumologic logging endpoints", err.Error())
		return
	}

	loggingSumologics, err := loggingsumologic.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Sumologic logging endpoints", err.Error())
		return
	}
	plan.LoggingSumologic = loggingsumologic.ComputeMatchOrder(loggingSumologics, plan.LoggingSumologic)

	if err := loggingsyslog.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingSyslog); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Syslog logging endpoints", err.Error())
		return
	}

	loggingSyslogs, err := loggingsyslog.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Syslog logging endpoints", err.Error())
		return
	}
	plan.LoggingSyslog = loggingsyslog.ComputeMatchOrder(loggingSyslogs, plan.LoggingSyslog)

	if err := loggingkafka.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingKafka); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Kafka logging endpoints", err.Error())
		return
	}

	loggingKafkas, err := loggingkafka.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Kafka logging endpoints", err.Error())
		return
	}
	plan.LoggingKafka = loggingkafka.ComputeMatchOrder(loggingKafkas, plan.LoggingKafka)

	if err := loggingkinesis.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingKinesis); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Kinesis logging endpoints", err.Error())
		return
	}

	loggingKineses, err := loggingkinesis.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Kinesis logging endpoints", err.Error())
		return
	}
	plan.LoggingKinesis = loggingkinesis.ComputeMatchOrder(loggingKineses, plan.LoggingKinesis)

	if err := loggingloggly.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingLoggly); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Loggly logging endpoints", err.Error())
		return
	}

	loggingLogglys, err := loggingloggly.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Loggly logging endpoints", err.Error())
		return
	}
	plan.LoggingLoggly = loggingloggly.ComputeMatchOrder(loggingLogglys, plan.LoggingLoggly)

	if err := logginglogshuttle.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingLogshuttle); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Log Shuttle logging endpoints", err.Error())
		return
	}

	loggingLogshuttles, err := logginglogshuttle.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Log Shuttle logging endpoints", err.Error())
		return
	}
	plan.LoggingLogshuttle = logginglogshuttle.ComputeMatchOrder(loggingLogshuttles, plan.LoggingLogshuttle)

	if err := loggingpapertrail.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, version, plan.LoggingPapertrail); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reconciling Papertrail logging endpoints", err.Error())
		return
	}

	loggingPapertrails, err := loggingpapertrail.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Papertrail logging endpoints", err.Error())
		return
	}
	plan.LoggingPapertrail = loggingpapertrail.ComputeMatchOrder(loggingPapertrails, plan.LoggingPapertrail)

	if err := computepackage.Update(ctx, r.providerData.AutoClient(), serviceID, version, plan.Package); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error updating Compute package", err.Error())
		return
	}

	packages, err := computepackage.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, version, plan.Package, false)
	if err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error reading Compute package", err.Error())
		return
	}
	plan.Package = packages

	if err := service.ValidateVersion(ctx, r.providerData.AutoClient(), serviceID, version); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error validating service version", err.Error())
		return
	}

	plan.ID = types.StringValue(serviceID)
	plan.ManagedVersion = types.Int64Value(int64(version))

	if _, err := r.providerData.AutoClient().ActivateVersion(ctx, &fastly.ActivateVersionInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	}); err != nil {
		recordOrphanSafeState()
		resp.Diagnostics.AddError("Error activating service version", err.Error())
		return
	}
	plan.ActiveVersion = types.Int64Value(int64(version))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	details, err := r.providerData.AutoClient().GetServiceDetails(ctx, &fastly.GetServiceDetailsInput{
		ServiceID: state.ID.ValueString(),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Fastly Compute service", err.Error())
		return
	}

	serviceType := fastly.ToValue(details.Type)
	if serviceType != service.TypeCompute {
		resp.Diagnostics.AddError(
			"Unexpected Fastly service type",
			fmt.Sprintf("Expected Compute service %q to have type %q, got %q.", state.ID.ValueString(), service.TypeCompute, serviceType),
		)
		return
	}

	if details.Name != nil {
		state.Name = types.StringValue(*details.Name)
	}
	if details.Comment != nil {
		state.Comment = types.StringValue(*details.Comment)
	}

	readVersion, active, err := service.SelectReadVersionFromDetails(details, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error selecting service version for read", err.Error())
		return
	}

	if active {
		state.ActiveVersion = types.Int64Value(int64(readVersion))
	} else {
		state.ActiveVersion = types.Int64Null()
	}
	state.ManagedVersion = types.Int64Value(int64(readVersion))

	domains, err := domain.ReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service domains", err.Error())
		return
	}
	healthChecks, err := healthcheck.ReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service health checks", err.Error())
		return
	}
	backends, err := backend.ReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service backends", err.Error())
		return
	}
	dictionaries, err := dictionary.ReadForVersionWithPlan(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion, state.Dictionary)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service dictionaries", err.Error())
		return
	}
	loggingBlobStorages, err := loggingblobstorage.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Blob Storage logging endpoints", err.Error())
		return
	}
	loggingCloudfiless, err := loggingcloudfiles.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Cloud Files logging endpoints", err.Error())
		return
	}
	loggingOpenStacks, err := loggingopenstack.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading OpenStack logging endpoints", err.Error())
		return
	}
	loggingDigitalOceans, err := loggingdigitalocean.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DigitalOcean logging endpoints", err.Error())
		return
	}
	loggingElasticsearches, err := loggingelasticsearch.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Elasticsearch logging endpoints", err.Error())
		return
	}
	loggingFTPs, err := loggingftp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading FTP logging endpoints", err.Error())
		return
	}
	loggingS3s, err := loggings3.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading S3 logging endpoints", err.Error())
		return
	}
	loggingScalyrs, err := loggingscalyr.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Scalyr logging endpoints", err.Error())
		return
	}
	loggingSFTPs, err := loggingsftp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading SFTP logging endpoints", err.Error())
		return
	}
	loggingNewRelicOTLPs, err := loggingnewrelicotlp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading New Relic OTLP logging endpoints", err.Error())
		return
	}
	loggingNewRelics, err := loggingnewrelic.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading New Relic logging endpoints", err.Error())
		return
	}
	loggingHerokus, err := loggingheroku.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Heroku logging endpoints", err.Error())
		return
	}
	loggingDatadogs, err := loggingdatadog.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Datadog logging endpoints", err.Error())
		return
	}
	loggingHoneycombs, err := logginghoneycomb.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Honeycomb logging endpoints", err.Error())
		return
	}
	loggingBigQueries, err := loggingbigquery.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading BigQuery logging endpoints", err.Error())
		return
	}
	loggingGCSs, err := logginggcs.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading GCS logging endpoints", err.Error())
		return
	}
	loggingGooglePubSubs, err := logginggooglepubsub.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Pub/Sub logging endpoints", err.Error())
		return
	}
	loggingGrafanaCloudLogss, err := logginggrafanacloudlogs.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading GrafanaCloudLogs logging endpoints", err.Error())
		return
	}
	loggingSplunks, err := loggingsplunk.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Splunk logging endpoints", err.Error())
		return
	}
	loggingHTTPS, err := logginghttps.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading HTTPS logging endpoints", err.Error())
		return
	}
	loggingSumologics, err := loggingsumologic.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Sumologic logging endpoints", err.Error())
		return
	}
	loggingSyslogs, err := loggingsyslog.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Syslog logging endpoints", err.Error())
		return
	}
	loggingKafkas, err := loggingkafka.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Kafka logging endpoints", err.Error())
		return
	}
	loggingKineses, err := loggingkinesis.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Kinesis logging endpoints", err.Error())
		return
	}
	loggingLogglys, err := loggingloggly.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Loggly logging endpoints", err.Error())
		return
	}
	loggingLogshuttles, err := logginglogshuttle.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Log Shuttle logging endpoints", err.Error())
		return
	}
	loggingPapertrails, err := loggingpapertrail.ComputeReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Papertrail logging endpoints", err.Error())
		return
	}
	state.Domain = domain.MatchOrder(domains, state.Domain)
	state.HealthCheck = healthcheck.MatchOrder(healthChecks, state.HealthCheck)
	state.Backend = backend.MatchOrder(backends, state.Backend)
	state.Dictionary = dictionary.MatchOrder(dictionaries, state.Dictionary)
	state.LoggingBlobStorage = loggingblobstorage.ComputeMatchOrder(loggingBlobStorages, state.LoggingBlobStorage)
	state.LoggingCloudfiles = loggingcloudfiles.ComputeMatchOrder(loggingCloudfiless, state.LoggingCloudfiles)
	state.LoggingOpenStack = loggingopenstack.ComputeMatchOrder(loggingOpenStacks, state.LoggingOpenStack)
	state.LoggingDigitalOcean = loggingdigitalocean.ComputeMatchOrder(loggingDigitalOceans, state.LoggingDigitalOcean)
	state.LoggingElasticsearch = loggingelasticsearch.ComputeMatchOrder(loggingElasticsearches, state.LoggingElasticsearch)
	state.LoggingFTP = loggingftp.ComputeMatchOrder(loggingFTPs, state.LoggingFTP)
	state.LoggingS3 = loggings3.ComputeMatchOrder(loggingS3s, state.LoggingS3)
	state.LoggingScalyr = loggingscalyr.ComputeMatchOrder(loggingScalyrs, state.LoggingScalyr)
	state.LoggingSFTP = loggingsftp.ComputeMatchOrder(loggingSFTPs, state.LoggingSFTP)
	state.LoggingNewRelicOTLP = loggingnewrelicotlp.ComputeMatchOrder(loggingNewRelicOTLPs, state.LoggingNewRelicOTLP)
	state.LoggingNewRelic = loggingnewrelic.ComputeMatchOrder(loggingNewRelics, state.LoggingNewRelic)
	state.LoggingHeroku = loggingheroku.ComputeMatchOrder(loggingHerokus, state.LoggingHeroku)
	state.LoggingDatadog = loggingdatadog.ComputeMatchOrder(loggingDatadogs, state.LoggingDatadog)
	state.LoggingHoneycomb = logginghoneycomb.ComputeMatchOrder(loggingHoneycombs, state.LoggingHoneycomb)
	state.LoggingBigQuery = loggingbigquery.ComputeMatchOrder(loggingBigQueries, state.LoggingBigQuery)
	state.LoggingGCS = logginggcs.ComputeMatchOrder(loggingGCSs, state.LoggingGCS)
	state.LoggingGooglePubSub = logginggooglepubsub.ComputeMatchOrder(loggingGooglePubSubs, state.LoggingGooglePubSub)
	state.LoggingGrafanaCloudLogs = logginggrafanacloudlogs.ComputeMatchOrder(loggingGrafanaCloudLogss, state.LoggingGrafanaCloudLogs)
	state.LoggingSplunk = loggingsplunk.ComputeMatchOrder(loggingSplunks, state.LoggingSplunk)
	state.LoggingHTTPS = logginghttps.ComputeMatchOrder(loggingHTTPS, state.LoggingHTTPS)
	state.LoggingSumologic = loggingsumologic.ComputeMatchOrder(loggingSumologics, state.LoggingSumologic)
	state.LoggingSyslog = loggingsyslog.ComputeMatchOrder(loggingSyslogs, state.LoggingSyslog)
	state.LoggingKafka = loggingkafka.ComputeMatchOrder(loggingKafkas, state.LoggingKafka)
	state.LoggingKinesis = loggingkinesis.ComputeMatchOrder(loggingKineses, state.LoggingKinesis)
	state.LoggingLoggly = loggingloggly.ComputeMatchOrder(loggingLogglys, state.LoggingLoggly)
	state.LoggingLogshuttle = logginglogshuttle.ComputeMatchOrder(loggingLogshuttles, state.LoggingLogshuttle)
	state.LoggingPapertrail = loggingpapertrail.ComputeMatchOrder(loggingPapertrails, state.LoggingPapertrail)

	resourceLinks, err := resourcelink.ReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service resource links", err.Error())
		return
	}
	state.ResourceLink = resourcelink.MatchOrder(resourceLinks, state.ResourceLink)

	importedBytes, diags := req.Private.GetKey(ctx, computePackageImportedPrivateKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	imported := len(importedBytes) > 0

	packages, err := computepackage.ReadForVersion(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion, state.Package, imported)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Compute package", err.Error())
		return
	}
	state.Package = packages

	if imported {
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, computePackageImportedPrivateKey, nil)...)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	var state Model

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ID.ValueString()

	if err := service.UpdateMetadataIfChanged(
		ctx,
		r.providerData.AutoClient(),
		serviceID,
		plan.Name,
		plan.Comment,
		state.Name,
		state.Comment,
	); err != nil {
		resp.Diagnostics.AddError("Error updating Fastly Compute service", err.Error())
		return
	}

	nestedChanged := !domain.Equal(plan.Domain, state.Domain) ||
		!healthcheck.Equal(plan.HealthCheck, state.HealthCheck) ||
		!backend.Equal(plan.Backend, state.Backend) ||
		!dictionary.Equal(plan.Dictionary, state.Dictionary) ||
		!resourcelink.Equal(plan.ResourceLink, state.ResourceLink) ||
		!computepackage.Equal(plan.Package, state.Package) ||
		!loggingblobstorage.ComputeEqual(plan.LoggingBlobStorage, state.LoggingBlobStorage) ||
		!loggingcloudfiles.ComputeEqual(plan.LoggingCloudfiles, state.LoggingCloudfiles) ||
		!loggingopenstack.ComputeEqual(plan.LoggingOpenStack, state.LoggingOpenStack) ||
		!loggingdigitalocean.ComputeEqual(plan.LoggingDigitalOcean, state.LoggingDigitalOcean) ||
		!loggingelasticsearch.ComputeEqual(plan.LoggingElasticsearch, state.LoggingElasticsearch) ||
		!loggingftp.ComputeEqual(plan.LoggingFTP, state.LoggingFTP) ||
		!loggings3.ComputeEqual(plan.LoggingS3, state.LoggingS3) ||
		!loggingscalyr.ComputeEqual(plan.LoggingScalyr, state.LoggingScalyr) ||
		!loggingsftp.ComputeEqual(plan.LoggingSFTP, state.LoggingSFTP) ||
		!loggingnewrelicotlp.ComputeEqual(plan.LoggingNewRelicOTLP, state.LoggingNewRelicOTLP) ||
		!loggingnewrelic.ComputeEqual(plan.LoggingNewRelic, state.LoggingNewRelic) ||
		!loggingheroku.ComputeEqual(plan.LoggingHeroku, state.LoggingHeroku) ||
		!loggingdatadog.ComputeEqual(plan.LoggingDatadog, state.LoggingDatadog) ||
		!logginghoneycomb.ComputeEqual(plan.LoggingHoneycomb, state.LoggingHoneycomb) ||
		!loggingbigquery.ComputeEqual(plan.LoggingBigQuery, state.LoggingBigQuery) ||
		!logginggcs.ComputeEqual(plan.LoggingGCS, state.LoggingGCS) ||
		!logginggooglepubsub.ComputeEqual(plan.LoggingGooglePubSub, state.LoggingGooglePubSub) ||
		!logginggrafanacloudlogs.ComputeEqual(plan.LoggingGrafanaCloudLogs, state.LoggingGrafanaCloudLogs) ||
		!loggingsplunk.ComputeEqual(plan.LoggingSplunk, state.LoggingSplunk) ||
		!logginghttps.ComputeEqual(plan.LoggingHTTPS, state.LoggingHTTPS) ||
		!loggingsumologic.ComputeEqual(plan.LoggingSumologic, state.LoggingSumologic) ||
		!loggingsyslog.ComputeEqual(plan.LoggingSyslog, state.LoggingSyslog) ||
		!loggingkafka.ComputeEqual(plan.LoggingKafka, state.LoggingKafka) ||
		!loggingkinesis.ComputeEqual(plan.LoggingKinesis, state.LoggingKinesis) ||
		!loggingloggly.ComputeEqual(plan.LoggingLoggly, state.LoggingLoggly) ||
		!logginglogshuttle.ComputeEqual(plan.LoggingLogshuttle, state.LoggingLogshuttle) ||
		!loggingpapertrail.ComputeEqual(plan.LoggingPapertrail, state.LoggingPapertrail) ||
		false
	needsVersionChange := nestedChanged

	targetVersion := 0

	if needsVersionChange {
		sourceVersion, shouldClone, err := r.selectWorkingVersion(ctx, serviceID)
		if err != nil {
			resp.Diagnostics.AddError("Error selecting Fastly service version", err.Error())
			return
		}

		if shouldClone {
			cloned, err := r.providerData.AutoClient().CloneVersion(ctx, &fastly.CloneVersionInput{
				ServiceID:      serviceID,
				ServiceVersion: sourceVersion,
			})
			if err != nil {
				resp.Diagnostics.AddError("Error cloning Fastly service version", err.Error())
				return
			}
			targetVersion = fastly.ToValue(cloned.Number)
		} else {
			targetVersion = sourceVersion
		}

		tflog.Info(ctx, "Selected Fastly Compute service working version", map[string]any{
			"service_id":     serviceID,
			"source_version": sourceVersion,
			"target_version": targetVersion,
			"cloned":         shouldClone,
			"nested_changed": nestedChanged,
		})

		if err := domain.Reconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.Domain); err != nil {
			resp.Diagnostics.AddError("Error reconciling domains", err.Error())
			return
		}

		domains, err := domain.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading service domains", err.Error())
			return
		}
		plan.Domain = domain.MatchOrder(domains, plan.Domain)

		// Health checks must be reconciled before backends: a backend can reference a health
		// check by name, and the Fastly API rejects a backend create that names a health check
		// which doesn't exist yet in this version.
		if err := healthcheck.Reconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.HealthCheck); err != nil {
			resp.Diagnostics.AddError("Error reconciling health checks", err.Error())
			return
		}

		healthChecks, err := healthcheck.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading service health checks", err.Error())
			return
		}
		plan.HealthCheck = healthcheck.MatchOrder(healthChecks, plan.HealthCheck)

		if err := backend.Reconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.Backend); err != nil {
			resp.Diagnostics.AddError("Error reconciling backends", err.Error())
			return
		}

		backends, err := backend.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading service backends", err.Error())
			return
		}
		plan.Backend = backend.MatchOrder(backends, plan.Backend)

		if err := dictionary.ReconcileWithPrevious(ctx, r.providerData.AutoClient(), serviceID, targetVersion, state.Dictionary, plan.Dictionary); err != nil {
			resp.Diagnostics.AddError("Error reconciling dictionaries", err.Error())
			return
		}

		dictionaries, err := dictionary.ReadForVersionWithPlan(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.Dictionary)
		if err != nil {
			resp.Diagnostics.AddError("Error reading service dictionaries", err.Error())
			return
		}
		plan.Dictionary = dictionary.MatchOrder(dictionaries, plan.Dictionary)

		if err := resourcelink.Reconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.ResourceLink); err != nil {
			resp.Diagnostics.AddError("Error reconciling resource links", err.Error())
			return
		}

		resourceLinks, err := resourcelink.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading service resource links", err.Error())
			return
		}
		plan.ResourceLink = resourcelink.MatchOrder(resourceLinks, plan.ResourceLink)

		if err := loggingblobstorage.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingBlobStorage); err != nil {
			resp.Diagnostics.AddError("Error reconciling Blob Storage logging endpoints", err.Error())
			return
		}

		loggingBlobStorages, err := loggingblobstorage.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Blob Storage logging endpoints", err.Error())
			return
		}
		plan.LoggingBlobStorage = loggingblobstorage.ComputeMatchOrder(loggingBlobStorages, plan.LoggingBlobStorage)

		if err := loggingcloudfiles.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingCloudfiles); err != nil {
			resp.Diagnostics.AddError("Error reconciling Cloud Files logging endpoints", err.Error())
			return
		}

		loggingCloudfiless, err := loggingcloudfiles.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Cloud Files logging endpoints", err.Error())
			return
		}
		plan.LoggingCloudfiles = loggingcloudfiles.ComputeMatchOrder(loggingCloudfiless, plan.LoggingCloudfiles)

		if err := loggingopenstack.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingOpenStack); err != nil {
			resp.Diagnostics.AddError("Error reconciling OpenStack logging endpoints", err.Error())
			return
		}

		loggingOpenStacks, err := loggingopenstack.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading OpenStack logging endpoints", err.Error())
			return
		}
		plan.LoggingOpenStack = loggingopenstack.ComputeMatchOrder(loggingOpenStacks, plan.LoggingOpenStack)

		if err := loggingdigitalocean.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingDigitalOcean); err != nil {
			resp.Diagnostics.AddError("Error reconciling DigitalOcean logging endpoints", err.Error())
			return
		}

		loggingDigitalOceans, err := loggingdigitalocean.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading DigitalOcean logging endpoints", err.Error())
			return
		}
		plan.LoggingDigitalOcean = loggingdigitalocean.ComputeMatchOrder(loggingDigitalOceans, plan.LoggingDigitalOcean)

		if err := loggingelasticsearch.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingElasticsearch); err != nil {
			resp.Diagnostics.AddError("Error reconciling Elasticsearch logging endpoints", err.Error())
			return
		}

		loggingElasticsearches, err := loggingelasticsearch.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Elasticsearch logging endpoints", err.Error())
			return
		}
		plan.LoggingElasticsearch = loggingelasticsearch.ComputeMatchOrder(loggingElasticsearches, plan.LoggingElasticsearch)

		if err := loggingftp.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingFTP); err != nil {
			resp.Diagnostics.AddError("Error reconciling FTP logging endpoints", err.Error())
			return
		}

		loggingFTPs, err := loggingftp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading FTP logging endpoints", err.Error())
			return
		}
		plan.LoggingFTP = loggingftp.ComputeMatchOrder(loggingFTPs, plan.LoggingFTP)

		if err := loggings3.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingS3); err != nil {
			resp.Diagnostics.AddError("Error reconciling S3 logging endpoints", err.Error())
			return
		}

		loggingS3s, err := loggings3.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading S3 logging endpoints", err.Error())
			return
		}
		plan.LoggingS3 = loggings3.ComputeMatchOrder(loggingS3s, plan.LoggingS3)

		if err := loggingscalyr.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingScalyr); err != nil {
			resp.Diagnostics.AddError("Error reconciling Scalyr logging endpoints", err.Error())
			return
		}

		loggingScalyrs, err := loggingscalyr.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Scalyr logging endpoints", err.Error())
			return
		}
		plan.LoggingScalyr = loggingscalyr.ComputeMatchOrder(loggingScalyrs, plan.LoggingScalyr)

		if err := loggingsftp.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingSFTP); err != nil {
			resp.Diagnostics.AddError("Error reconciling SFTP logging endpoints", err.Error())
			return
		}

		loggingSFTPs, err := loggingsftp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading SFTP logging endpoints", err.Error())
			return
		}
		plan.LoggingSFTP = loggingsftp.ComputeMatchOrder(loggingSFTPs, plan.LoggingSFTP)

		if err := loggingnewrelicotlp.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingNewRelicOTLP); err != nil {
			resp.Diagnostics.AddError("Error reconciling New Relic OTLP logging endpoints", err.Error())
			return
		}

		loggingNewRelicOTLPs, err := loggingnewrelicotlp.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading New Relic OTLP logging endpoints", err.Error())
			return
		}
		plan.LoggingNewRelicOTLP = loggingnewrelicotlp.ComputeMatchOrder(loggingNewRelicOTLPs, plan.LoggingNewRelicOTLP)

		if err := loggingnewrelic.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingNewRelic); err != nil {
			resp.Diagnostics.AddError("Error reconciling New Relic logging endpoints", err.Error())
			return
		}

		loggingNewRelics, err := loggingnewrelic.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading New Relic logging endpoints", err.Error())
			return
		}
		plan.LoggingNewRelic = loggingnewrelic.ComputeMatchOrder(loggingNewRelics, plan.LoggingNewRelic)

		if err := loggingheroku.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingHeroku); err != nil {
			resp.Diagnostics.AddError("Error reconciling Heroku logging endpoints", err.Error())
			return
		}

		loggingHerokus, err := loggingheroku.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Heroku logging endpoints", err.Error())
			return
		}
		plan.LoggingHeroku = loggingheroku.ComputeMatchOrder(loggingHerokus, plan.LoggingHeroku)

		if err := loggingdatadog.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingDatadog); err != nil {
			resp.Diagnostics.AddError("Error reconciling Datadog logging endpoints", err.Error())
			return
		}

		loggingDatadogs, err := loggingdatadog.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Datadog logging endpoints", err.Error())
			return
		}
		plan.LoggingDatadog = loggingdatadog.ComputeMatchOrder(loggingDatadogs, plan.LoggingDatadog)

		if err := logginghoneycomb.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingHoneycomb); err != nil {
			resp.Diagnostics.AddError("Error reconciling Honeycomb logging endpoints", err.Error())
			return
		}

		loggingHoneycombs, err := logginghoneycomb.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Honeycomb logging endpoints", err.Error())
			return
		}
		plan.LoggingHoneycomb = logginghoneycomb.ComputeMatchOrder(loggingHoneycombs, plan.LoggingHoneycomb)

		if err := loggingbigquery.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingBigQuery); err != nil {
			resp.Diagnostics.AddError("Error reconciling BigQuery logging endpoints", err.Error())
			return
		}

		loggingBigQueries, err := loggingbigquery.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading BigQuery logging endpoints", err.Error())
			return
		}
		plan.LoggingBigQuery = loggingbigquery.ComputeMatchOrder(loggingBigQueries, plan.LoggingBigQuery)

		if err := logginggcs.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingGCS); err != nil {
			resp.Diagnostics.AddError("Error reconciling GCS logging endpoints", err.Error())
			return
		}

		loggingGCSs, err := logginggcs.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading GCS logging endpoints", err.Error())
			return
		}
		plan.LoggingGCS = logginggcs.ComputeMatchOrder(loggingGCSs, plan.LoggingGCS)

		if err := logginggooglepubsub.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingGooglePubSub); err != nil {
			resp.Diagnostics.AddError("Error reconciling Pub/Sub logging endpoints", err.Error())
			return
		}

		loggingGooglePubSubs, err := logginggooglepubsub.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Pub/Sub logging endpoints", err.Error())
			return
		}
		plan.LoggingGooglePubSub = logginggooglepubsub.ComputeMatchOrder(loggingGooglePubSubs, plan.LoggingGooglePubSub)

		if err := logginggrafanacloudlogs.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingGrafanaCloudLogs); err != nil {
			resp.Diagnostics.AddError("Error reconciling GrafanaCloudLogs logging endpoints", err.Error())
			return
		}

		loggingGrafanaCloudLogss, err := logginggrafanacloudlogs.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading GrafanaCloudLogs logging endpoints", err.Error())
			return
		}
		plan.LoggingGrafanaCloudLogs = logginggrafanacloudlogs.ComputeMatchOrder(loggingGrafanaCloudLogss, plan.LoggingGrafanaCloudLogs)

		if err := loggingsplunk.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingSplunk); err != nil {
			resp.Diagnostics.AddError("Error reconciling Splunk logging endpoints", err.Error())
			return
		}

		loggingSplunks, err := loggingsplunk.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Splunk logging endpoints", err.Error())
			return
		}
		plan.LoggingSplunk = loggingsplunk.ComputeMatchOrder(loggingSplunks, plan.LoggingSplunk)

		if err := logginghttps.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingHTTPS); err != nil {
			resp.Diagnostics.AddError("Error reconciling HTTPS logging endpoints", err.Error())
			return
		}

		loggingHTTPS, err := logginghttps.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading HTTPS logging endpoints", err.Error())
			return
		}
		plan.LoggingHTTPS = logginghttps.ComputeMatchOrder(loggingHTTPS, plan.LoggingHTTPS)

		if err := loggingsumologic.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingSumologic); err != nil {
			resp.Diagnostics.AddError("Error reconciling Sumologic logging endpoints", err.Error())
			return
		}

		loggingSumologics, err := loggingsumologic.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Sumologic logging endpoints", err.Error())
			return
		}
		plan.LoggingSumologic = loggingsumologic.ComputeMatchOrder(loggingSumologics, plan.LoggingSumologic)

		if err := loggingsyslog.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingSyslog); err != nil {
			resp.Diagnostics.AddError("Error reconciling Syslog logging endpoints", err.Error())
			return
		}

		loggingSyslogs, err := loggingsyslog.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Syslog logging endpoints", err.Error())
			return
		}
		plan.LoggingSyslog = loggingsyslog.ComputeMatchOrder(loggingSyslogs, plan.LoggingSyslog)

		if err := loggingkafka.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingKafka); err != nil {
			resp.Diagnostics.AddError("Error reconciling Kafka logging endpoints", err.Error())
			return
		}

		loggingKafkas, err := loggingkafka.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Kafka logging endpoints", err.Error())
			return
		}
		plan.LoggingKafka = loggingkafka.ComputeMatchOrder(loggingKafkas, plan.LoggingKafka)

		if err := loggingkinesis.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingKinesis); err != nil {
			resp.Diagnostics.AddError("Error reconciling Kinesis logging endpoints", err.Error())
			return
		}

		loggingKineses, err := loggingkinesis.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Kinesis logging endpoints", err.Error())
			return
		}
		plan.LoggingKinesis = loggingkinesis.ComputeMatchOrder(loggingKineses, plan.LoggingKinesis)

		if err := loggingloggly.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingLoggly); err != nil {
			resp.Diagnostics.AddError("Error reconciling Loggly logging endpoints", err.Error())
			return
		}

		loggingLogglys, err := loggingloggly.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Loggly logging endpoints", err.Error())
			return
		}
		plan.LoggingLoggly = loggingloggly.ComputeMatchOrder(loggingLogglys, plan.LoggingLoggly)

		if err := logginglogshuttle.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingLogshuttle); err != nil {
			resp.Diagnostics.AddError("Error reconciling Log Shuttle logging endpoints", err.Error())
			return
		}

		loggingLogshuttles, err := logginglogshuttle.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Log Shuttle logging endpoints", err.Error())
			return
		}
		plan.LoggingLogshuttle = logginglogshuttle.ComputeMatchOrder(loggingLogshuttles, plan.LoggingLogshuttle)

		if err := loggingpapertrail.ComputeReconcile(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.LoggingPapertrail); err != nil {
			resp.Diagnostics.AddError("Error reconciling Papertrail logging endpoints", err.Error())
			return
		}

		loggingPapertrails, err := loggingpapertrail.ComputeReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Papertrail logging endpoints", err.Error())
			return
		}
		plan.LoggingPapertrail = loggingpapertrail.ComputeMatchOrder(loggingPapertrails, plan.LoggingPapertrail)

		if len(state.Package) > 0 && len(plan.Package) == 0 {
			resp.Diagnostics.AddError(
				"Removing Compute packages is not supported",
				"Deleting a package from a service version is not currently supported. Provide a package block or create a new service/version workflow that does not rely on package removal.",
			)
			return
		}

		if err := computepackage.Update(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.Package); err != nil {
			resp.Diagnostics.AddError("Error updating Compute package", err.Error())
			return
		}

		packages, err := computepackage.ReadForVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion, plan.Package, false)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Compute package", err.Error())
			return
		}
		plan.Package = packages

		if err := service.ValidateVersion(ctx, r.providerData.AutoClient(), serviceID, targetVersion); err != nil {
			resp.Diagnostics.AddError("Error validating service version", err.Error())
			return
		}

		plan.ManagedVersion = types.Int64Value(int64(targetVersion))

		if _, err := r.providerData.AutoClient().ActivateVersion(ctx, &fastly.ActivateVersionInput{
			ServiceID:      serviceID,
			ServiceVersion: targetVersion,
		}); err != nil {
			resp.Diagnostics.AddError("Error activating service version", err.Error())
			return
		}
		plan.ActiveVersion = types.Int64Value(int64(targetVersion))
	} else {
		// No version change needed - preserve version numbers and order nested state to match the plan
		plan.ManagedVersion = state.ManagedVersion
		plan.ActiveVersion = state.ActiveVersion
		plan.Domain = domain.MatchOrder(state.Domain, plan.Domain)
		plan.HealthCheck = healthcheck.MatchOrder(state.HealthCheck, plan.HealthCheck)
		plan.Backend = backend.MatchOrder(state.Backend, plan.Backend)
		plan.Dictionary = dictionary.MatchOrder(state.Dictionary, plan.Dictionary)
		plan.ResourceLink = resourcelink.MatchOrder(state.ResourceLink, plan.ResourceLink)
		plan.Package = state.Package
		plan.LoggingBlobStorage = loggingblobstorage.ComputeMatchOrder(state.LoggingBlobStorage, plan.LoggingBlobStorage)
		plan.LoggingCloudfiles = loggingcloudfiles.ComputeMatchOrder(state.LoggingCloudfiles, plan.LoggingCloudfiles)
		plan.LoggingOpenStack = loggingopenstack.ComputeMatchOrder(state.LoggingOpenStack, plan.LoggingOpenStack)
		plan.LoggingDigitalOcean = loggingdigitalocean.ComputeMatchOrder(state.LoggingDigitalOcean, plan.LoggingDigitalOcean)
		plan.LoggingElasticsearch = loggingelasticsearch.ComputeMatchOrder(state.LoggingElasticsearch, plan.LoggingElasticsearch)
		plan.LoggingFTP = loggingftp.ComputeMatchOrder(state.LoggingFTP, plan.LoggingFTP)
		plan.LoggingS3 = loggings3.ComputeMatchOrder(state.LoggingS3, plan.LoggingS3)
		plan.LoggingScalyr = loggingscalyr.ComputeMatchOrder(state.LoggingScalyr, plan.LoggingScalyr)
		plan.LoggingSFTP = loggingsftp.ComputeMatchOrder(state.LoggingSFTP, plan.LoggingSFTP)
		plan.LoggingNewRelicOTLP = loggingnewrelicotlp.ComputeMatchOrder(state.LoggingNewRelicOTLP, plan.LoggingNewRelicOTLP)
		plan.LoggingNewRelic = loggingnewrelic.ComputeMatchOrder(state.LoggingNewRelic, plan.LoggingNewRelic)
		plan.LoggingHeroku = loggingheroku.ComputeMatchOrder(state.LoggingHeroku, plan.LoggingHeroku)
		plan.LoggingDatadog = loggingdatadog.ComputeMatchOrder(state.LoggingDatadog, plan.LoggingDatadog)
		plan.LoggingHoneycomb = logginghoneycomb.ComputeMatchOrder(state.LoggingHoneycomb, plan.LoggingHoneycomb)
		plan.LoggingBigQuery = loggingbigquery.ComputeMatchOrder(state.LoggingBigQuery, plan.LoggingBigQuery)
		plan.LoggingGCS = logginggcs.ComputeMatchOrder(state.LoggingGCS, plan.LoggingGCS)
		plan.LoggingGooglePubSub = logginggooglepubsub.ComputeMatchOrder(state.LoggingGooglePubSub, plan.LoggingGooglePubSub)
		plan.LoggingGrafanaCloudLogs = logginggrafanacloudlogs.ComputeMatchOrder(state.LoggingGrafanaCloudLogs, plan.LoggingGrafanaCloudLogs)
		plan.LoggingSplunk = loggingsplunk.ComputeMatchOrder(state.LoggingSplunk, plan.LoggingSplunk)
		plan.LoggingHTTPS = logginghttps.ComputeMatchOrder(state.LoggingHTTPS, plan.LoggingHTTPS)
		plan.LoggingSumologic = loggingsumologic.ComputeMatchOrder(state.LoggingSumologic, plan.LoggingSumologic)
		plan.LoggingSyslog = loggingsyslog.ComputeMatchOrder(state.LoggingSyslog, plan.LoggingSyslog)
		plan.LoggingKafka = loggingkafka.ComputeMatchOrder(state.LoggingKafka, plan.LoggingKafka)
		plan.LoggingKinesis = loggingkinesis.ComputeMatchOrder(state.LoggingKinesis, plan.LoggingKinesis)
		plan.LoggingLoggly = loggingloggly.ComputeMatchOrder(state.LoggingLoggly, plan.LoggingLoggly)
		plan.LoggingLogshuttle = logginglogshuttle.ComputeMatchOrder(state.LoggingLogshuttle, plan.LoggingLogshuttle)
		plan.LoggingPapertrail = loggingpapertrail.ComputeMatchOrder(state.LoggingPapertrail, plan.LoggingPapertrail)
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := service.DeleteWithPolicy(
		ctx,
		r.providerData.AutoClient(),
		state.ID.ValueString(),
		service.BoolValue(state.ForceDestroy),
		service.BoolValue(state.Reuse),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Fastly Compute service", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, computePackageImportedPrivateKey, []byte("true"))...)
}

func (r *Resource) selectWorkingVersion(ctx context.Context, serviceID string) (version int, shouldClone bool, err error) {
	details, err := r.providerData.AutoClient().GetServiceDetails(ctx, &fastly.GetServiceDetailsInput{
		ServiceID: serviceID,
	})
	if err != nil {
		return 0, false, err
	}

	return service.SelectWorkingVersionFromDetails(details, serviceID)
}
