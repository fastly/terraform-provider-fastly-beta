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

func TestAccFastlyServiceGzip_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	gzipName := fmt.Sprintf("gzip-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigGzipBasic(serviceName, domainName, gzipName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "name", gzipName),
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "content_types.#", "2"),
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "extensions.#", "2"),
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_gzip.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_gzip.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceGzip_minimal(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	gzipName := fmt.Sprintf("gzip-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigGzipMinimal(serviceName, domainName, gzipName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "name", gzipName),
					resource.TestCheckNoResourceAttr("fastly_service_gzip.test", "content_types"),
					resource.TestCheckNoResourceAttr("fastly_service_gzip.test", "extensions"),
				),
			},
			{
				// The API silently substitutes a default content_types/extensions list when
				// they're left unset - re-applying the same config must not show drift.
				Config:   ConfigGzipMinimal(serviceName, domainName, gzipName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceGzip_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	gzipName := fmt.Sprintf("gzip-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigGzipBasic(serviceName, domainName, gzipName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "content_types.#", "2"),
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "extensions.#", "2"),
				),
			},
			{
				Config: ConfigGzipUpdated(serviceName, domainName, gzipName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "content_types.#", "3"),
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "extensions.#", "3"),
				),
			},
		},
	})
}

func TestAccFastlyServiceGzip_withCacheCondition(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	gzipName := fmt.Sprintf("gzip-%s", acctest.RandString(10))
	conditionName := fmt.Sprintf("condition-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigGzipWithCacheCondition(serviceName, domainName, gzipName, conditionName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "cache_condition", conditionName),
					resource.TestCheckResourceAttr("fastly_service_condition.cache", "type", "CACHE"),
				),
			},
		},
	})
}

// TestAccFastlyServiceGzip_computeServiceRejected verifies that fastly_service_gzip, a VCL-only
// resource (gzip configurations are not supported for Compute services in the legacy provider
// either), is rejected when targeting a Compute service.
func TestAccFastlyServiceGzip_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	gzipName := fmt.Sprintf("gzip-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigGzipOnComputeService(serviceName, gzipName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_gzip does not support Fastly service.*of type "Compute"`),
			},
		},
	})
}

// TestAccFastlyServiceGzip_lockedVersion verifies that the provider refuses to write a gzip
// configuration to an activated (locked) service version. Version 1, which holds the service and
// domain the test cleans up afterward, is never activated - only a cloned version 2 is - so the
// locked-version write attempt itself never lands in state and cleanup of version 1 is
// unaffected.
func TestAccFastlyServiceGzip_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	gzipName := fmt.Sprintf("gzip-%s", acctest.RandString(10))

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
				Config:      ConfigGzipOnLockedVersion(serviceName, domainName, gzipName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceGzip_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	gzipName := fmt.Sprintf("gzip-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigGzipForImport(serviceName, domainName, gzipName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_gzip.test", "name", gzipName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_gzip.test"]
						if !ok {
							return fmt.Errorf("gzip resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_gzip.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, gzipName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
