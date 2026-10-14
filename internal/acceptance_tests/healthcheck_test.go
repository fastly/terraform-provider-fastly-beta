package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyServiceCDNAuto_withHealthCheck(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithHealthCheck(serviceName, domainName, healthCheckName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.name", healthCheckName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.host", "example.com"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.path", "/healthz"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.check_interval", "5000"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.expected_response", "200"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.http_version", "1.1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.initial", "3"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.method", "HEAD"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.threshold", "3"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.timeout", "5000"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.window", "5"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_withMultipleHealthChecks(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName1 := fmt.Sprintf("hc_1_%s", acctest.RandString(10))
	healthCheckName2 := fmt.Sprintf("hc_2_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithMultipleHealthChecks(serviceName, domainName, healthCheckName1, healthCheckName2),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.#", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.name", healthCheckName1),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.name", healthCheckName2),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.host", "other.example.com"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.path", "/status"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.check_interval", "10000"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.expected_response", "204"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.headers.#", "1"),
					resource.TestCheckTypeSetElemAttr("fastly_service_cdn_auto.test", "healthcheck.1.headers.*", "X-Api-Key: abc123"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.http_version", "1.0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.initial", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.method", "GET"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.threshold", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.timeout", "3000"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.1.window", "10"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_withHealthCheckUpdate(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithHealthCheck(serviceName, domainName, healthCheckName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.host", "example.com"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.threshold", "3"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithHealthCheckUpdated(serviceName, domainName, healthCheckName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.name", healthCheckName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.host", "updated.example.com"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.path", "/status"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.check_interval", "10000"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.threshold", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.headers.#", "1"),
					resource.TestCheckTypeSetElemAttr("fastly_service_cdn_auto.test", "healthcheck.0.headers.*", "X-Api-Key: abc123"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withHealthCheckAndBackend confirms that a backend referencing a
// health check by name applies cleanly, verifying health checks are reconciled before backend
// within the same service version (see servicecdnauto's Create/Update, which reconcile
// healthcheck immediately after condition and before backend for this reason).
func TestAccFastlyServiceCDNAuto_withHealthCheckAndBackend(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc_%s", acctest.RandString(10))
	backendName := fmt.Sprintf("backend_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithHealthCheckAndBackend(serviceName, domainName, healthCheckName, backendName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "healthcheck.0.name", healthCheckName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "backend.0.name", backendName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "backend.0.healthcheck", healthCheckName),
				),
			},
		},
	})
}

func TestAccFastlyServiceComputeAuto_withHealthCheck(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigComputeAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "healthcheck.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "1"),
				),
			},
			{
				Config: ConfigComputeAutoWithHealthCheck(serviceName, domainName, healthCheckName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "healthcheck.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "healthcheck.0.name", healthCheckName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "healthcheck.0.host", "example.com"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "healthcheck.0.path", "/healthz"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceHealthCheck_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHealthCheckBasic(serviceName, domainName, healthCheckName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "name", healthCheckName),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "host", "example.com"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "path", "/healthz"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "check_interval", "5000"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "expected_response", "200"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "http_version", "1.1"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "initial", "3"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "method", "HEAD"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "threshold", "3"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "timeout", "5000"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "window", "5"),
					resource.TestCheckNoResourceAttr("fastly_service_healthcheck.test", "headers"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_healthcheck.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_healthcheck.test", "id"),
				),
			},
			{
				Config:   ConfigHealthCheckBasic(serviceName, domainName, healthCheckName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceHealthCheck_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHealthCheckBasic(serviceName, domainName, healthCheckName),
				Check:  resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "host", "example.com"),
			},
			{
				Config: ConfigHealthCheckUpdated(serviceName, domainName, healthCheckName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_healthcheck.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "name", healthCheckName),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "host", "updated.example.com"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "path", "/status"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "check_interval", "10000"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "expected_response", "204"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "headers.#", "2"),
					resource.TestCheckTypeSetElemAttr("fastly_service_healthcheck.test", "headers.*", "X-Api-Key: abc123"),
					resource.TestCheckTypeSetElemAttr("fastly_service_healthcheck.test", "headers.*", "X-Env: test"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "http_version", "1.0"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "initial", "1"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "method", "GET"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "threshold", "2"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "timeout", "3000"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "window", "10"),
				),
			},
			{
				Config:   ConfigHealthCheckUpdated(serviceName, domainName, healthCheckName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceHealthCheck_headersRemovedForcesReplace verifies that removing every header
// replaces the health check, since an in-place update cannot clear headers remotely.
func TestAccFastlyServiceHealthCheck_headersRemovedForcesReplace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHealthCheckUpdated(serviceName, domainName, healthCheckName),
				Check:  resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "headers.#", "2"),
			},
			{
				Config: ConfigHealthCheckBasic(serviceName, domainName, healthCheckName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_healthcheck.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("fastly_service_healthcheck.test", "headers"),
					CheckHealthCheckHeadersInFastly("fastly_service_healthcheck.test", healthCheckName, 0),
				),
			},
		},
	})
}

// TestAccFastlyServiceHealthCheck_withBackend confirms an explicit backend can reference an
// explicit health check by name on the same service version.
func TestAccFastlyServiceHealthCheck_withBackend(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHealthCheckWithBackend(serviceName, domainName, healthCheckName, backendName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "name", healthCheckName),
					resource.TestCheckResourceAttr("fastly_service_backend.origin", "healthcheck", healthCheckName),
				),
			},
		},
	})
}

// TestAccFastlyServiceHealthCheck_computeService verifies that fastly_service_healthcheck, unlike
// the VCL-only explicit resources, is accepted on a Compute service.
func TestAccFastlyServiceHealthCheck_computeService(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHealthCheckOnComputeService(serviceName, healthCheckName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "name", healthCheckName),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "version", "1"),
				),
			},
		},
	})
}

// TestAccFastlyServiceHealthCheck_lockedVersion verifies that the provider refuses to write a
// health check to an activated (locked) service version. Version 1, which holds the service and
// domain the test cleans up afterward, is never activated - only a cloned version 2 is.
func TestAccFastlyServiceHealthCheck_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceCDNWithDomain(serviceName, domainName, 1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_cdn.test"]
						if !ok {
							return fmt.Errorf("service resource not found")
						}

						client, err := NewFastlyClient()
						if err != nil {
							return fmt.Errorf("error creating Fastly client: %w", err)
						}

						ctx := context.Background()
						cloned, err := client.CloneVersion(ctx, &fastly.CloneVersionInput{
							ServiceID:      rs.Primary.ID,
							ServiceVersion: 1,
						})
						if err != nil {
							return fmt.Errorf("error cloning version: %w", err)
						}

						_, err = client.ActivateVersion(ctx, &fastly.ActivateVersionInput{
							ServiceID:      rs.Primary.ID,
							ServiceVersion: fastly.ToValue(cloned.Number),
						})
						if err != nil {
							return fmt.Errorf("error activating cloned version: %w", err)
						}

						return nil
					},
				),
			},
			{
				Config:      ConfigHealthCheckOnLockedVersion(serviceName, domainName, healthCheckName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceHealthCheck_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName := fmt.Sprintf("hc-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHealthCheckForImport(serviceName, domainName, healthCheckName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "name", healthCheckName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_healthcheck.test"]
						if !ok {
							return fmt.Errorf("health check resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_healthcheck.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, healthCheckName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
				// Import has no prior config to borrow spacing from, so headers come back in
				// the API's "Name:value" form rather than the configured "Name: value".
				ImportStateVerifyIgnore: []string{"headers"},
			},
		},
	})
}

// TestAccFastlyServiceHealthCheck_nameForcesReplace verifies changing name forces destroy/create
// rather than an in-place update.
func TestAccFastlyServiceHealthCheck_nameForcesReplace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	healthCheckName1 := fmt.Sprintf("hc-%s", acctest.RandString(10))
	healthCheckName2 := fmt.Sprintf("hc-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHealthCheckBasic(serviceName, domainName, healthCheckName1),
				Check:  resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "name", healthCheckName1),
			},
			{
				Config: ConfigHealthCheckBasic(serviceName, domainName, healthCheckName2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_healthcheck.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("fastly_service_healthcheck.test", "name", healthCheckName2),
			},
		},
	})
}

// CheckHealthCheckHeadersInFastly fetches the health check straight from the API, so a stale
// remote value can't hide behind matching Terraform state.
func CheckHealthCheckHeadersInFastly(resourceName, healthCheckName string, wantCount int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		var version int
		if _, err := fmt.Sscanf(rs.Primary.Attributes["version"], "%d", &version); err != nil {
			return fmt.Errorf("error parsing version: %w", err)
		}

		hc, err := client.GetHealthCheck(context.Background(), &fastly.GetHealthCheckInput{
			ServiceID:      rs.Primary.Attributes["service_id"],
			ServiceVersion: version,
			Name:           healthCheckName,
		})
		if err != nil {
			return fmt.Errorf("error fetching health check %q: %w", healthCheckName, err)
		}

		if len(hc.Headers) != wantCount {
			return fmt.Errorf("health check %q has %d headers in Fastly, want %d: %v", healthCheckName, len(hc.Headers), wantCount, hc.Headers)
		}
		return nil
	}
}
