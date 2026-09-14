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

func TestAccFastlyServiceLoggingElasticsearch_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingElasticsearchBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "index", "logs-index"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "url", "https://elasticsearch.example.com"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "processing_region", "none"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "request_max_bytes", "0"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "request_max_entries", "0"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_elasticsearch.test", "format"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_elasticsearch.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_elasticsearch.test", "id"),
				),
			},
			{
				// The default format is a Computed default sent verbatim to the API,
				// so it must round-trip byte-for-byte and leave no residual diff.
				Config:   ConfigLoggingElasticsearchBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingElasticsearch_emptyFormat verifies an explicit
// format = "" is rejected at validate time rather than failing apply.
func TestAccFastlyServiceLoggingElasticsearch_emptyFormat(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingElasticsearchEmptyFormat(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile("`format` cannot be explicitly set to an empty string"),
			},
		},
	})
}

func TestAccFastlyServiceLoggingElasticsearch_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingElasticsearchBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "index", "logs-index"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "processing_region", "none"),
				),
			},
			{
				Config: ConfigLoggingElasticsearchUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "index", "logs-index-updated"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "url", "https://elasticsearch-updated.example.com"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "pipeline", "my-pipeline"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "authentication.user", "es-user"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "authentication.password", "es-password"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "request_max_bytes", "1000"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "request_max_entries", "100"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "format", "%h %l %u %t \"%r\" %>s %b"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "placement", "none"),
				),
			},
		},
	})
}

func TestAccFastlyServiceLoggingElasticsearch_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingElasticsearchForImport(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "name", loggerName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_elasticsearch.test"]
						if !ok {
							return fmt.Errorf("elasticsearch resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_logging_elasticsearch.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, loggerName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingElasticsearch_clearToDefaults verifies removing
// optional attributes reverts each to its schema default.
func TestAccFastlyServiceLoggingElasticsearch_clearToDefaults(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingElasticsearchUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "pipeline", "my-pipeline"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "processing_region", "eu"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "placement", "none"),
				),
			},
			{
				Config: ConfigLoggingElasticsearchBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "pipeline", ""),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "processing_region", "none"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "authentication.user", ""),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "authentication.password", ""),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "request_max_bytes", "0"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "request_max_entries", "0"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "response_condition", ""),
					// unset placement is distinct from "none" — see below.
					resource.TestCheckNoResourceAttr("fastly_service_logging_elasticsearch.test", "placement"),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingElasticsearch_placementUnsetVsNone verifies
// unset placement and explicit "none" are distinct, round-trippable states.
func TestAccFastlyServiceLoggingElasticsearch_placementUnsetVsNone(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				// Start unset.
				Config: ConfigLoggingElasticsearchBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_elasticsearch.test", "placement"),
				),
			},
			{
				// Update to explicit "none".
				Config: ConfigLoggingElasticsearchUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "placement", "none"),
				),
			},
			{
				// Update back to unset.
				Config: ConfigLoggingElasticsearchBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_elasticsearch.test", "placement"),
				),
			},
			{
				// The API's null response must leave no residual diff against the
				// same, still-unset config.
				Config:   ConfigLoggingElasticsearchBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceLoggingElasticsearch_versionUpdateInPlace verifies
// bumping version is an in-place update, not a destroy-and-recreate.
func TestAccFastlyServiceLoggingElasticsearch_versionUpdateInPlace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	var serviceID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingElasticsearchAtVersion(serviceName, domainName, loggerName, 1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "version", "1"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_elasticsearch.test"]
						if !ok {
							return fmt.Errorf("elasticsearch resource not found")
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
				Config: ConfigLoggingElasticsearchAtVersion(serviceName, domainName, loggerName, 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_logging_elasticsearch.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "version", "2"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_logging_elasticsearch.test"]
						if !ok {
							return fmt.Errorf("elasticsearch resource not found")
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
						if _, err := client.GetElasticsearch(context.Background(), &fastly.GetElasticsearchInput{
							ServiceID:      serviceID,
							ServiceVersion: 2,
							Name:           loggerName,
						}); err != nil {
							return fmt.Errorf("error fetching Elasticsearch logging endpoint at version 2: %w", err)
						}

						return nil
					},
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingElasticsearch_computeRejectsVCLOnlyFields
// verifies format is rejected at apply time when attached to Compute.
func TestAccFastlyServiceLoggingElasticsearch_computeRejectsVCLOnlyFields(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingElasticsearchComputeFormat(serviceName, loggerName),
				ExpectError: regexp.MustCompile("VCL-only attributes not supported on Compute services"),
			},
		},
	})
}

// TestAccFastlyServiceLoggingElasticsearch_formatDefault catches upstream
// changes to Fastly's default format, which would leave
// constants.LoggingElasticsearchDefaultFormat stale.
func TestAccFastlyServiceLoggingElasticsearch_formatDefault(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingElasticsearchCompute(serviceName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					CheckLoggingElasticsearchFormatDefault("fastly_service_compute.test", loggerName, 1),
				),
			},
		},
	})
}

// TestAccFastlyServiceLoggingElasticsearch_computeConsistentAfterApply
// covers the plan -> API -> flatten -> state path on Compute, which unit
// tests can't reach: the API's own VCL-only values must not leak into state.
func TestAccFastlyServiceLoggingElasticsearch_computeConsistentAfterApply(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("elasticsearch-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingElasticsearchCompute(serviceName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					CheckLoggingElasticsearchExistsInFastly("fastly_service_compute.test", loggerName, 1),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "name", loggerName),
					// The VCL-only attributes must hold their schema defaults, not
					// whatever the API returned for the wasm service.
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "format", constants.LoggingElasticsearchDefaultFormat),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "format_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_logging_elasticsearch.test", "response_condition", ""),
					resource.TestCheckNoResourceAttr("fastly_service_logging_elasticsearch.test", "placement"),
				),
			},
			{
				Config:   ConfigLoggingElasticsearchCompute(serviceName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

// CheckLoggingElasticsearchFormatDefault fails if the format Fastly reports
// differs from constants.LoggingElasticsearchDefaultFormat.
func CheckLoggingElasticsearchFormatDefault(serviceName, loggerName string, version int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[serviceName]
		if !ok {
			return fmt.Errorf("service not found: %s", serviceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		logger, err := client.GetElasticsearch(context.Background(), &fastly.GetElasticsearchInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           loggerName,
		})
		if err != nil {
			return fmt.Errorf("error fetching Elasticsearch logging endpoint from Fastly: %w", err)
		}
		if logger == nil {
			return fmt.Errorf("Elasticsearch logging endpoint %s not found in Fastly", loggerName)
		}

		if logger.Format == nil {
			return fmt.Errorf("Fastly returned a null format for Elasticsearch logging endpoint %s, expected its default format", loggerName)
		}

		if got := *logger.Format; got != constants.LoggingElasticsearchDefaultFormat {
			return fmt.Errorf(
				"constants.LoggingElasticsearchDefaultFormat no longer matches the format Fastly assigns by default\ngot from API: %q\nconstant:     %q",
				got, constants.LoggingElasticsearchDefaultFormat,
			)
		}

		return nil
	}
}

// CheckLoggingElasticsearchExistsInFastly verifies an Elasticsearch logging
// endpoint exists in the Fastly API.
func CheckLoggingElasticsearchExistsInFastly(serviceName, loggerName string, version int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[serviceName]
		if !ok {
			return fmt.Errorf("service not found: %s", serviceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		logger, err := client.GetElasticsearch(context.Background(), &fastly.GetElasticsearchInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           loggerName,
		})
		if err != nil {
			return fmt.Errorf("error fetching Elasticsearch logging endpoint from Fastly: %w", err)
		}

		if logger == nil {
			return fmt.Errorf("Elasticsearch logging endpoint %s not found in Fastly", loggerName)
		}

		return nil
	}
}
