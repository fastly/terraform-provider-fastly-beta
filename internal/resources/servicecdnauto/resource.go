package servicecdnauto

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/errors"
	"github.com/fastly/terraform-provider-fastly/internal/reconcile"
	"github.com/fastly/terraform-provider-fastly/internal/resources/backend"
	"github.com/fastly/terraform-provider-fastly/internal/resources/cachesetting"
	"github.com/fastly/terraform-provider-fastly/internal/resources/cdnacl"
	"github.com/fastly/terraform-provider-fastly/internal/resources/condition"
	"github.com/fastly/terraform-provider-fastly/internal/resources/dictionary"
	"github.com/fastly/terraform-provider-fastly/internal/resources/director"
	"github.com/fastly/terraform-provider-fastly/internal/resources/domain"
	"github.com/fastly/terraform-provider-fastly/internal/resources/dynamicsnippet"
	"github.com/fastly/terraform-provider-fastly/internal/resources/gzip"
	"github.com/fastly/terraform-provider-fastly/internal/resources/header"
	"github.com/fastly/terraform-provider-fastly/internal/resources/healthcheck"
	"github.com/fastly/terraform-provider-fastly/internal/resources/imageoptimizerdefaultsettings"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggingbigquery"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggingblobstorage"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggingdatadog"
	"github.com/fastly/terraform-provider-fastly/internal/resources/logginggcs"
	"github.com/fastly/terraform-provider-fastly/internal/resources/logginghttps"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggingnewrelic"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggingnewrelicotlp"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggings3"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggingsplunk"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggingsumologic"
	"github.com/fastly/terraform-provider-fastly/internal/resources/loggingsyslog"
	"github.com/fastly/terraform-provider-fastly/internal/resources/ratelimiter"
	"github.com/fastly/terraform-provider-fastly/internal/resources/requestsetting"
	"github.com/fastly/terraform-provider-fastly/internal/resources/responseobject"
	"github.com/fastly/terraform-provider-fastly/internal/resources/settings"
	"github.com/fastly/terraform-provider-fastly/internal/resources/snippet"
	"github.com/fastly/terraform-provider-fastly/internal/resources/vcl"
	"github.com/fastly/terraform-provider-fastly/internal/service"
	"github.com/fastly/terraform-provider-fastly/internal/validation"

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

// imageOptimizerImportedPrivateKey marks, via private state, that the current Read follows an
// import. ImportStatePassthroughID leaves ImageOptimizerDefaultSettings empty in state (only "id"
// is set), so without this marker a just-imported service with the block configured remotely
// would incorrectly read back as absent - see ReadForVersion's forceRefresh parameter.
const imageOptimizerImportedPrivateKey = "image_optimizer_default_settings_imported"

var (
	_ resource.Resource                   = &Resource{}
	_ resource.ResourceWithConfigure      = &Resource{}
	_ resource.ResourceWithImportState    = &Resource{}
	_ resource.ResourceWithValidateConfig = &Resource{}
)

func NewResource() resource.Resource {
	return &Resource{}
}

type Model struct {
	ID                            types.String                                `tfsdk:"id"`
	Name                          types.String                                `tfsdk:"name"`
	Comment                       types.String                                `tfsdk:"comment"`
	ForceDestroy                  types.Bool                                  `tfsdk:"force_destroy"`
	Reuse                         types.Bool                                  `tfsdk:"reuse"`
	ActiveVersion                 types.Int64                                 `tfsdk:"active_version"`
	ManagedVersion                types.Int64                                 `tfsdk:"managed_version"`
	Settings                      []settings.NestedModel                      `tfsdk:"settings"`
	Domain                        []domain.NestedModel                        `tfsdk:"domain"`
	Backend                       []backend.NestedModel                       `tfsdk:"backend"`
	Director                      []director.NestedModel                      `tfsdk:"director"`
	ACL                           []cdnacl.NestedModel                        `tfsdk:"acl"`
	Condition                     []condition.NestedModel                     `tfsdk:"condition"`
	HealthCheck                   []healthcheck.NestedModel                   `tfsdk:"healthcheck"`
	Header                        []header.NestedModel                        `tfsdk:"header"`
	Gzip                          []gzip.NestedModel                          `tfsdk:"gzip"`
	CacheSetting                  []cachesetting.NestedModel                  `tfsdk:"cache_setting"`
	RequestSetting                []requestsetting.NestedModel                `tfsdk:"request_setting"`
	ResponseObject                []responseobject.NestedModel                `tfsdk:"response_object"`
	Dictionary                    []dictionary.NestedModel                    `tfsdk:"dictionary"`
	RateLimiter                   []ratelimiter.NestedModel                   `tfsdk:"rate_limiter"`
	LoggingBlobStorage            []loggingblobstorage.NestedModel            `tfsdk:"logging_blobstorage"`
	LoggingS3                     []loggings3.NestedModel                     `tfsdk:"logging_s3"`
	LoggingNewRelicOTLP           []loggingnewrelicotlp.NestedModel           `tfsdk:"logging_newrelicotlp"`
	LoggingNewRelic               []loggingnewrelic.NestedModel               `tfsdk:"logging_newrelic"`
	LoggingDatadog                []loggingdatadog.NestedModel                `tfsdk:"logging_datadog"`
	LoggingBigQuery               []loggingbigquery.NestedModel               `tfsdk:"logging_bigquery"`
	LoggingGCS                    []logginggcs.NestedModel                    `tfsdk:"logging_gcs"`
	LoggingSplunk                 []loggingsplunk.NestedModel                 `tfsdk:"logging_splunk"`
	LoggingHTTPS                  []logginghttps.NestedModel                  `tfsdk:"logging_https"`
	LoggingSumologic              []loggingsumologic.NestedModel              `tfsdk:"logging_sumologic"`
	LoggingSyslog                 []loggingsyslog.NestedModel                 `tfsdk:"logging_syslog"`
	ImageOptimizerDefaultSettings []imageoptimizerdefaultsettings.NestedModel `tfsdk:"image_optimizer_default_settings"`
	Snippet                       []snippet.NestedModel                       `tfsdk:"snippet"`
	DynamicSnippet                []dynamicsnippet.NestedModel                `tfsdk:"dynamic_snippet"`
	VCL                           []vcl.NestedModel                           `tfsdk:"vcl"`
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_cdn_auto"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Automatic-lifecycle Fastly CDN service resource with nested versioned configuration. The provider automatically clones, validates, and activates changed versions.",
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
			"settings":                         settings.NestedBlockSchema(),
			"domain":                           domain.NestedBlockSchema(),
			"backend":                          backend.NestedBlockSchema(),
			"director":                         director.NestedBlockSchema(),
			"acl":                              cdnacl.NestedBlockSchema(),
			"condition":                        condition.NestedBlockSchema(),
			"healthcheck":                      healthcheck.NestedBlockSchema(),
			"header":                           header.NestedBlockSchema(),
			"gzip":                             gzip.NestedBlockSchema(),
			"cache_setting":                    cachesetting.NestedBlockSchema(),
			"request_setting":                  requestsetting.NestedBlockSchema(),
			"response_object":                  responseobject.NestedBlockSchema(),
			"dictionary":                       dictionary.NestedBlockSchema(),
			"rate_limiter":                     ratelimiter.NestedBlockSchema(),
			"logging_blobstorage":              loggingblobstorage.NestedBlockSchema(),
			"logging_s3":                       loggings3.NestedBlockSchema(),
			"logging_newrelicotlp":             loggingnewrelicotlp.NestedBlockSchema(),
			"logging_newrelic":                 loggingnewrelic.NestedBlockSchema(),
			"logging_datadog":                  loggingdatadog.NestedBlockSchema(),
			"logging_bigquery":                 loggingbigquery.NestedBlockSchema(),
			"logging_gcs":                      logginggcs.NestedBlockSchema(),
			"logging_splunk":                   loggingsplunk.NestedBlockSchema(),
			"logging_https":                    logginghttps.NestedBlockSchema(),
			"logging_sumologic":                loggingsumologic.NestedBlockSchema(),
			"logging_syslog":                   loggingsyslog.NestedBlockSchema(),
			"image_optimizer_default_settings": imageoptimizerdefaultsettings.NestedBlockSchema(),
			"snippet":                          snippet.NestedBlockSchema(),
			"dynamic_snippet":                  dynamicsnippet.NestedBlockSchema(),
			"vcl":                              vcl.NestedBlockSchema(),
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

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := snippet.ValidateConfig(config.Snippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("snippet"),
			"Invalid VCL snippet configuration",
			err.Error(),
		)
	}

	if err := dynamicsnippet.ValidateConfig(config.DynamicSnippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("dynamic_snippet"),
			"Invalid dynamic VCL snippet configuration",
			err.Error(),
		)
	}

	if err := dynamicsnippet.ValidateNoNameConflicts(config.DynamicSnippet, config.Snippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("dynamic_snippet"),
			"Invalid VCL snippet configuration",
			err.Error(),
		)
	}

	if err := vcl.ValidateConfig(config.VCL); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("vcl"),
			"Invalid custom VCL configuration",
			err.Error(),
		)
	}

	if err := ratelimiter.ValidateConfig(config.RateLimiter); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("rate_limiter"),
			"Invalid rate limiter configuration",
			err.Error(),
		)
	}

	if err := ratelimiter.ValidateDictionaryReferences(config.RateLimiter, config.Dictionary); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("rate_limiter"),
			"Invalid rate limiter configuration",
			err.Error(),
		)
	}

	if err := director.ValidateConfig(config.Director); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("director"),
			"Invalid director configuration",
			err.Error(),
		)
	}

	if err := director.ValidateBackendReferences(config.Director, config.Backend); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("director"),
			"Invalid director configuration",
			err.Error(),
		)
	}

	if err := ratelimiter.ValidateResponseObjectReferences(config.RateLimiter, config.ResponseObject); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("rate_limiter"),
			"Invalid rate limiter configuration",
			err.Error(),
		)
	}

	if err := backend.ValidateHealthcheckReferences(config.Backend, config.HealthCheck); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("backend"),
			"Invalid backend configuration",
			err.Error(),
		)
	}

	conditionNames := validation.NameSet(config.Condition, func(m condition.NestedModel) types.String { return m.Name })

	if err := backend.ValidateConditionReferences(config.Backend, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("backend"),
			"Invalid backend configuration",
			err.Error(),
		)
	}

	if err := cachesetting.ValidateConditionReferences(config.CacheSetting, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("cache_setting"),
			"Invalid cache setting configuration",
			err.Error(),
		)
	}

	if err := gzip.ValidateConditionReferences(config.Gzip, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("gzip"),
			"Invalid gzip configuration",
			err.Error(),
		)
	}

	if err := header.ValidateConditionReferences(config.Header, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("header"),
			"Invalid header configuration",
			err.Error(),
		)
	}

	if err := requestsetting.ValidateConditionReferences(config.RequestSetting, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("request_setting"),
			"Invalid request setting configuration",
			err.Error(),
		)
	}

	if err := responseobject.ValidateConditionReferences(config.ResponseObject, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("response_object"),
			"Invalid response object configuration",
			err.Error(),
		)
	}

	if err := loggingbigquery.ValidateConditionReferences(config.LoggingBigQuery, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_bigquery"),
			"Invalid BigQuery logging configuration",
			err.Error(),
		)
	}

	if err := loggingblobstorage.ValidateConditionReferences(config.LoggingBlobStorage, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_blobstorage"),
			"Invalid Blob Storage logging configuration",
			err.Error(),
		)
	}

	if err := loggingdatadog.ValidateConditionReferences(config.LoggingDatadog, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_datadog"),
			"Invalid Datadog logging configuration",
			err.Error(),
		)
	}

	if err := logginggcs.ValidateConditionReferences(config.LoggingGCS, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_gcs"),
			"Invalid GCS logging configuration",
			err.Error(),
		)
	}

	if err := logginghttps.ValidateConditionReferences(config.LoggingHTTPS, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_https"),
			"Invalid HTTPS logging configuration",
			err.Error(),
		)
	}

	if err := loggingnewrelic.ValidateConditionReferences(config.LoggingNewRelic, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_newrelic"),
			"Invalid New Relic logging configuration",
			err.Error(),
		)
	}

	if err := loggingnewrelicotlp.ValidateConditionReferences(config.LoggingNewRelicOTLP, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_newrelicotlp"),
			"Invalid New Relic OTLP logging configuration",
			err.Error(),
		)
	}

	if err := loggings3.ValidateConditionReferences(config.LoggingS3, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_s3"),
			"Invalid S3 logging configuration",
			err.Error(),
		)
	}

	if err := loggingsplunk.ValidateConditionReferences(config.LoggingSplunk, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_splunk"),
			"Invalid Splunk logging configuration",
			err.Error(),
		)
	}

	if err := loggingsumologic.ValidateConditionReferences(config.LoggingSumologic, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_sumologic"),
			"Invalid Sumologic logging configuration",
			err.Error(),
		)
	}

	if err := loggingsyslog.ValidateConditionReferences(config.LoggingSyslog, conditionNames); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("logging_syslog"),
			"Invalid Syslog logging configuration",
			err.Error(),
		)
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := snippet.Validate(plan.Snippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("snippet"),
			"Invalid VCL snippet configuration",
			err.Error(),
		)
		return
	}

	if err := dynamicsnippet.Validate(plan.DynamicSnippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("dynamic_snippet"),
			"Invalid dynamic VCL snippet configuration",
			err.Error(),
		)
		return
	}

	if err := dynamicsnippet.ValidateNoNameConflicts(plan.DynamicSnippet, plan.Snippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("dynamic_snippet"),
			"Invalid VCL snippet configuration",
			err.Error(),
		)
		return
	}

	if err := vcl.Validate(plan.VCL); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("vcl"),
			"Invalid custom VCL configuration",
			err.Error(),
		)
		return
	}

	created, err := r.providerData.AutoClient().CreateService(ctx, &fastly.CreateServiceInput{
		Name:    new(plan.Name.ValueString()),
		Comment: new(plan.Comment.ValueString()),
		Type:    new(service.TypeVCL),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Fastly CDN service", err.Error())
		return
	}

	serviceID := fastly.ToValue(created.ServiceID)
	version := 1

	tflog.Info(ctx, "Created Fastly CDN service", map[string]any{
		"service_id": serviceID,
		"version":    version,
	})

	client := r.providerData.AutoClient()
	previous := &Model{}

	if label, phase, err := runMutateSteps(ctx, client, serviceID, version, beforeBackendAndDirectorSteps(&plan, previous)); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error %s %s", phase, label), err.Error())
		return
	}

	if err := backend.Reconcile(ctx, client, serviceID, version, plan.Backend); err != nil {
		resp.Diagnostics.AddError("Error reconciling backends", err.Error())
		return
	}

	backends, err := backend.ReadForVersion(ctx, client, serviceID, version)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service backends", err.Error())
		return
	}
	plan.Backend = backend.MatchOrder(backends, plan.Backend)

	// Directors reconcile after backends: a director's backends can reference one by name, and
	// creating a director naming one that doesn't exist yet fails.
	if err := director.Reconcile(ctx, client, serviceID, version, plan.Director); err != nil {
		resp.Diagnostics.AddError("Error reconciling directors", err.Error())
		return
	}

	directors, err := director.ReadForVersion(ctx, client, serviceID, version)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service directors", err.Error())
		return
	}
	plan.Director = director.MatchOrder(directors, plan.Director)

	if label, phase, err := runMutateSteps(ctx, client, serviceID, version, beforeDictionaryAndRateLimiterSteps(&plan, previous)); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error %s %s", phase, label), err.Error())
		return
	}

	if err := dictionary.ReconcileWithPrevious(ctx, client, serviceID, version, nil, plan.Dictionary); err != nil {
		resp.Diagnostics.AddError("Error reconciling dictionaries", err.Error())
		return
	}

	dictionaries, err := dictionary.ReadForVersionWithPlan(ctx, client, serviceID, version, plan.Dictionary)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service dictionaries", err.Error())
		return
	}
	plan.Dictionary = dictionary.MatchOrder(dictionaries, plan.Dictionary)

	// Rate limiters reconcile after dictionaries: uri_dictionary_name can reference one by name,
	// and creating a rate limiter naming one that doesn't exist yet fails.
	if err := ratelimiter.Reconcile(ctx, client, serviceID, version, plan.RateLimiter); err != nil {
		resp.Diagnostics.AddError("Error reconciling rate limiters", err.Error())
		return
	}

	rateLimiters, err := ratelimiter.ReadForVersion(ctx, client, serviceID, version)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service rate limiters", err.Error())
		return
	}
	plan.RateLimiter = ratelimiter.MatchOrder(rateLimiters, plan.RateLimiter)

	if label, phase, err := runMutateSteps(ctx, client, serviceID, version, afterDictionaryAndRateLimiterSteps(&plan, previous)); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error %s %s", phase, label), err.Error())
		return
	}

	if err := service.ValidateVersion(ctx, r.providerData.AutoClient(), serviceID, version); err != nil {
		resp.Diagnostics.AddError("Error validating service version", err.Error())
		return
	}

	plan.ID = types.StringValue(serviceID)
	plan.ManagedVersion = types.Int64Value(int64(version))

	if _, err := r.providerData.AutoClient().ActivateVersion(ctx, &fastly.ActivateVersionInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	}); err != nil {
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
		resp.Diagnostics.AddError("Error reading Fastly CDN service", err.Error())
		return
	}

	serviceType := fastly.ToValue(details.Type)
	if serviceType != service.TypeVCL {
		resp.Diagnostics.AddError(
			"Unexpected Fastly service type",
			fmt.Sprintf("Expected VCL service %q to have type %q, got %q.", state.ID.ValueString(), service.TypeVCL, serviceType),
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

	importedBytes, diags := req.Private.GetKey(ctx, imageOptimizerImportedPrivateKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	imported := len(importedBytes) > 0
	if imported {
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, imageOptimizerImportedPrivateKey, nil)...)
	}

	if label, err := runReadSteps(ctx, r.providerData.AutoClient(), state.ID.ValueString(), readVersion, readSteps(&state, imported)); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading %s", label), err.Error())
		return
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

	if err := snippet.Validate(plan.Snippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("snippet"),
			"Invalid VCL snippet configuration",
			err.Error(),
		)
		return
	}

	if err := dynamicsnippet.Validate(plan.DynamicSnippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("dynamic_snippet"),
			"Invalid dynamic VCL snippet configuration",
			err.Error(),
		)
		return
	}

	if err := dynamicsnippet.ValidateNoNameConflicts(plan.DynamicSnippet, plan.Snippet); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("dynamic_snippet"),
			"Invalid VCL snippet configuration",
			err.Error(),
		)
		return
	}

	if err := vcl.Validate(plan.VCL); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("vcl"),
			"Invalid custom VCL configuration",
			err.Error(),
		)
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
		resp.Diagnostics.AddError("Error updating Fastly CDN service", err.Error())
		return
	}

	steps := planSteps(&plan, &state)
	nestedChanged := false
	for _, s := range steps {
		if !s.equal() {
			nestedChanged = true
			break
		}
	}
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

		tflog.Info(ctx, "Selected Fastly CDN service working version", map[string]any{
			"service_id":     serviceID,
			"source_version": sourceVersion,
			"target_version": targetVersion,
			"cloned":         shouldClone,
			"nested_changed": nestedChanged,
		})

		client := r.providerData.AutoClient()

		if label, phase, err := runMutateSteps(ctx, client, serviceID, targetVersion, beforeBackendAndDirectorSteps(&plan, &state)); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error %s %s", phase, label), err.Error())
			return
		}

		// Backends and directors reconcile as a ThreePassUpdate pair: a backend create must run
		// before the director referencing it, but a backend delete must wait until directors no
		// longer reference it.
		if err := reconcile.ThreePassUpdate(
			len(plan.Director) > 0 || len(state.Director) > 0,
			func() error {
				if err := backend.Reconcile(ctx, client, serviceID, targetVersion, plan.Backend); err != nil {
					resp.Diagnostics.AddError("Error reconciling backends", err.Error())
					return err
				}
				return nil
			},
			func() error {
				if err := backend.CreateOrUpdate(ctx, client, serviceID, targetVersion, plan.Backend); err != nil {
					resp.Diagnostics.AddError("Error creating or updating backends", err.Error())
					return err
				}
				return nil
			},
			func() error {
				if err := director.Reconcile(ctx, client, serviceID, targetVersion, plan.Director); err != nil {
					resp.Diagnostics.AddError("Error reconciling directors", err.Error())
					return err
				}

				directors, err := director.ReadForVersion(ctx, client, serviceID, targetVersion)
				if err != nil {
					resp.Diagnostics.AddError("Error reading service directors", err.Error())
					return err
				}
				plan.Director = director.MatchOrder(directors, plan.Director)
				return nil
			},
			func() error {
				if err := backend.DeleteRemoved(ctx, client, serviceID, targetVersion, plan.Backend); err != nil {
					resp.Diagnostics.AddError("Error deleting removed backends", err.Error())
					return err
				}
				return nil
			},
		); err != nil {
			return
		}

		backends, err := backend.ReadForVersion(ctx, client, serviceID, targetVersion)
		if err != nil {
			resp.Diagnostics.AddError("Error reading service backends", err.Error())
			return
		}
		plan.Backend = backend.MatchOrder(backends, plan.Backend)

		if label, phase, err := runMutateSteps(ctx, client, serviceID, targetVersion, beforeDictionaryAndRateLimiterSteps(&plan, &state)); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error %s %s", phase, label), err.Error())
			return
		}

		// Dictionaries and rate limiters reconcile as a ThreePassUpdate pair: a rate limiter
		// create needs its dictionary first, but a dictionary delete must wait until rate
		// limiters stop referencing it. CheckRemovalGuards runs first regardless, guarding
		// dictionary-item loss - a separate concern from that ordering.
		if err := dictionary.CheckRemovalGuards(ctx, client, serviceID, state.Dictionary, plan.Dictionary); err != nil {
			resp.Diagnostics.AddError("Error reconciling dictionaries", err.Error())
			return
		}

		if err := reconcile.ThreePassUpdate(
			len(plan.RateLimiter) > 0 || len(state.RateLimiter) > 0,
			func() error {
				if err := dictionary.Reconcile(ctx, client, serviceID, targetVersion, plan.Dictionary); err != nil {
					resp.Diagnostics.AddError("Error reconciling dictionaries", err.Error())
					return err
				}
				return nil
			},
			func() error {
				if err := dictionary.CreateOrUpdate(ctx, client, serviceID, targetVersion, plan.Dictionary); err != nil {
					resp.Diagnostics.AddError("Error reconciling dictionaries", err.Error())
					return err
				}
				return nil
			},
			func() error {
				if err := ratelimiter.Reconcile(ctx, client, serviceID, targetVersion, plan.RateLimiter); err != nil {
					resp.Diagnostics.AddError("Error reconciling rate limiters", err.Error())
					return err
				}

				rateLimiters, err := ratelimiter.ReadForVersion(ctx, client, serviceID, targetVersion)
				if err != nil {
					resp.Diagnostics.AddError("Error reading service rate limiters", err.Error())
					return err
				}
				plan.RateLimiter = ratelimiter.MatchOrder(rateLimiters, plan.RateLimiter)
				return nil
			},
			func() error {
				if err := dictionary.DeleteRemoved(ctx, client, serviceID, targetVersion, plan.Dictionary); err != nil {
					resp.Diagnostics.AddError("Error reconciling dictionaries", err.Error())
					return err
				}
				return nil
			},
		); err != nil {
			return
		}

		dictionaries, err := dictionary.ReadForVersionWithPlan(ctx, client, serviceID, targetVersion, plan.Dictionary)
		if err != nil {
			resp.Diagnostics.AddError("Error reading service dictionaries", err.Error())
			return
		}
		plan.Dictionary = dictionary.MatchOrder(dictionaries, plan.Dictionary)

		if label, phase, err := runMutateSteps(ctx, client, serviceID, targetVersion, afterDictionaryAndRateLimiterSteps(&plan, &state)); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error %s %s", phase, label), err.Error())
			return
		}

		if err := service.ValidateVersion(ctx, client, serviceID, targetVersion); err != nil {
			resp.Diagnostics.AddError("Error validating service version", err.Error())
			return
		}

		plan.ManagedVersion = types.Int64Value(int64(targetVersion))

		if _, err := client.ActivateVersion(ctx, &fastly.ActivateVersionInput{
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

		for _, s := range steps {
			s.matchOnly()
		}
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
		resp.Diagnostics.AddError("Error deleting Fastly CDN service", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, imageOptimizerImportedPrivateKey, []byte("true"))...)
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
