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

func TestAccFastlyServiceCacheSetting_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	cacheSettingName := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCacheSettingBasic(serviceName, domainName, cacheSettingName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "name", cacheSettingName),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "action", "cache"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "ttl", "3600"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "stale_ttl", "120"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_cache_setting.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_cache_setting.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCacheSetting_minimal(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	cacheSettingName := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCacheSettingMinimal(serviceName, domainName, cacheSettingName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "name", cacheSettingName),
					resource.TestCheckNoResourceAttr("fastly_service_cache_setting.test", "action"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "ttl", "0"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "stale_ttl", "0"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCacheSetting_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	cacheSettingName := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCacheSettingBasic(serviceName, domainName, cacheSettingName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "action", "cache"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "ttl", "3600"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "stale_ttl", "120"),
				),
			},
			{
				Config: ConfigCacheSettingUpdated(serviceName, domainName, cacheSettingName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "action", "pass"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "ttl", "7200"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "stale_ttl", "300"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCacheSetting_withCacheCondition(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	cacheSettingName := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))
	conditionName := fmt.Sprintf("condition-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCacheSettingWithCacheCondition(serviceName, domainName, cacheSettingName, conditionName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "cache_condition", conditionName),
					resource.TestCheckResourceAttr("fastly_service_condition.cache", "type", "CACHE"),
				),
			},
		},
	})
}

// TestAccFastlyServiceCacheSetting_computeServiceRejected verifies that
// fastly_service_cache_setting, a VCL-only resource (cache settings are not supported for
// Compute services in the legacy provider either), is rejected when targeting a Compute service.
func TestAccFastlyServiceCacheSetting_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	cacheSettingName := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigCacheSettingOnComputeService(serviceName, cacheSettingName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_cache_setting does not support Fastly service.*of type "Compute"`),
			},
		},
	})
}

// TestAccFastlyServiceCacheSetting_lockedVersion verifies that the provider refuses to write a
// cache setting to an activated (locked) service version. Version 1, which holds the service and
// domain the test cleans up afterward, is never activated - only a cloned version 2 is - so the
// locked-version write attempt itself never lands in state and cleanup of version 1 is
// unaffected.
func TestAccFastlyServiceCacheSetting_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	cacheSettingName := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))

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
				Config:      ConfigCacheSettingOnLockedVersion(serviceName, domainName, cacheSettingName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceCacheSetting_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	cacheSettingName := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCacheSettingForImport(serviceName, domainName, cacheSettingName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "name", cacheSettingName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_cache_setting.test"]
						if !ok {
							return fmt.Errorf("cache setting resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_cache_setting.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, cacheSettingName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceCacheSetting_nameForcesReplace verifies changing name forces
// destroy/create rather than an in-place update.
func TestAccFastlyServiceCacheSetting_nameForcesReplace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	cacheSettingName1 := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))
	cacheSettingName2 := fmt.Sprintf("cache-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCacheSettingBasic(serviceName, domainName, cacheSettingName1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "name", cacheSettingName1),
				),
			},
			{
				Config: ConfigCacheSettingBasic(serviceName, domainName, cacheSettingName2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_cache_setting.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("fastly_service_cache_setting.test", "name", cacheSettingName2),
			},
		},
	})
}
