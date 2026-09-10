package acceptancetests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFastlyServiceCDNAuto_withLoggingGrafanaCloudLogs exercises Grafana
// Cloud Logs logging as a nested block inside fastly_service_cdn_auto: adding
// the endpoint clones and activates a new version, and the reconciled state
// reflects the created endpoint.
func TestAccFastlyServiceCDNAuto_withLoggingGrafanaCloudLogs(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("grafanacloudlogs-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingGrafanaCloudLogsExistsInFastly("fastly_service_cdn_auto.test", loggerName, 2),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.url", "https://test123.grafana.net"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.user", "123456"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.authentication.token", "test-grafanacloudlogs-token"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
			{
				// The default format must round-trip, and removing nothing must not
				// trigger another clone/activate.
				Config:   ConfigCDNAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withLoggingGrafanaCloudLogsUpdate changes
// optional attributes on a nested Grafana Cloud Logs logging endpoint,
// exercising the reconcile update path (in-place update, not delete+recreate)
// inside a newly cloned and activated version.
// TestAccFastlyServiceCDNAuto_loggingGrafanaCloudLogsEmptyFormat verifies
// that the nested logging_grafanacloudlogs block rejects an explicit
// format = "" at validate time rather than failing apply with the opaque
// "Provider produced inconsistent result after apply" — mirrors the fiddle
// CI failure originally seen on logging_https (fastly/fiddle run
// 34388361955) but through the nested/_auto path.
func TestAccFastlyServiceCDNAuto_loggingGrafanaCloudLogsEmptyFormat(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("grafanacloudlogs-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigCDNAutoWithLoggingGrafanaCloudLogsEmptyFormat(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile("`format` cannot be explicitly set to an empty string"),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_withLoggingGrafanaCloudLogsUpdate(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("grafanacloudlogs-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingGrafanaCloudLogsExistsInFastly("fastly_service_cdn_auto.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.url", "https://test123.grafana.net"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithLoggingGrafanaCloudLogsUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingGrafanaCloudLogsExistsInFastly("fastly_service_cdn_auto.test", loggerName, 2),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.authentication.token", "updated-grafanacloudlogs-token"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.url", "https://test456.grafana.net"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
			{
				Config:   ConfigCDNAutoWithLoggingGrafanaCloudLogsUpdated(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withLoggingGrafanaCloudLogsRemoved verifies that
// deleting the nested block removes the endpoint from the Fastly API in a
// newly activated version, rather than leaving it orphaned.
func TestAccFastlyServiceCDNAuto_withLoggingGrafanaCloudLogsRemoved(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("grafanacloudlogs-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingGrafanaCloudLogsExistsInFastly("fastly_service_cdn_auto.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.#", "1"),
				),
			},
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_loggingGrafanaCloudLogsPlacementUnsetVsNone
// verifies that the nested logging_grafanacloudlogs block's placement
// round-trips between unset and explicit "none". It checks
// active_version/managed_version alongside placement because those Computed
// fields only advance when a nested block actually changes — the reset needs
// to be visible there too.
func TestAccFastlyServiceCDNAuto_loggingGrafanaCloudLogsPlacementUnsetVsNone(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("grafanacloudlogs-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				// Start unset. Created directly with the logging block already
				// present, so this is a single Create at version 1 — not a subsequent
				// clone — unlike TestAccFastlyServiceCDNAuto_withLoggingGrafanaCloudLogs,
				// which adds the block in a second step against an already-created
				// service.
				Config: ConfigCDNAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
					resource.TestCheckNoResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.placement"),
				),
			},
			{
				// Update to explicit "none".
				Config: ConfigCDNAutoWithLoggingGrafanaCloudLogsPlacementNone(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.placement", "none"),
				),
			},
			{
				// Update back to unset.
				Config: ConfigCDNAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "3"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "3"),
					resource.TestCheckNoResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.placement"),
				),
			},
			{
				// The API's null response must leave no residual diff against the
				// same, still-unset config.
				Config:   ConfigCDNAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withMultipleLoggingGrafanaCloudLogs verifies that
// multiple nested Grafana Cloud Logs logging endpoints reconcile correctly and
// preserve configured order across reads.
func TestAccFastlyServiceCDNAuto_withMultipleLoggingGrafanaCloudLogs(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName1 := fmt.Sprintf("grafanacloudlogs-logger-1-%s", acctest.RandString(10))
	loggerName2 := fmt.Sprintf("grafanacloudlogs-logger-2-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithMultipleLoggingGrafanaCloudLogs(serviceName, domainName, loggerName1, loggerName2),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingGrafanaCloudLogsExistsInFastly("fastly_service_cdn_auto.test", loggerName1, 1),
					CheckLoggingGrafanaCloudLogsExistsInFastly("fastly_service_cdn_auto.test", loggerName2, 1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.#", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.name", loggerName1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.1.name", loggerName2),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config:   ConfigCDNAutoWithMultipleLoggingGrafanaCloudLogs(serviceName, domainName, loggerName1, loggerName2),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withBackendAndLoggingGrafanaCloudLogs verifies
// Grafana Cloud Logs logging reconciles alongside another nested block type in
// the same auto service.
func TestAccFastlyServiceCDNAuto_withBackendAndLoggingGrafanaCloudLogs(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("grafanacloudlogs-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithBackendAndLoggingGrafanaCloudLogs(serviceName, domainName, backendName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingGrafanaCloudLogsExistsInFastly("fastly_service_cdn_auto.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "backend.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "backend.0.name", backendName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_grafanacloudlogs.0.name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
		},
	})
}

// TestAccFastlyServiceComputeAuto_withLoggingGrafanaCloudLogs exercises Grafana
// Cloud Logs logging as a nested block inside fastly_service_compute_auto,
// covering the reconcile path for the Compute family.
func TestAccFastlyServiceComputeAuto_withLoggingGrafanaCloudLogs(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("grafanacloudlogs-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigComputeAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					CheckLoggingGrafanaCloudLogsExistsInFastly("fastly_service_compute_auto.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_grafanacloudlogs.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_grafanacloudlogs.0.name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_grafanacloudlogs.0.authentication.token", "test-grafanacloudlogs-token"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_grafanacloudlogs.0.url", "https://test123.grafana.net"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "1"),
				),
			},
			{
				// The Compute nested block never sends the VCL-only fields, so the
				// remote endpoint's server-side format must not surface as a diff.
				Config:   ConfigComputeAutoWithLoggingGrafanaCloudLogs(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceComputeAuto_loggingGrafanaCloudLogsRejectsVCLOnlyFields
// verifies that format (and, by extension,
// format_version/placement/response_condition) is not a valid attribute on
// service_compute_auto's nested logging_grafanacloudlogs block. Those
// attributes only affect generated VCL, so ComputeNestedBlockSchema omits them
// entirely — Terraform should reject this at plan time with its own
// "Unsupported argument" schema error, without ever reaching the Fastly API.
func TestAccFastlyServiceComputeAuto_loggingGrafanaCloudLogsRejectsVCLOnlyFields(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("grafanacloudlogs-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigComputeAutoWithLoggingGrafanaCloudLogsFormat(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile(`Unsupported argument`),
			},
		},
	})
}
