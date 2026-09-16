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
	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
)

func TestAccFastlyServiceLoggingOpenStack_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingOpenStackBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "bucket_name", "test-openstack-bucket"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "url", "https://auth.storage.example.com/v1.0"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "authentication.user", "test-user"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "authentication.access_key", "test-access-key"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "processing_region", "none"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_openstack.test", "format"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_openstack.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_openstack.test", "id"),
				),
			},
			{
				// The default format is a Computed default sent verbatim to the API,
				// so it must round-trip byte-for-byte and leave no residual diff.
				Config:   ConfigLoggingOpenStackBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingOpenStack_emptyFormat verifies an explicit
// format = "" is rejected at validate time rather than failing apply.
func TestAccFastlyServiceLoggingOpenStack_emptyFormat(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingOpenStackEmptyFormat(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile("`format` cannot be explicitly set to an empty string"),
			},
		},
	})
}

// TestAccFastlyServiceLoggingOpenStack_emptyTimestampFormat verifies an
// explicit timestamp_format = "" is rejected at validate time.
func TestAccFastlyServiceLoggingOpenStack_emptyTimestampFormat(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingOpenStackEmptyTimestampFormat(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile("`timestamp_format` cannot be explicitly set to an empty string"),
			},
		},
	})
}

func TestAccFastlyServiceLoggingOpenStack_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingOpenStackBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "processing_region", "none"),
				),
			},
			{
				Config: ConfigLoggingOpenStackUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "authentication.access_key", "updated-access-key"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "format", "%h %l %u %t \"%r\" %>s %b"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "placement", "none"),
				),
			},
		},
	})
}

func TestAccFastlyServiceLoggingOpenStack_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingOpenStackForImport(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "name", loggerName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_openstack.test"]
						if !ok {
							return fmt.Errorf("openstack resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_logging_openstack.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, loggerName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingOpenStack_clearToDefaults sets the optional
// attributes, then removes them, and verifies each reverts to its schema default
// (or, for placement, to unset — it has no default) rather than leaving a
// perpetual diff.
func TestAccFastlyServiceLoggingOpenStack_clearToDefaults(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingOpenStackUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "placement", "none"),
				),
			},
			{
				Config: ConfigLoggingOpenStackBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "processing_region", "none"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "response_condition", ""),
					// placement is left unconfigured here, which is distinct from
					// explicitly set to "none" — see
					// TestAccFastlyServiceLoggingOpenStack_placementUnsetVsNone.
					resource.TestCheckNoResourceAttr("fastly_service_logging_openstack.test", "placement"),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingOpenStack_placementUnsetVsNone verifies that
// leaving placement unconfigured and explicitly setting it to "none" are
// distinct, round-trippable states — not just "on create" but across updates in
// both directions — rather than being collapsed together, since the API treats
// an unset placement (auto-place in vcl_log/vcl_deliver) differently from an
// explicit "none" (suppress the log statement entirely).
func TestAccFastlyServiceLoggingOpenStack_placementUnsetVsNone(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				// Start unset.
				Config: ConfigLoggingOpenStackBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_openstack.test", "placement"),
				),
			},
			{
				// Update to explicit "none".
				Config: ConfigLoggingOpenStackUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "placement", "none"),
				),
			},
			{
				// Update back to unset.
				Config: ConfigLoggingOpenStackBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_openstack.test", "placement"),
				),
			},
			{
				// The API's null response must leave no residual diff against the
				// same, still-unset config.
				Config:   ConfigLoggingOpenStackBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingOpenStack_versionUpdateInPlace verifies that
// bumping the explicit resource's version argument is an in-place update
// against the new version rather than a destroy-and-recreate. The explicit
// clone workflow copies the endpoint into the new version, so version is
// intentionally not replacement-forcing (unlike service_id and name).
func TestAccFastlyServiceLoggingOpenStack_versionUpdateInPlace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	var serviceID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingOpenStackAtVersion(serviceName, domainName, loggerName, 1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "version", "1"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_openstack.test"]
						if !ok {
							return fmt.Errorf("openstack resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					client, err := NewFastlyClient()
					if err != nil {
						t.Fatalf("error creating Fastly client: %s", err)
					}
					if _, err := client.CloneVersion(context.Background(), &fastly.CloneVersionInput{
						ServiceID:      serviceID,
						ServiceVersion: 1,
					}); err != nil {
						t.Fatalf("error cloning version 1: %s", err)
					}
				},
				Config: ConfigLoggingOpenStackAtVersion(serviceName, domainName, loggerName, 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_logging_openstack.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "version", "2"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_openstack.test"]
						if !ok {
							return fmt.Errorf("openstack resource not found")
						}

						gotID := rs.Primary.Attributes["id"]
						wantID := fmt.Sprintf("%s-2-%s", serviceID, loggerName)
						if gotID != wantID {
							return fmt.Errorf("expected id %q to reflect version 2, got %q", wantID, gotID)
						}

						client, err := NewFastlyClient()
						if err != nil {
							return fmt.Errorf("error creating Fastly client: %w", err)
						}
						if _, err := client.GetOpenstack(context.Background(), &fastly.GetOpenstackInput{
							ServiceID:      serviceID,
							ServiceVersion: 2,
							Name:           loggerName,
						}); err != nil {
							return fmt.Errorf("error fetching OpenStack logging endpoint at version 2: %w", err)
						}

						return nil
					},
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingOpenStack_computeRejectsVCLOnlyFields verifies
// that fastly_service_logging_openstack rejects format (a VCL-only attribute)
// when attached to a Compute service. The standalone resource's schema is
// shared across both service types, so this is enforced by
// ValidateNoVCLOnlyAttributesForCompute at apply time rather than by the schema
// itself.
func TestAccFastlyServiceLoggingOpenStack_computeRejectsVCLOnlyFields(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingOpenStackComputeFormat(serviceName, loggerName),
				ExpectError: regexp.MustCompile("VCL-only attributes not supported on Compute services"),
			},
		},
	})
}

// TestAccFastlyServiceLoggingOpenStack_formatDefault catches upstream changes
// to the format Fastly assigns when none is sent, which would leave
// constants.LoggingOpenStackDefaultFormat stale. Compute is used because it's
// the only path that omits format from the request - on VCL the schema default
// is always sent, so the API just echoes our own constant back.
func TestAccFastlyServiceLoggingOpenStack_formatDefault(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingOpenStackCompute(serviceName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					CheckLoggingOpenStackFormatDefault("fastly_service_compute.test", loggerName, 1),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingOpenStack_computeConsistentAfterApply covers the
// whole plan -> API response -> flatten -> state path on a Compute service,
// which the unit tests cannot reach. The VCL-only attributes are never sent for
// Compute, but their schema defaults still land in the plan, so the API's own
// values (a different default format, and placement forced to "none" on wasm)
// used to be read back into state and fail Terraform's post-apply consistency
// check with "Provider produced inconsistent result after apply". The trailing
// PlanOnly step then proves the same values survive a refresh with no residual
// diff.
func TestAccFastlyServiceLoggingOpenStack_computeConsistentAfterApply(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("openstack-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingOpenStackCompute(serviceName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					CheckLoggingOpenStackExistsInFastly("fastly_service_compute.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "name", loggerName),
					// The VCL-only attributes must hold their schema defaults, not
					// whatever the API returned for the wasm service.
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "format", constants.LoggingOpenStackDefaultFormat),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_openstack.test", "response_condition", ""),
					resource.TestCheckNoResourceAttr("fastly_service_logging_openstack.test", "placement"),
				),
			},
			{
				Config:   ConfigLoggingOpenStackCompute(serviceName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// CheckLoggingOpenStackFormatDefault fails if the format Fastly reports for a
// logging endpoint differs from constants.LoggingOpenStackDefaultFormat. Reads
// the API directly, since FlattenToComputeNestedModel writes the constant into
// state without consulting the response. Only meaningful on an endpoint created
// without a format in the request.
func CheckLoggingOpenStackFormatDefault(serviceName, loggerName string, version int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[serviceName]
		if !ok {
			return fmt.Errorf("service not found: %s", serviceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		logger, err := client.GetOpenstack(context.Background(), &fastly.GetOpenstackInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           loggerName,
		})
		if err != nil {
			return fmt.Errorf("error fetching OpenStack logging endpoint from Fastly: %w", err)
		}
		if logger == nil {
			return fmt.Errorf("OpenStack logging endpoint %s not found in Fastly", loggerName)
		}

		if logger.Format == nil {
			return fmt.Errorf("Fastly returned a null format for OpenStack logging endpoint %s, expected its default format", loggerName)
		}

		if got := *logger.Format; got != constants.LoggingOpenStackDefaultFormat {
			return fmt.Errorf(
				"constants.LoggingOpenStackDefaultFormat no longer matches the format Fastly assigns by default\ngot from API: %q\nconstant:     %q",
				got, constants.LoggingOpenStackDefaultFormat,
			)
		}

		return nil
	}
}

// CheckLoggingOpenStackExistsInFastly verifies an OpenStack logging endpoint
// exists in the Fastly API.
func CheckLoggingOpenStackExistsInFastly(serviceName, loggerName string, version int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[serviceName]
		if !ok {
			return fmt.Errorf("service not found: %s", serviceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		logger, err := client.GetOpenstack(context.Background(), &fastly.GetOpenstackInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           loggerName,
		})
		if err != nil {
			return fmt.Errorf("error fetching OpenStack logging endpoint from Fastly: %w", err)
		}

		if logger == nil {
			return fmt.Errorf("OpenStack logging endpoint %s not found in Fastly", loggerName)
		}

		return nil
	}
}
