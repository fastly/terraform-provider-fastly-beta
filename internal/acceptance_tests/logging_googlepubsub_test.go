package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
)

func TestAccFastlyServiceLoggingGooglePubSub_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingGooglePubSubBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "project_id", "fastly-test-project"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "topic", "fastly-test-topic"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.email", "test-pubsub@fastly-test-project.iam.gserviceaccount.com"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_googlepubsub.test", "authentication.secret_key"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "processing_region", "none"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_googlepubsub.test", "format"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_googlepubsub.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_googlepubsub.test", "id"),
				),
			},
			{
				// The default format is a Computed default sent verbatim to the API,
				// so it must round-trip byte-for-byte and leave no residual diff.
				Config:   ConfigLoggingGooglePubSubBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingGooglePubSub_placementUnsetVsNone verifies unset
// placement and explicit "none" round-trip as distinct states across updates
// in both directions. Mirrors TestAccFastlyServiceLoggingBigQuery_placementUnsetVsNone.
func TestAccFastlyServiceLoggingGooglePubSub_placementUnsetVsNone(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				// Start unset.
				Config: ConfigLoggingGooglePubSubBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_googlepubsub.test", "placement"),
				),
			},
			{
				// Update to explicit "none".
				Config: ConfigLoggingGooglePubSubUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "placement", "none"),
				),
			},
			{
				// Update back to unset.
				Config: ConfigLoggingGooglePubSubBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_googlepubsub.test", "placement"),
				),
			},
			{
				// The API's null response must leave no residual diff against the
				// same, still-unset config.
				Config:   ConfigLoggingGooglePubSubBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingGooglePubSub_authEnvDefaults verifies that
// account_name, email, and secret_key still pick up FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME /
// FASTLY_GOOGLE_PUBSUB_EMAIL / FASTLY_GOOGLE_PUBSUB_SECRET_KEY when the entire
// authentication object is omitted from config.
//
// Not run in parallel: t.Setenv panics if the test also calls t.Parallel, and
// this test needs the env vars set for its own duration only.
func TestAccFastlyServiceLoggingGooglePubSub_authEnvDefaults(t *testing.T) {
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", "test-service-account")
	t.Setenv("FASTLY_GOOGLE_PUBSUB_EMAIL", "")
	t.Setenv("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingGooglePubSubNoAuth(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.account_name", "test-service-account"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.email", ""),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.secret_key", ""),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingGooglePubSub_authEnvDefaultsDeprecatedFallback
// verifies that account_name still picks up the deprecated
// FASTLY_GCS_ACCOUNT_NAME environment variable (used by the live provider,
// which borrowed GCS's env var for Pub/Sub too) when
// FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME is unset and the entire authentication
// object is omitted from config.
//
// Not run in parallel: t.Setenv panics if the test also calls t.Parallel, and
// this test needs the env vars set for its own duration only.
func TestAccFastlyServiceLoggingGooglePubSub_authEnvDefaultsDeprecatedFallback(t *testing.T) {
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", "")
	t.Setenv("FASTLY_GCS_ACCOUNT_NAME", "test-legacy-service-account")
	t.Setenv("FASTLY_GOOGLE_PUBSUB_EMAIL", "")
	t.Setenv("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingGooglePubSubNoAuth(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.account_name", "test-legacy-service-account"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.email", ""),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.secret_key", ""),
				),
			},
		},
	})
}

func TestAccFastlyServiceLoggingGooglePubSub_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingGooglePubSubBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "topic", "fastly-test-topic"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "processing_region", "none"),
				),
			},
			{
				Config: ConfigLoggingGooglePubSubUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "topic", "fastly-test-topic-updated"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.email", "updated-pubsub@fastly-test-project.iam.gserviceaccount.com"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_googlepubsub.test", "authentication.secret_key"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "format", "%h %l %u %t \"%r\" %>s %b"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "placement", "none"),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingGooglePubSub_accountNameToEmailSecretKey verifies
// that switching an existing endpoint's authentication from account_name to
// email/secret_key actually clears account_name on the API side via
// UpdateOrRecreate (delete+recreate), since the API rejects an explicit empty
// account_name on update.
func TestAccFastlyServiceLoggingGooglePubSub_accountNameToEmailSecretKey(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingGooglePubSubAccountName(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.account_name", "test-service-account"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.email", ""),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.secret_key", ""),
				),
			},
			{
				Config: ConfigLoggingGooglePubSubBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.account_name", ""),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "authentication.email", "test-pubsub@fastly-test-project.iam.gserviceaccount.com"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_googlepubsub.test", "authentication.secret_key"),
				),
			},
			{
				// The recreated endpoint's own state must leave no residual diff
				// against the same config on a subsequent refresh.
				Config:   ConfigLoggingGooglePubSubBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceLoggingGooglePubSub_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingGooglePubSubForImport(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_googlepubsub.test", "name", loggerName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_googlepubsub.test"]
						if !ok {
							return fmt.Errorf("pubsub resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_logging_googlepubsub.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, loggerName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyServiceLoggingGooglePubSub_computeRejectsVCLOnlyFields(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingGooglePubSubComputeFormat(serviceName, loggerName),
				ExpectError: regexp.MustCompile("VCL-only attributes not supported on Compute services"),
			},
		},
	})
}

// TestAccFastlyServiceLoggingGooglePubSub_formatDefault catches upstream
// changes to the format Fastly assigns when none is sent, which would leave
// constants.LoggingGooglePubSubDefaultFormat stale. Compute is used because
// it's the only path that omits format from the request - on VCL the schema
// default is always sent, so the API just echoes our own constant back.
func TestAccFastlyServiceLoggingGooglePubSub_formatDefault(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("pubsub-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingGooglePubSubCompute(serviceName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					CheckLoggingGooglePubSubFormatDefault("fastly_service_compute.test", loggerName, 1),
				),
			},
		},
	})
}

// CheckLoggingGooglePubSubFormatDefault fails if the format Fastly reports for
// a logging endpoint differs from constants.LoggingGooglePubSubDefaultFormat.
// Reads the API directly, since FlattenToComputeNestedModel writes the
// constant into state without consulting the response. Only meaningful on an
// endpoint created without a format in the request.
func CheckLoggingGooglePubSubFormatDefault(serviceName, loggerName string, version int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[serviceName]
		if !ok {
			return fmt.Errorf("service not found: %s", serviceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		logger, err := client.GetPubsub(context.Background(), &fastly.GetPubsubInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           loggerName,
		})
		if err != nil {
			return fmt.Errorf("error fetching Pub/Sub logging endpoint from Fastly: %w", err)
		}
		if logger == nil {
			return fmt.Errorf("Pub/Sub logging endpoint %s not found in Fastly", loggerName)
		}

		if logger.Format == nil {
			return fmt.Errorf("Fastly returned a null format for Pub/Sub logging endpoint %s, expected its default format", loggerName)
		}

		if got := *logger.Format; got != constants.LoggingGooglePubSubDefaultFormat {
			return fmt.Errorf(
				"constants.LoggingGooglePubSubDefaultFormat no longer matches the format Fastly assigns by default\ngot from API: %q\nconstant:     %q",
				got, constants.LoggingGooglePubSubDefaultFormat,
			)
		}

		return nil
	}
}
