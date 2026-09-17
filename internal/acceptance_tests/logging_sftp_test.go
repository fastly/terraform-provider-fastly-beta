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

func TestAccFastlyServiceLoggingSFTP_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingSFTPBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "address", "sftp.example.com"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "path", "/"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "authentication.user", "test-user"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "authentication.password", "test-password"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "port", "22"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "processing_region", "none"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_sftp.test", "format"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_sftp.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_sftp.test", "id"),
				),
			},
			{
				// The default format is a Computed default sent verbatim to the API,
				// so it must round-trip byte-for-byte and leave no residual diff.
				Config:   ConfigLoggingSFTPBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_secretKey verifies that authenticating with
// secret_key instead of password is accepted — the legacy SDKv2 provider
// supports either, so this checks the non-default path.
func TestAccFastlyServiceLoggingSFTP_secretKey(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingSFTPSecretKey(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "authentication.user", "test-user"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_sftp.test", "authentication.secret_key"),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_missingCredential verifies that omitting
// both password and secret_key is rejected at plan time via
// requireOneCredential, rather than the legacy SDKv2 provider's apply-time-only
// Create check ("either password or secret_key must be set").
func TestAccFastlyServiceLoggingSFTP_missingCredential(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingSFTPMissingCredential(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile("Missing SFTP credential"),
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_emptyFormat verifies an explicit format = ""
// is rejected at validate time rather than failing apply.
func TestAccFastlyServiceLoggingSFTP_emptyFormat(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingSFTPEmptyFormat(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile("`format` cannot be explicitly set to an empty string"),
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_emptyTimestampFormat verifies an explicit
// timestamp_format = "" is rejected at validate time.
func TestAccFastlyServiceLoggingSFTP_emptyTimestampFormat(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingSFTPEmptyTimestampFormat(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile("`timestamp_format` cannot be explicitly set to an empty string"),
			},
		},
	})
}

func TestAccFastlyServiceLoggingSFTP_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingSFTPBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "port", "22"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "processing_region", "none"),
				),
			},
			{
				Config: ConfigLoggingSFTPUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "authentication.password", "updated-password"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "path", "/logs/"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "port", "2222"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "format", "%h %l %u %t \"%r\" %>s %b"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "placement", "none"),
				),
			},
		},
	})
}

func TestAccFastlyServiceLoggingSFTP_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingSFTPForImport(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "name", loggerName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_sftp.test"]
						if !ok {
							return fmt.Errorf("sftp resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_logging_sftp.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, loggerName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_clearToDefaults sets the optional
// attributes, then removes them, and verifies each reverts to its schema
// default (or, for placement, to unset — it has no default) rather than
// leaving a perpetual diff.
func TestAccFastlyServiceLoggingSFTP_clearToDefaults(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingSFTPUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "port", "2222"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "placement", "none"),
				),
			},
			{
				Config: ConfigLoggingSFTPBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "port", "22"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "processing_region", "none"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "response_condition", ""),
					// placement is left unconfigured here, which is distinct from
					// explicitly set to "none" — see
					// TestAccFastlyServiceLoggingSFTP_placementUnsetVsNone.
					resource.TestCheckNoResourceAttr("fastly_service_logging_sftp.test", "placement"),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_placementUnsetVsNone verifies that leaving
// placement unconfigured and explicitly setting it to "none" are distinct,
// round-trippable states — not just "on create" but across updates in both
// directions — rather than being collapsed together, since the API treats an
// unset placement (auto-place in vcl_log/vcl_deliver) differently from an
// explicit "none" (suppress the log statement entirely).
func TestAccFastlyServiceLoggingSFTP_placementUnsetVsNone(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				// Start unset.
				Config: ConfigLoggingSFTPBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_sftp.test", "placement"),
				),
			},
			{
				// Update to explicit "none".
				Config: ConfigLoggingSFTPUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "placement", "none"),
				),
			},
			{
				// Update back to unset.
				Config: ConfigLoggingSFTPBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_sftp.test", "placement"),
				),
			},
			{
				// The API's null response must leave no residual diff against the
				// same, still-unset config.
				Config:   ConfigLoggingSFTPBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_versionUpdateInPlace verifies that bumping
// the explicit resource's version argument is an in-place update against the
// new version rather than a destroy-and-recreate. The explicit clone workflow
// copies the endpoint into the new version, so version is intentionally not
// replacement-forcing (unlike service_id and name).
func TestAccFastlyServiceLoggingSFTP_versionUpdateInPlace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	var serviceID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingSFTPAtVersion(serviceName, domainName, loggerName, 1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "version", "1"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_sftp.test"]
						if !ok {
							return fmt.Errorf("sftp resource not found")
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
				Config: ConfigLoggingSFTPAtVersion(serviceName, domainName, loggerName, 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_logging_sftp.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "version", "2"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_sftp.test"]
						if !ok {
							return fmt.Errorf("sftp resource not found")
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
						if _, err := client.GetSFTP(context.Background(), &fastly.GetSFTPInput{
							ServiceID:      serviceID,
							ServiceVersion: 2,
							Name:           loggerName,
						}); err != nil {
							return fmt.Errorf("error fetching SFTP logging endpoint at version 2: %w", err)
						}

						return nil
					},
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_computeRejectsVCLOnlyFields verifies that
// fastly_service_logging_sftp rejects format (a VCL-only attribute) when
// attached to a Compute service. The standalone resource's schema is shared
// across both service types, so this is enforced by
// ValidateNoVCLOnlyAttributesForCompute at apply time rather than by the
// schema itself.
func TestAccFastlyServiceLoggingSFTP_computeRejectsVCLOnlyFields(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingSFTPComputeFormat(serviceName, loggerName),
				ExpectError: regexp.MustCompile("VCL-only attributes not supported on Compute services"),
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_formatDefault catches upstream changes to the
// format Fastly assigns when none is sent, which would leave
// constants.LoggingSFTPDefaultFormat stale. Compute is used because it's the
// only path that omits format from the request - on VCL the schema default is
// always sent, so the API just echoes our own constant back.
func TestAccFastlyServiceLoggingSFTP_formatDefault(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingSFTPCompute(serviceName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					CheckLoggingSFTPFormatDefault("fastly_service_compute.test", loggerName, 1),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingSFTP_computeConsistentAfterApply covers the whole
// plan -> API response -> flatten -> state path on a Compute service, which
// the unit tests cannot reach. The VCL-only attributes are never sent for
// Compute, but their schema defaults still land in the plan, so the API's own
// values (a different default format, and placement forced to "none" on wasm)
// used to be read back into state and fail Terraform's post-apply consistency
// check with "Provider produced inconsistent result after apply". The
// trailing PlanOnly step then proves the same values survive a refresh with
// no residual diff.
func TestAccFastlyServiceLoggingSFTP_computeConsistentAfterApply(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("sftp-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingSFTPCompute(serviceName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					CheckLoggingSFTPExistsInFastly("fastly_service_compute.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "name", loggerName),
					// The VCL-only attributes must hold their schema defaults, not
					// whatever the API returned for the wasm service.
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "format", constants.LoggingSFTPDefaultFormat),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_sftp.test", "response_condition", ""),
					resource.TestCheckNoResourceAttr("fastly_service_logging_sftp.test", "placement"),
				),
			},
			{
				Config:   ConfigLoggingSFTPCompute(serviceName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// CheckLoggingSFTPFormatDefault fails if the format Fastly reports for a
// logging endpoint differs from constants.LoggingSFTPDefaultFormat. Reads the
// API directly, since FlattenToComputeNestedModel writes the constant into
// state without consulting the response. Only meaningful on an endpoint
// created without a format in the request.
func CheckLoggingSFTPFormatDefault(serviceName, loggerName string, version int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[serviceName]
		if !ok {
			return fmt.Errorf("service not found: %s", serviceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		logger, err := client.GetSFTP(context.Background(), &fastly.GetSFTPInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           loggerName,
		})
		if err != nil {
			return fmt.Errorf("error fetching SFTP logging endpoint from Fastly: %w", err)
		}
		if logger == nil {
			return fmt.Errorf("SFTP logging endpoint %s not found in Fastly", loggerName)
		}

		if logger.Format == nil {
			return fmt.Errorf("Fastly returned a null format for SFTP logging endpoint %s, expected its default format", loggerName)
		}

		if got := *logger.Format; got != constants.LoggingSFTPDefaultFormat {
			return fmt.Errorf(
				"constants.LoggingSFTPDefaultFormat no longer matches the format Fastly assigns by default\ngot from API: %q\nconstant:     %q",
				got, constants.LoggingSFTPDefaultFormat,
			)
		}

		return nil
	}
}

// CheckLoggingSFTPExistsInFastly verifies an SFTP logging endpoint exists in
// the Fastly API.
func CheckLoggingSFTPExistsInFastly(serviceName, loggerName string, version int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[serviceName]
		if !ok {
			return fmt.Errorf("service not found: %s", serviceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		logger, err := client.GetSFTP(context.Background(), &fastly.GetSFTPInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           loggerName,
		})
		if err != nil {
			return fmt.Errorf("error fetching SFTP logging endpoint from Fastly: %w", err)
		}

		if logger == nil {
			return fmt.Errorf("SFTP logging endpoint %s not found in Fastly", loggerName)
		}

		return nil
	}
}
