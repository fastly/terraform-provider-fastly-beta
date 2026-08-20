package servicecdnauto

import (
	"context"

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

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

// mutateStep reconciles one nested resource type, then reads the result back into plan. Builder
// functions bind these per (plan, previous) via closures, since each resource package's
// Reconcile/ReadForVersion/MatchOrder types differ.
type mutateStep struct {
	label     string
	reconcile func(ctx context.Context, client *fastly.Client, serviceID string, version int) error
	readBack  func(ctx context.Context, client *fastly.Client, serviceID string, version int) error
}

// runMutateSteps runs each step's reconcile then readBack, in slice order, stopping at the first
// error. label/phase identify which step and which half failed, for the caller's diagnostic.
func runMutateSteps(ctx context.Context, client *fastly.Client, serviceID string, version int, steps []mutateStep) (label, phase string, err error) {
	for _, s := range steps {
		if err := s.reconcile(ctx, client, serviceID, version); err != nil {
			return s.label, "reconciling", err
		}
		if err := s.readBack(ctx, client, serviceID, version); err != nil {
			return s.label, "reading", err
		}
	}
	return "", "", nil
}

// readStep reads one nested resource type back and assigns it into state. Used only by Read,
// which has no cross-type ordering constraint, so all 28 types share one flat list.
type readStep struct {
	label string
	run   func(ctx context.Context, client *fastly.Client, serviceID string, version int) error
}

func runReadSteps(ctx context.Context, client *fastly.Client, serviceID string, version int, steps []readStep) (label string, err error) {
	for _, s := range steps {
		if err := s.run(ctx, client, serviceID, version); err != nil {
			return s.label, err
		}
	}
	return "", nil
}

// planStep captures a nested resource type's Equal and reorder-only MatchOrder calls
// so ordering doesn't matter and all 28 types share one list.
type planStep struct {
	equal     func() bool
	matchOnly func()
}

// beforeBackendAndDirectorSteps: domain and condition have no dependencies, and healthcheck
// must precede backend since a backend can reference a health check by name.
func beforeBackendAndDirectorSteps(plan, previous *Model) []mutateStep {
	return []mutateStep{
		{
			label: "settings",
			// settings.Reconcile already returns the reconciled result
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				result, err := settings.Reconcile(ctx, client, serviceID, version, previous.Settings, plan.Settings)
				if err != nil {
					return err
				}
				plan.Settings = result
				return nil
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return nil
			},
		},
		{
			label: "domains",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return domain.Reconcile(ctx, client, serviceID, version, plan.Domain)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := domain.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.Domain = domain.MatchOrder(items, plan.Domain)
				return nil
			},
		},
		{
			label: "conditions",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return condition.Reconcile(ctx, client, serviceID, version, plan.Condition)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := condition.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.Condition = condition.MatchOrder(items, plan.Condition)
				return nil
			},
		},
		{
			label: "health checks",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return healthcheck.Reconcile(ctx, client, serviceID, version, plan.HealthCheck)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := healthcheck.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.HealthCheck = healthcheck.MatchOrder(items, plan.HealthCheck)
				return nil
			},
		},
	}
}

// beforeDictionaryAndRateLimiterSteps: none of these six depend on each other or on
// backend/director.
func beforeDictionaryAndRateLimiterSteps(plan, previous *Model) []mutateStep {
	return []mutateStep{
		{
			label: "ACLs",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return cdnacl.ReconcileWithPrevious(ctx, client, serviceID, version, previous.ACL, plan.ACL)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				acls, err := cdnacl.ReadForVersionWithPlan(ctx, client, serviceID, version, plan.ACL)
				if err != nil {
					return err
				}
				plan.ACL = acls
				return nil
			},
		},
		{
			label: "headers",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return header.Reconcile(ctx, client, serviceID, version, plan.Header)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := header.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.Header = header.MatchOrder(items, plan.Header)
				return nil
			},
		},
		{
			label: "gzip configurations",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return gzip.ReconcileWithPrevious(ctx, client, serviceID, version, previous.Gzip, plan.Gzip)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := gzip.ReadForVersionWithPlan(ctx, client, serviceID, version, plan.Gzip)
				if err != nil {
					return err
				}
				plan.Gzip = gzip.MatchOrder(items, plan.Gzip)
				return nil
			},
		},
		{
			label: "cache settings",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return cachesetting.Reconcile(ctx, client, serviceID, version, plan.CacheSetting)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := cachesetting.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.CacheSetting = cachesetting.MatchOrder(items, plan.CacheSetting)
				return nil
			},
		},
		{
			label: "request settings",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return requestsetting.Reconcile(ctx, client, serviceID, version, plan.RequestSetting)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := requestsetting.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.RequestSetting = requestsetting.MatchOrder(items, plan.RequestSetting)
				return nil
			},
		},
		{
			label: "response objects",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return responseobject.Reconcile(ctx, client, serviceID, version, plan.ResponseObject)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := responseobject.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.ResponseObject = responseobject.MatchOrder(items, plan.ResponseObject)
				return nil
			},
		},
	}
}

// afterDictionaryAndRateLimiterSteps: none of the logging endpoints, image optimizer settings,
// or VCL types depend on anything else.
func afterDictionaryAndRateLimiterSteps(plan, previous *Model) []mutateStep {
	return []mutateStep{
		{
			label: "Blob Storage logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggingblobstorage.Reconcile(ctx, client, serviceID, version, plan.LoggingBlobStorage)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingblobstorage.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingBlobStorage = loggingblobstorage.MatchOrder(items, plan.LoggingBlobStorage)
				return nil
			},
		},
		{
			label: "S3 logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggings3.Reconcile(ctx, client, serviceID, version, plan.LoggingS3)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggings3.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingS3 = loggings3.MatchOrder(items, plan.LoggingS3)
				return nil
			},
		},
		{
			label: "New Relic OTLP logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggingnewrelicotlp.Reconcile(ctx, client, serviceID, version, plan.LoggingNewRelicOTLP)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingnewrelicotlp.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingNewRelicOTLP = loggingnewrelicotlp.MatchOrder(items, plan.LoggingNewRelicOTLP)
				return nil
			},
		},
		{
			label: "New Relic logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggingnewrelic.Reconcile(ctx, client, serviceID, version, plan.LoggingNewRelic)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingnewrelic.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingNewRelic = loggingnewrelic.MatchOrder(items, plan.LoggingNewRelic)
				return nil
			},
		},
		{
			label: "Datadog logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggingdatadog.Reconcile(ctx, client, serviceID, version, plan.LoggingDatadog)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingdatadog.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingDatadog = loggingdatadog.MatchOrder(items, plan.LoggingDatadog)
				return nil
			},
		},
		{
			label: "BigQuery logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggingbigquery.Reconcile(ctx, client, serviceID, version, plan.LoggingBigQuery)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingbigquery.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingBigQuery = loggingbigquery.MatchOrder(items, plan.LoggingBigQuery)
				return nil
			},
		},
		{
			label: "GCS logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return logginggcs.Reconcile(ctx, client, serviceID, version, plan.LoggingGCS)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := logginggcs.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingGCS = logginggcs.MatchOrder(items, plan.LoggingGCS)
				return nil
			},
		},
		{
			label: "Splunk logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggingsplunk.Reconcile(ctx, client, serviceID, version, plan.LoggingSplunk)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingsplunk.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingSplunk = loggingsplunk.MatchOrder(items, plan.LoggingSplunk)
				return nil
			},
		},
		{
			label: "HTTPS logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return logginghttps.Reconcile(ctx, client, serviceID, version, plan.LoggingHTTPS)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := logginghttps.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingHTTPS = logginghttps.MatchOrder(items, plan.LoggingHTTPS)
				return nil
			},
		},
		{
			label: "Sumologic logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggingsumologic.Reconcile(ctx, client, serviceID, version, plan.LoggingSumologic)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingsumologic.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingSumologic = loggingsumologic.MatchOrder(items, plan.LoggingSumologic)
				return nil
			},
		},
		{
			label: "Syslog logging endpoints",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return loggingsyslog.Reconcile(ctx, client, serviceID, version, plan.LoggingSyslog)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingsyslog.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.LoggingSyslog = loggingsyslog.MatchOrder(items, plan.LoggingSyslog)
				return nil
			},
		},
		{
			label: "Image Optimizer default settings",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return imageoptimizerdefaultsettings.Reconcile(ctx, client, serviceID, version, previous.ImageOptimizerDefaultSettings, plan.ImageOptimizerDefaultSettings)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				result, err := imageoptimizerdefaultsettings.ReadForVersion(ctx, client, serviceID, version, plan.ImageOptimizerDefaultSettings, false)
				if err != nil {
					return err
				}
				plan.ImageOptimizerDefaultSettings = result
				return nil
			},
		},
		{
			label: "VCL snippets",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return snippet.Reconcile(ctx, client, serviceID, version, plan.Snippet)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := snippet.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.Snippet = snippet.MatchOrderPreservePlanContent(items, plan.Snippet)
				return nil
			},
		},
		{
			label: "dynamic VCL snippets",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return dynamicsnippet.Reconcile(ctx, client, serviceID, version, plan.DynamicSnippet)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := dynamicsnippet.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.DynamicSnippet = dynamicsnippet.MatchOrderPreservePlanFields(items, plan.DynamicSnippet)
				return nil
			},
		},
		{
			label: "custom VCL files",
			reconcile: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				return vcl.Reconcile(ctx, client, serviceID, version, plan.VCL)
			},
			readBack: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := vcl.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				plan.VCL = vcl.MatchOrderPreservePlanContent(items, plan.VCL)
				return nil
			},
		},
	}
}

// readSteps covers all 28 nested resource types, including backend/director/dictionary/
// rate_limiter
func readSteps(state *Model, imported bool) []readStep {
	return []readStep{
		{
			label: "settings",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				result, err := settings.ReadForVersion(ctx, client, serviceID, version, state.Settings)
				if err != nil {
					return err
				}
				state.Settings = result
				return nil
			},
		},
		{
			label: "domains",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := domain.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.Domain = domain.MatchOrder(items, state.Domain)
				return nil
			},
		},
		{
			label: "backends",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := backend.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.Backend = backend.MatchOrder(items, state.Backend)
				return nil
			},
		},
		{
			label: "directors",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := director.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.Director = director.MatchOrder(items, state.Director)
				return nil
			},
		},
		{
			label: "ACLs",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				acls, err := cdnacl.ReadForVersionWithPlan(ctx, client, serviceID, version, state.ACL)
				if err != nil {
					return err
				}
				state.ACL = cdnacl.MatchOrder(acls, state.ACL)
				return nil
			},
		},
		{
			label: "conditions",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := condition.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.Condition = condition.MatchOrder(items, state.Condition)
				return nil
			},
		},
		{
			label: "health checks",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := healthcheck.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.HealthCheck = healthcheck.MatchOrder(items, state.HealthCheck)
				return nil
			},
		},
		{
			label: "headers",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := header.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.Header = header.MatchOrder(items, state.Header)
				return nil
			},
		},
		{
			label: "gzip configurations",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := gzip.ReadForVersionWithPlan(ctx, client, serviceID, version, state.Gzip)
				if err != nil {
					return err
				}
				state.Gzip = gzip.MatchOrder(items, state.Gzip)
				return nil
			},
		},
		{
			label: "cache settings",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := cachesetting.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.CacheSetting = cachesetting.MatchOrder(items, state.CacheSetting)
				return nil
			},
		},
		{
			label: "request settings",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := requestsetting.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.RequestSetting = requestsetting.MatchOrder(items, state.RequestSetting)
				return nil
			},
		},
		{
			label: "response objects",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := responseobject.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.ResponseObject = responseobject.MatchOrder(items, state.ResponseObject)
				return nil
			},
		},
		{
			label: "dictionaries",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := dictionary.ReadForVersionWithPlan(ctx, client, serviceID, version, state.Dictionary)
				if err != nil {
					return err
				}
				state.Dictionary = dictionary.MatchOrder(items, state.Dictionary)
				return nil
			},
		},
		{
			label: "rate limiters",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := ratelimiter.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.RateLimiter = ratelimiter.MatchOrder(items, state.RateLimiter)
				return nil
			},
		},
		{
			label: "Blob Storage logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingblobstorage.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingBlobStorage = loggingblobstorage.MatchOrder(items, state.LoggingBlobStorage)
				return nil
			},
		},
		{
			label: "S3 logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggings3.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingS3 = loggings3.MatchOrder(items, state.LoggingS3)
				return nil
			},
		},
		{
			label: "New Relic OTLP logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingnewrelicotlp.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingNewRelicOTLP = loggingnewrelicotlp.MatchOrder(items, state.LoggingNewRelicOTLP)
				return nil
			},
		},
		{
			label: "New Relic logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingnewrelic.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingNewRelic = loggingnewrelic.MatchOrder(items, state.LoggingNewRelic)
				return nil
			},
		},
		{
			label: "Datadog logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingdatadog.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingDatadog = loggingdatadog.MatchOrder(items, state.LoggingDatadog)
				return nil
			},
		},
		{
			label: "BigQuery logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingbigquery.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingBigQuery = loggingbigquery.MatchOrder(items, state.LoggingBigQuery)
				return nil
			},
		},
		{
			label: "GCS logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := logginggcs.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingGCS = logginggcs.MatchOrder(items, state.LoggingGCS)
				return nil
			},
		},
		{
			label: "Splunk logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingsplunk.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingSplunk = loggingsplunk.MatchOrder(items, state.LoggingSplunk)
				return nil
			},
		},
		{
			label: "HTTPS logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := logginghttps.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingHTTPS = logginghttps.MatchOrder(items, state.LoggingHTTPS)
				return nil
			},
		},
		{
			label: "Sumologic logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingsumologic.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingSumologic = loggingsumologic.MatchOrder(items, state.LoggingSumologic)
				return nil
			},
		},
		{
			label: "Syslog logging endpoints",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := loggingsyslog.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.LoggingSyslog = loggingsyslog.MatchOrder(items, state.LoggingSyslog)
				return nil
			},
		},
		{
			label: "Image Optimizer default settings",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				result, err := imageoptimizerdefaultsettings.ReadForVersion(ctx, client, serviceID, version, state.ImageOptimizerDefaultSettings, imported)
				if err != nil {
					return err
				}
				state.ImageOptimizerDefaultSettings = result
				return nil
			},
		},
		{
			label: "VCL snippets",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := snippet.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.Snippet = snippet.MatchOrder(items, state.Snippet)
				return nil
			},
		},
		{
			label: "dynamic VCL snippets",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := dynamicsnippet.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.DynamicSnippet = dynamicsnippet.MatchOrder(items, state.DynamicSnippet)
				return nil
			},
		},
		{
			label: "custom VCL files",
			run: func(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
				items, err := vcl.ReadForVersion(ctx, client, serviceID, version)
				if err != nil {
					return err
				}
				state.VCL = vcl.MatchOrder(items, state.VCL)
				return nil
			},
		},
	}
}

// planSteps covers all 28 nested resource types for Update's change-detection check and the
// reorder-only branch that follows it
func planSteps(plan, state *Model) []planStep {
	return []planStep{
		{
			equal:     func() bool { return settings.Equal(plan.Settings, state.Settings) },
			matchOnly: func() { plan.Settings = state.Settings },
		},
		{
			equal:     func() bool { return domain.Equal(plan.Domain, state.Domain) },
			matchOnly: func() { plan.Domain = domain.MatchOrder(state.Domain, plan.Domain) },
		},
		{
			equal:     func() bool { return backend.Equal(plan.Backend, state.Backend) },
			matchOnly: func() { plan.Backend = backend.MatchOrder(state.Backend, plan.Backend) },
		},
		{
			equal:     func() bool { return director.Equal(plan.Director, state.Director) },
			matchOnly: func() { plan.Director = director.MatchOrder(state.Director, plan.Director) },
		},
		{
			equal:     func() bool { return cdnacl.Equal(plan.ACL, state.ACL) },
			matchOnly: func() { plan.ACL = cdnacl.MatchOrder(state.ACL, plan.ACL) },
		},
		{
			equal:     func() bool { return condition.Equal(plan.Condition, state.Condition) },
			matchOnly: func() { plan.Condition = condition.MatchOrder(state.Condition, plan.Condition) },
		},
		{
			equal:     func() bool { return healthcheck.Equal(plan.HealthCheck, state.HealthCheck) },
			matchOnly: func() { plan.HealthCheck = healthcheck.MatchOrder(state.HealthCheck, plan.HealthCheck) },
		},
		{
			equal:     func() bool { return header.Equal(plan.Header, state.Header) },
			matchOnly: func() { plan.Header = header.MatchOrder(state.Header, plan.Header) },
		},
		{
			equal:     func() bool { return gzip.Equal(plan.Gzip, state.Gzip) },
			matchOnly: func() { plan.Gzip = gzip.MatchOrder(state.Gzip, plan.Gzip) },
		},
		{
			equal:     func() bool { return cachesetting.Equal(plan.CacheSetting, state.CacheSetting) },
			matchOnly: func() { plan.CacheSetting = cachesetting.MatchOrder(state.CacheSetting, plan.CacheSetting) },
		},
		{
			equal:     func() bool { return requestsetting.Equal(plan.RequestSetting, state.RequestSetting) },
			matchOnly: func() { plan.RequestSetting = requestsetting.MatchOrder(state.RequestSetting, plan.RequestSetting) },
		},
		{
			equal:     func() bool { return responseobject.Equal(plan.ResponseObject, state.ResponseObject) },
			matchOnly: func() { plan.ResponseObject = responseobject.MatchOrder(state.ResponseObject, plan.ResponseObject) },
		},
		{
			equal:     func() bool { return dictionary.Equal(plan.Dictionary, state.Dictionary) },
			matchOnly: func() { plan.Dictionary = dictionary.MatchOrder(state.Dictionary, plan.Dictionary) },
		},
		{
			equal:     func() bool { return ratelimiter.Equal(plan.RateLimiter, state.RateLimiter) },
			matchOnly: func() { plan.RateLimiter = ratelimiter.MatchOrder(state.RateLimiter, plan.RateLimiter) },
		},
		{
			equal: func() bool { return loggingblobstorage.Equal(plan.LoggingBlobStorage, state.LoggingBlobStorage) },
			matchOnly: func() {
				plan.LoggingBlobStorage = loggingblobstorage.MatchOrder(state.LoggingBlobStorage, plan.LoggingBlobStorage)
			},
		},
		{
			equal:     func() bool { return loggings3.Equal(plan.LoggingS3, state.LoggingS3) },
			matchOnly: func() { plan.LoggingS3 = loggings3.MatchOrder(state.LoggingS3, plan.LoggingS3) },
		},
		{
			equal: func() bool { return loggingnewrelicotlp.Equal(plan.LoggingNewRelicOTLP, state.LoggingNewRelicOTLP) },
			matchOnly: func() {
				plan.LoggingNewRelicOTLP = loggingnewrelicotlp.MatchOrder(state.LoggingNewRelicOTLP, plan.LoggingNewRelicOTLP)
			},
		},
		{
			equal:     func() bool { return loggingnewrelic.Equal(plan.LoggingNewRelic, state.LoggingNewRelic) },
			matchOnly: func() { plan.LoggingNewRelic = loggingnewrelic.MatchOrder(state.LoggingNewRelic, plan.LoggingNewRelic) },
		},
		{
			equal:     func() bool { return loggingdatadog.Equal(plan.LoggingDatadog, state.LoggingDatadog) },
			matchOnly: func() { plan.LoggingDatadog = loggingdatadog.MatchOrder(state.LoggingDatadog, plan.LoggingDatadog) },
		},
		{
			equal:     func() bool { return loggingbigquery.Equal(plan.LoggingBigQuery, state.LoggingBigQuery) },
			matchOnly: func() { plan.LoggingBigQuery = loggingbigquery.MatchOrder(state.LoggingBigQuery, plan.LoggingBigQuery) },
		},
		{
			equal:     func() bool { return logginggcs.Equal(plan.LoggingGCS, state.LoggingGCS) },
			matchOnly: func() { plan.LoggingGCS = logginggcs.MatchOrder(state.LoggingGCS, plan.LoggingGCS) },
		},
		{
			equal:     func() bool { return loggingsplunk.Equal(plan.LoggingSplunk, state.LoggingSplunk) },
			matchOnly: func() { plan.LoggingSplunk = loggingsplunk.MatchOrder(state.LoggingSplunk, plan.LoggingSplunk) },
		},
		{
			equal:     func() bool { return logginghttps.Equal(plan.LoggingHTTPS, state.LoggingHTTPS) },
			matchOnly: func() { plan.LoggingHTTPS = logginghttps.MatchOrder(state.LoggingHTTPS, plan.LoggingHTTPS) },
		},
		{
			equal: func() bool { return loggingsumologic.Equal(plan.LoggingSumologic, state.LoggingSumologic) },
			matchOnly: func() {
				plan.LoggingSumologic = loggingsumologic.MatchOrder(state.LoggingSumologic, plan.LoggingSumologic)
			},
		},
		{
			equal:     func() bool { return loggingsyslog.Equal(plan.LoggingSyslog, state.LoggingSyslog) },
			matchOnly: func() { plan.LoggingSyslog = loggingsyslog.MatchOrder(state.LoggingSyslog, plan.LoggingSyslog) },
		},
		{
			equal: func() bool {
				return imageoptimizerdefaultsettings.Equal(plan.ImageOptimizerDefaultSettings, state.ImageOptimizerDefaultSettings)
			},
			matchOnly: func() { plan.ImageOptimizerDefaultSettings = state.ImageOptimizerDefaultSettings },
		},
		{
			equal:     func() bool { return snippet.Equal(plan.Snippet, state.Snippet) },
			matchOnly: func() { plan.Snippet = snippet.MatchOrderPreservePlanContent(state.Snippet, plan.Snippet) },
		},
		{
			equal: func() bool { return dynamicsnippet.Equal(plan.DynamicSnippet, state.DynamicSnippet) },
			matchOnly: func() {
				plan.DynamicSnippet = dynamicsnippet.MatchOrderPreservePlanFields(state.DynamicSnippet, plan.DynamicSnippet)
			},
		},
		{
			equal:     func() bool { return vcl.Equal(plan.VCL, state.VCL) },
			matchOnly: func() { plan.VCL = vcl.MatchOrderPreservePlanContent(state.VCL, plan.VCL) },
		},
	}
}
