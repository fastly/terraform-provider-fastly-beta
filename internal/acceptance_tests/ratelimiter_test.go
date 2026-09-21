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
)

func TestAccFastlyServiceRateLimiter_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	rateLimiterName := fmt.Sprintf("ratelimiter-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRateLimiterBasic(serviceName, domainName, rateLimiterName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "name", rateLimiterName),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "action", "log_only"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "logger_type", "s3"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "client_key.0", "req.http.Fastly-Client-IP"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "http_methods.0", "GET"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "http_methods.1", "POST"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "penalty_box_duration", "10"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "rps_limit", "100"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "window_size", "60"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "feature_revision", "1"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_ratelimiter.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_ratelimiter.test", "id"),
					resource.TestCheckResourceAttrSet("fastly_service_ratelimiter.test", "rate_limiter_id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceRateLimiter_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	rateLimiterName := fmt.Sprintf("ratelimiter-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRateLimiterBasic(serviceName, domainName, rateLimiterName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "rps_limit", "100"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "window_size", "60"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "penalty_box_duration", "10"),
				),
			},
			{
				// Updating in-place limits and http_methods/client_key must not delete+recreate
				// the rate limiter - none of the fields that force a recreate (uri_dictionary_name,
				// response_object_name, response) are touched here.
				Config: ConfigRateLimiterUpdated(serviceName, domainName, rateLimiterName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "rps_limit", "500"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "window_size", "10"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "penalty_box_duration", "20"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "client_key.#", "2"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "http_methods.#", "3"),
				),
			},
		},
	})
}

// TestAccFastlyServiceRateLimiter_clearingResponseRecreates confirms that switching action away
// from "response" and dropping the response block round-trips through a real delete+create
// (see ops.needsRecreate) rather than silently leaving the old response configured server-side,
// which is what a plain update would do since the Fastly API rejects an explicit empty response.
func TestAccFastlyServiceRateLimiter_clearingResponseRecreates(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	rateLimiterName := fmt.Sprintf("ratelimiter-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRateLimiterResponseAction(serviceName, domainName, rateLimiterName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "action", "response"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "response.content", "Rate limit exceeded"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "response.content_type", "text/plain"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "response.status", "429"),
				),
			},
			{
				Config: ConfigRateLimiterBasic(serviceName, domainName, rateLimiterName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "action", "log_only"),
					resource.TestCheckNoResourceAttr("fastly_service_ratelimiter.test", "response.content"),
				),
			},
		},
	})
}

// TestAccFastlyServiceRateLimiter_computeServiceRejected verifies that fastly_service_ratelimiter,
// a VCL-only resource (rate limiters are not supported for Compute services in the legacy
// provider either), is rejected when targeting a Compute service.
func TestAccFastlyServiceRateLimiter_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	rateLimiterName := fmt.Sprintf("ratelimiter-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigRateLimiterOnComputeService(serviceName, rateLimiterName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_ratelimiter does not support Fastly service.*of type "Compute"`),
			},
		},
	})
}

// TestAccFastlyServiceRateLimiter_lockedVersion verifies that the provider refuses to write a
// rate limiter to an activated (locked) service version. Version 1, which holds the service and
// domain the test cleans up afterward, is never activated - only a cloned version 2 is - so the
// locked-version write attempt itself never lands in state and cleanup of version 1 is
// unaffected.
func TestAccFastlyServiceRateLimiter_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	rateLimiterName := fmt.Sprintf("ratelimiter-%s", acctest.RandString(10))

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
				Config:      ConfigRateLimiterOnLockedVersion(serviceName, domainName, rateLimiterName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceRateLimiter_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	rateLimiterName := fmt.Sprintf("ratelimiter-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRateLimiterForImport(serviceName, domainName, rateLimiterName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_ratelimiter.test", "name", rateLimiterName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_ratelimiter.test"]
						if !ok {
							return fmt.Errorf("rate limiter resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_ratelimiter.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, rateLimiterName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
