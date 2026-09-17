package acceptancetests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFastlyServiceCDNAuto_withLoggingSFTP exercises SFTP logging as a
// nested block inside fastly_service_cdn_auto: adding the endpoint clones and
// activates a new version, and the reconciled state reflects the created
// endpoint.
func TestAccFastlyServiceCDNAuto_withLoggingSFTP(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingSFTPExistsInFastly("fastly_service_cdn_auto.test", loggerName, 2),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.authentication.user", "test-user"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.authentication.password", "test-password"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
			{
				// The default format must round-trip, and removing nothing must not
				// trigger another clone/activate.
				Config:   ConfigCDNAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withLoggingSFTPUpdate changes optional
// attributes on a nested SFTP logging endpoint, exercising the reconcile
// update path (in-place update, not delete+recreate) inside a newly cloned
// and activated version.
func TestAccFastlyServiceCDNAuto_withLoggingSFTPUpdate(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingSFTPExistsInFastly("fastly_service_cdn_auto.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithLoggingSFTPUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingSFTPExistsInFastly("fastly_service_cdn_auto.test", loggerName, 2),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.authentication.password", "updated-password"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.port", "2222"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
			{
				Config:   ConfigCDNAutoWithLoggingSFTPUpdated(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withLoggingSFTPRemoved verifies that deleting the
// nested block removes the endpoint from the Fastly API in a newly activated
// version, rather than leaving it orphaned.
func TestAccFastlyServiceCDNAuto_withLoggingSFTPRemoved(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingSFTPExistsInFastly("fastly_service_cdn_auto.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.#", "1"),
				),
			},
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_loggingSFTPPlacementUnsetVsNone verifies that the
// nested logging_sftp block's placement round-trips between unset and explicit
// "none". It checks active_version/managed_version alongside placement
// because those Computed fields only advance when a nested block actually
// changes — the reset needs to be visible there too.
func TestAccFastlyServiceCDNAuto_loggingSFTPPlacementUnsetVsNone(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				// Start unset. Created directly with the logging block already
				// present, so this is a single Create at version 1 — not a subsequent
				// clone — unlike TestAccFastlyServiceCDNAuto_withLoggingSFTP, which adds
				// the block in a second step against an already-created service.
				Config: ConfigCDNAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
					resource.TestCheckNoResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.placement"),
				),
			},
			{
				// Update to explicit "none".
				Config: ConfigCDNAutoWithLoggingSFTPPlacementNone(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.placement", "none"),
				),
			},
			{
				// Update back to unset.
				Config: ConfigCDNAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "3"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "3"),
					resource.TestCheckNoResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.placement"),
				),
			},
			{
				// The API's null response must leave no residual diff against the
				// same, still-unset config.
				Config:   ConfigCDNAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withMultipleLoggingSFTP verifies that multiple
// nested SFTP logging endpoints reconcile correctly and preserve configured
// order across reads.
func TestAccFastlyServiceCDNAuto_withMultipleLoggingSFTP(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName1 := fmt.Sprintf("sftp-logger-1-%s", acctest.RandString(10))
	loggerName2 := fmt.Sprintf("sftp-logger-2-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithMultipleLoggingSFTP(serviceName, domainName, loggerName1, loggerName2),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingSFTPExistsInFastly("fastly_service_cdn_auto.test", loggerName1, 1),
					CheckLoggingSFTPExistsInFastly("fastly_service_cdn_auto.test", loggerName2, 1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.#", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.name", loggerName1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.1.name", loggerName2),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config:   ConfigCDNAutoWithMultipleLoggingSFTP(serviceName, domainName, loggerName1, loggerName2),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withBackendAndLoggingSFTP verifies SFTP logging
// reconciles alongside another nested block type in the same auto service.
func TestAccFastlyServiceCDNAuto_withBackendAndLoggingSFTP(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithBackendAndLoggingSFTP(serviceName, domainName, backendName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckLoggingSFTPExistsInFastly("fastly_service_cdn_auto.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "backend.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "backend.0.name", backendName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_sftp.0.name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
		},
	})
}

// TestAccFastlyServiceComputeAuto_withLoggingSFTP exercises SFTP logging as a
// nested block inside fastly_service_compute_auto, covering the reconcile
// path for the Compute family.
func TestAccFastlyServiceComputeAuto_withLoggingSFTP(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigComputeAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					CheckLoggingSFTPExistsInFastly("fastly_service_compute_auto.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_sftp.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_sftp.0.name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_sftp.0.authentication.user", "test-user"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_sftp.0.authentication.password", "test-password"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "1"),
				),
			},
			{
				// The Compute nested block never sends the VCL-only fields, so the
				// remote endpoint's server-side format must not surface as a diff.
				Config:   ConfigComputeAutoWithLoggingSFTP(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceComputeAuto_loggingSFTPRejectsVCLOnlyFields verifies that
// format (and, by extension, format_version/placement/response_condition) is
// not a valid attribute on service_compute_auto's nested logging_sftp block.
// Those attributes only affect generated VCL, so ComputeNestedBlockSchema
// omits them entirely — Terraform should reject this at plan time with its
// own "Unsupported argument" schema error, without ever reaching the Fastly
// API.
func TestAccFastlyServiceComputeAuto_loggingSFTPRejectsVCLOnlyFields(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigComputeAutoWithLoggingSFTPFormat(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile(`Unsupported argument`),
			},
		},
	})
}
