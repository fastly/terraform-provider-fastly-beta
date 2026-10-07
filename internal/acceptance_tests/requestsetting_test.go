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

func TestAccFastlyServiceRequestSetting_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	requestSettingName := fmt.Sprintf("request-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRequestSettingBasic(serviceName, domainName, requestSettingName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "name", requestSettingName),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "action", "lookup"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "bypass_busy_wait", "true"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "default_host", "host.example.com"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "force_miss", "true"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "force_ssl", "true"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "hash_keys", "req.url,req.http.host"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "max_stale_age", "120"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "timer_support", "true"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "xff", "append"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_request_setting.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_request_setting.test", "id"),
				),
			},
			{
				Config:   ConfigRequestSettingBasic(serviceName, domainName, requestSettingName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceRequestSetting_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	requestSettingName := fmt.Sprintf("request-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRequestSettingBasic(serviceName, domainName, requestSettingName),
				Check:  resource.TestCheckResourceAttr("fastly_service_request_setting.test", "action", "lookup"),
			},
			{
				Config: ConfigRequestSettingUpdated(serviceName, domainName, requestSettingName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_request_setting.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "name", requestSettingName),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "action", "pass"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "bypass_busy_wait", "false"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "default_host", "other.example.com"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "force_miss", "false"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "force_ssl", "false"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "hash_keys", "req.http.host"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "max_stale_age", "300"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "timer_support", "false"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "xff", "clear"),
				),
			},
			{
				Config:   ConfigRequestSettingUpdated(serviceName, domainName, requestSettingName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceRequestSetting_clearOptionalEnums(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	requestSettingName := fmt.Sprintf("request-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRequestSettingUpdated(serviceName, domainName, requestSettingName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "action", "pass"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "xff", "clear"),
				),
			},
			{
				Config: ConfigRequestSettingMinimal(serviceName, domainName, requestSettingName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("fastly_service_request_setting.test", "action"),
					resource.TestCheckNoResourceAttr("fastly_service_request_setting.test", "xff"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "bypass_busy_wait", "false"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "default_host", ""),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "force_miss", "false"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "force_ssl", "false"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "hash_keys", ""),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "max_stale_age", "0"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "request_condition", ""),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "timer_support", "false"),
				),
			},
			{
				Config:   ConfigRequestSettingMinimal(serviceName, domainName, requestSettingName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceRequestSetting_withRequestCondition(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	requestSettingName := fmt.Sprintf("request-setting-%s", acctest.RandString(10))
	conditionName := fmt.Sprintf("condition-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRequestSettingWithRequestCondition(serviceName, domainName, requestSettingName, conditionName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_condition.request", "type", "REQUEST"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "request_condition", conditionName),
				),
			},
		},
	})
}

// TestAccFastlyServiceRequestSetting_computeServiceRejected verifies Request Settings remain
// VCL/CDN-only when represented as a first-class explicit resource.
func TestAccFastlyServiceRequestSetting_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	requestSettingName := fmt.Sprintf("request-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigRequestSettingOnComputeService(serviceName, requestSettingName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_request_setting does not support Fastly service.*of type "Compute"`),
			},
		},
	})
}

// TestAccFastlyServiceRequestSetting_lockedVersion verifies the provider refuses to write to an
// activated (locked) service version.
func TestAccFastlyServiceRequestSetting_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	requestSettingName := fmt.Sprintf("request-setting-%s", acctest.RandString(10))

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
				Config:      ConfigRequestSettingOnLockedVersion(serviceName, domainName, requestSettingName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceRequestSetting_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	requestSettingName := fmt.Sprintf("request-setting-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRequestSettingForImport(serviceName, domainName, requestSettingName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "name", requestSettingName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_request_setting.test"]
						if !ok {
							return fmt.Errorf("request setting resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_request_setting.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, requestSettingName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceRequestSetting_nameForcesReplace verifies changing the stable name identity
// replaces the resource instead of attempting the API's rename path in place.
func TestAccFastlyServiceRequestSetting_nameForcesReplace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	requestSettingName1 := fmt.Sprintf("request-setting-%s", acctest.RandString(10))
	requestSettingName2 := fmt.Sprintf("request-setting-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRequestSettingBasic(serviceName, domainName, requestSettingName1),
				Check:  resource.TestCheckResourceAttr("fastly_service_request_setting.test", "name", requestSettingName1),
			},
			{
				Config: ConfigRequestSettingBasic(serviceName, domainName, requestSettingName2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_request_setting.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("fastly_service_request_setting.test", "name", requestSettingName2),
			},
		},
	})
}

// TestAccFastlyServiceRequestSetting_delete verifies removing only the first-class Request Setting
// deletes the remote object while leaving the explicit service/version in place.
func TestAccFastlyServiceRequestSetting_delete(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	requestSettingName := fmt.Sprintf("request-setting-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigRequestSettingBasic(serviceName, domainName, requestSettingName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_request_setting.test", "name", requestSettingName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_request_setting.test"]
						if !ok {
							return fmt.Errorf("request setting resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						if _, err := fmt.Sscanf(rs.Primary.Attributes["version"], "%d", &versionNumber); err != nil {
							return fmt.Errorf("error parsing request setting version: %w", err)
						}
						return nil
					},
				),
			},
			{
				Config: ConfigServiceCDNWithDomain(serviceName, domainName, 1),
				Check: func(_ *terraform.State) error {
					client, err := NewFastlyClient()
					if err != nil {
						return fmt.Errorf("error creating Fastly client: %w", err)
					}
					items, err := client.ListRequestSettings(context.Background(), &fastly.ListRequestSettingsInput{
						ServiceID:      serviceID,
						ServiceVersion: versionNumber,
					})
					if err != nil {
						return fmt.Errorf("error listing request settings after delete: %w", err)
					}
					for _, item := range items {
						if item != nil && fastly.ToValue(item.Name) == requestSettingName {
							return fmt.Errorf("request setting %q still exists after Terraform delete", requestSettingName)
						}
					}
					return nil
				},
			},
		},
	})
}
