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

// Every test that writes settings also enables Image Optimizer via
// fastly_service_product_image_optimizer, which requires the test account to be allowed to
// enable Image Optimizer.

func TestAccFastlyServiceImageOptimizerDefaultSettings_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigImageOptimizerDefaultSettingsBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "resize_filter", "bicubic"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "webp", "true"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "webp_quality", "70"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "jpeg_type", "progressive"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "jpeg_quality", "90"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "upscale", "true"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "allow_video", "true"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_image_optimizer_default_settings.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_image_optimizer_default_settings.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceImageOptimizerDefaultSettings_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigImageOptimizerDefaultSettingsBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "resize_filter", "bicubic"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "webp", "true"),
				),
			},
			{
				Config: ConfigImageOptimizerDefaultSettingsUpdated(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "resize_filter", "nearest"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "webp", "false"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "webp_quality", "50"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "jpeg_type", "baseline"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "jpeg_quality", "60"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "upscale", "false"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "allow_video", "false"),
				),
			},
		},
	})
}

func TestAccFastlyServiceImageOptimizerDefaultSettings_defaults(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				// Every optional attribute omitted must populate its documented default rather
				// than drifting on every plan.
				Config: ConfigImageOptimizerDefaultSettingsMinimal(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "resize_filter", "lanczos3"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "webp", "false"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "webp_quality", "85"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "jpeg_type", "auto"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "jpeg_quality", "85"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "upscale", "false"),
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "allow_video", "false"),
				),
			},
			{
				Config:   ConfigImageOptimizerDefaultSettingsMinimal(serviceName, domainName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceImageOptimizerDefaultSettings_computeServiceRejected verifies the resource
// is rejected on a Compute service, matching fastly_service_product_image_optimizer's CDN-only
// restriction.
func TestAccFastlyServiceImageOptimizerDefaultSettings_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigImageOptimizerDefaultSettingsOnComputeService(serviceName),
				ExpectError: regexp.MustCompile(`fastly_service_image_optimizer_default_settings\s+does\s+not\s+support\s+Fastly\s+service.*of\s+type\s+"Compute"`),
			},
		},
	})
}

// TestAccFastlyServiceImageOptimizerDefaultSettings_lockedVersion verifies that targeting an
// active (locked) version is rejected before any API write.
func TestAccFastlyServiceImageOptimizerDefaultSettings_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

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
				Config:      ConfigImageOptimizerDefaultSettingsOnLockedVersion(serviceName, domainName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceImageOptimizerDefaultSettings_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigImageOptimizerDefaultSettingsBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_image_optimizer_default_settings.test"]
						if !ok {
							return fmt.Errorf("image optimizer default settings resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_image_optimizer_default_settings.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s", serviceID, versionNumber), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceImageOptimizerDefaultSettings_destroyResetsDefaults verifies that destroying
// the resource resets the version's settings back to their API defaults rather than leaving the
// last-applied values in place - the settings always exist while Image Optimizer is enabled, so
// there is nothing to actually delete.
func TestAccFastlyServiceImageOptimizerDefaultSettings_destroyResetsDefaults(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	var serviceID string
	var versionNumber int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigImageOptimizerDefaultSettingsBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_image_optimizer_default_settings.test", "webp", "true"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_image_optimizer_default_settings.test"]
						if !ok {
							return fmt.Errorf("image optimizer default settings resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						_, err := fmt.Sscanf(rs.Primary.Attributes["version"], "%d", &versionNumber)
						return err
					},
				),
			},
			{
				// Image Optimizer stays enabled so the reset is observable remotely.
				Config: ConfigImageOptimizerDefaultSettingsEnabledOnly(serviceName, domainName),
				Check: func(_ *terraform.State) error {
					return ImageOptimizerDefaultSettingsMatchAPIDefaults(serviceID, versionNumber)
				},
			},
		},
	})
}
