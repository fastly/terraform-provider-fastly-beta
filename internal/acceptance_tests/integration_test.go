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
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
)

func TestAccFastlyIntegration_webhook(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	name := fmt.Sprintf("tf-test-integration-%s", suffix)
	updatedName := fmt.Sprintf("tf-test-integration-updated-%s", suffix)
	var integrationID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigIntegration(name, "created by acceptance test", "webhook", map[string]string{
					"webhook": "https://example.com/hooks/one",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_integration.test", "name", name),
					resource.TestCheckResourceAttr("fastly_integration.test", "description", "created by acceptance test"),
					resource.TestCheckResourceAttr("fastly_integration.test", "type", "webhook"),
					resource.TestCheckResourceAttr("fastly_integration.test", "config.webhook", "https://example.com/hooks/one"),
					resource.TestCheckResourceAttrSet("fastly_integration.test", "id"),
					CheckIntegrationRemoteState("fastly_integration.test", name, "created by acceptance test", "webhook"),
					captureResourceID("fastly_integration.test", &integrationID),
				),
			},
			{
				Config: ConfigIntegration(updatedName, "updated by acceptance test", "webhook", map[string]string{
					"webhook": "https://example.com/hooks/two",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_integration.test", "name", updatedName),
					resource.TestCheckResourceAttr("fastly_integration.test", "description", "updated by acceptance test"),
					resource.TestCheckResourceAttr("fastly_integration.test", "config.webhook", "https://example.com/hooks/two"),
					CheckIntegrationRemoteState("fastly_integration.test", updatedName, "updated by acceptance test", "webhook"),
					// Same ID across the update confirms this was an in-place update, not a delete+recreate.
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_integration.test"]
						if !ok {
							return fmt.Errorf("resource not found in state: fastly_integration.test")
						}
						if rs.Primary.ID != integrationID {
							return fmt.Errorf("expected Integration ID to remain %q across update, got %q", integrationID, rs.Primary.ID)
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "fastly_integration.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config"},
			},
		},
	})
}

// TestAccFastlyIntegration_datadog covers a config with a secret field (apikey).
func TestAccFastlyIntegration_datadog(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-integration-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigIntegration(name, "created by acceptance test", fastly.IntegrationTypeDatadog, map[string]string{
					"apikey": acctest.RandString(20),
					"site":   "datadoghq.eu",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_integration.test", "type", fastly.IntegrationTypeDatadog),
					resource.TestCheckResourceAttr("fastly_integration.test", "config.site", "datadoghq.eu"),
					CheckIntegrationRemoteState("fastly_integration.test", name, "created by acceptance test", fastly.IntegrationTypeDatadog),
				),
			},
			{
				ResourceName:            "fastly_integration.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config"},
			},
		},
	})
}

func TestAccFastlyIntegration_invalidTypeValidationError(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-integration-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigIntegrationInvalidType(name),
				ExpectError: regexp.MustCompile("Attribute type value must be one of"),
			},
		},
	})
}

func TestAccFastlyIntegration_jiraissue(t *testing.T) {
	testAccFastlyIntegrationBasic(t, fastly.IntegrationTypeJiraIssue, map[string]string{
		"baseurl": "https://my-org.atlassian.net", "username": "user@my-org.com",
		"token": acctest.RandString(10), "projectkey": "ABC", "issuetype": "Bug",
	})
}

func TestAccFastlyIntegration_jsm(t *testing.T) {
	testAccFastlyIntegrationBasic(t, fastly.IntegrationTypeJSM, map[string]string{"apikey": acctest.RandString(10)})
}

func TestAccFastlyIntegration_microsoftteams(t *testing.T) {
	testAccFastlyIntegrationBasic(t, "microsoftteams", map[string]string{"webhook": "https://example.webhook.office.com"})
}

func TestAccFastlyIntegration_newrelic(t *testing.T) {
	testAccFastlyIntegrationBasic(t, "newrelic", map[string]string{"key": acctest.RandString(10), "account": acctest.RandString(6)})
}

func TestAccFastlyIntegration_opsgenie(t *testing.T) {
	testAccFastlyIntegrationBasic(t, fastly.IntegrationTypeOpsGenie, map[string]string{"apikey": acctest.RandString(10)})
}

func TestAccFastlyIntegration_pagerduty(t *testing.T) {
	testAccFastlyIntegrationBasic(t, "pagerduty", map[string]string{"key": acctest.RandString(10)})
}

func TestAccFastlyIntegration_slack(t *testing.T) {
	testAccFastlyIntegrationBasic(t, "slack", map[string]string{
		"webhook": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
	})
}

func TestAccFastlyIntegration_splunkoncall(t *testing.T) {
	testAccFastlyIntegrationBasic(t, fastly.IntegrationTypeSplunkOnCall, map[string]string{
		"url": "https://alert.victorops.com/integrations/generic/20131114/alert/x",
	})
}

// testAccFastlyIntegrationBasic creates one integration of typ and verifies it against the live API.
func testAccFastlyIntegrationBasic(t *testing.T, typ string, config map[string]string) {
	t.Helper()
	t.Parallel()

	name := fmt.Sprintf("tf-test-integration-%s-%s", typ, acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigIntegration(name, "created by acceptance test", typ, config),
				Check: resource.ComposeTestCheckFunc(
					CheckIntegrationRemoteState("fastly_integration.test", name, "created by acceptance test", typ),
				),
			},
		},
	})
}

// TestAccFastlyIntegration_mailinglist covers the type with the "needs confirmation" warning.
func TestAccFastlyIntegration_mailinglist(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-integration-%s", acctest.RandString(10))
	address := fmt.Sprintf("noreply-%s@fastly.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigIntegration(name, "created by acceptance test", "mailinglist", map[string]string{
					"address": address,
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_integration.test", "type", "mailinglist"),
					resource.TestCheckResourceAttr("fastly_integration.test", "config.address", address),
					CheckIntegrationRemoteState("fastly_integration.test", name, "created by acceptance test", "mailinglist"),
				),
			},
			{
				ResourceName:            "fastly_integration.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config"},
			},
		},
	})
}

// TestAccFastlyIntegration_recreateAfterManualDelete confirms an out-of-band delete is recreated on apply.
func TestAccFastlyIntegration_recreateAfterManualDelete(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-integration-%s", acctest.RandString(10))
	config := ConfigIntegration(name, "created by acceptance test", "slack", map[string]string{
		"webhook": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
	})

	var integrationID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					CheckIntegrationRemoteState("fastly_integration.test", name, "created by acceptance test", "slack"),
					captureResourceID("fastly_integration.test", &integrationID),
				),
			},
			{
				PreConfig: func() {
					client, err := NewFastlyClient()
					if err != nil {
						t.Fatalf("error creating Fastly client: %s", err)
					}
					if err := client.DeleteIntegration(context.Background(), &fastly.DeleteIntegrationInput{ID: integrationID}); err != nil {
						t.Fatalf("error manually deleting Integration: %s", err)
					}
				},
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					CheckIntegrationRemoteState("fastly_integration.test", name, "created by acceptance test", "slack"),
				),
			},
		},
	})
}

func CheckIntegrationDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_integration" {
			continue
		}

		_, err := client.GetIntegration(context.Background(), &fastly.GetIntegrationInput{ID: rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Integration %q was destroyed: %w", rs.Primary.ID, err)
		}

		return fmt.Errorf("Integration %s still exists", rs.Primary.ID)
	}

	return nil
}

// CheckIntegrationRemoteState verifies name/description/type directly against the live API.
func CheckIntegrationRemoteState(resourceName, expectedName, expectedDescription, expectedType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		i, err := client.GetIntegration(context.Background(), &fastly.GetIntegrationInput{ID: rs.Primary.ID})
		if err != nil {
			return fmt.Errorf("error reading Integration %q: %w", rs.Primary.ID, err)
		}

		if got := fastly.ToValue(i.Name); got != expectedName {
			return fmt.Errorf("unexpected remote Integration name: got %q, want %q", got, expectedName)
		}
		if got := fastly.ToValue(i.Description); got != expectedDescription {
			return fmt.Errorf("unexpected remote Integration description: got %q, want %q", got, expectedDescription)
		}
		if got := fastly.ToValue(i.Type); got != expectedType {
			return fmt.Errorf("unexpected remote Integration type: got %q, want %q", got, expectedType)
		}

		return nil
	}
}
