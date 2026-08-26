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
	"github.com/fastly/terraform-provider-fastly/internal/errors"
)

func TestAccFastlyAlert_statsAccountWide(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	name := fmt.Sprintf("tf-test-alert-%s", suffix)
	updatedName := fmt.Sprintf("tf-test-alert-updated-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigAlertStatsAccountWide(name, "created by acceptance test", "status_5xx", "above_threshold", "5m", 10),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_alert.test", "name", name),
					resource.TestCheckResourceAttr("fastly_alert.test", "description", "created by acceptance test"),
					resource.TestCheckResourceAttr("fastly_alert.test", "source", "stats"),
					resource.TestCheckResourceAttr("fastly_alert.test", "metric", "status_5xx"),
					resource.TestCheckNoResourceAttr("fastly_alert.test", "service_id"),
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.type", "above_threshold"),
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.period", "5m"),
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.threshold", "10"),
					resource.TestCheckResourceAttrSet("fastly_alert.test", "id"),
					CheckAlertRemoteDescription("fastly_alert.test", "created by acceptance test"),
				),
			},
			{
				Config: ConfigAlertStatsAccountWide(updatedName, "updated by acceptance test", "status_4xx", "below_threshold", "15m", 100),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_alert.test", "name", updatedName),
					resource.TestCheckResourceAttr("fastly_alert.test", "description", "updated by acceptance test"),
					resource.TestCheckResourceAttr("fastly_alert.test", "metric", "status_4xx"),
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.type", "below_threshold"),
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.period", "15m"),
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.threshold", "100"),
					CheckAlertRemoteDescription("fastly_alert.test", "updated by acceptance test"),
				),
			},
			{
				Config:   ConfigAlertStatsAccountWide(updatedName, "updated by acceptance test", "status_4xx", "below_threshold", "15m", 100),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_alert.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyAlert_percentIncreaseWithIgnoreBelow(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-alert-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAlertDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigAlertPercentIncreaseWithIgnoreBelow(name, 0.25, 10),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.type", "percent_increase"),
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.threshold", "0.25"),
					resource.TestCheckResourceAttr("fastly_alert.test", "evaluation_strategy.0.ignore_below", "10"),
				),
			},
			{
				ResourceName:      "fastly_alert.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyAlert_domainsScoped(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	serviceName := fmt.Sprintf("tf-test-%s", suffix)
	domainName := fmt.Sprintf("%s.example.com", suffix)
	alertName := fmt.Sprintf("tf-test-alert-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy: func(s *terraform.State) error {
			if err := CheckAlertDestroy(s); err != nil {
				return err
			}
			return CheckServiceDestroy("fastly_service_cdn_auto")(s)
		},
		Steps: []resource.TestStep{
			{
				Config: ConfigAlertDomainsScoped(serviceName, domainName, alertName, "domain scoped alert", "status_5xx", "above_threshold", "5m", 10),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_alert.test", "name", alertName),
					resource.TestCheckResourceAttr("fastly_alert.test", "source", "domains"),
					resource.TestCheckResourceAttrPair("fastly_alert.test", "service_id", "fastly_service_cdn_auto.test", "id"),
					resource.TestCheckResourceAttr("fastly_alert.test", "dimensions.0.domains.#", "1"),
					resource.TestCheckTypeSetElemAttr("fastly_alert.test", "dimensions.0.domains.*", domainName),
				),
			},
			{
				ResourceName:      "fastly_alert.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyAlert_missingServiceIDValidationError(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-alert-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigAlertDomainsMissingServiceID(name, "status_5xx"),
				ExpectError: regexp.MustCompile("empty `service_id` is only supported for `stats` as a source"),
			},
		},
	})
}

func CheckAlertDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_alert" {
			continue
		}

		_, err := client.GetAlertDefinition(context.Background(), &fastly.GetAlertDefinitionInput{ID: &rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Alert %q was destroyed: %w", rs.Primary.ID, err)
		}

		return fmt.Errorf("Alert %s still exists", rs.Primary.ID)
	}

	return nil
}

func CheckAlertRemoteDescription(resourceName, expectedDescription string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		ad, err := client.GetAlertDefinition(context.Background(), &fastly.GetAlertDefinitionInput{ID: &rs.Primary.ID})
		if err != nil {
			return fmt.Errorf("error reading Alert %q: %w", rs.Primary.ID, err)
		}

		wantRemote := expectedDescription + " Managed by Terraform"
		if ad.Description != wantRemote {
			return fmt.Errorf("unexpected remote Alert description: got %q, want %q", ad.Description, wantRemote)
		}

		return nil
	}
}
