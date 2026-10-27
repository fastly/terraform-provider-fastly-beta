package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/fastly/go-fastly/v18/fastly/domainmanagement/v1/domains"
	"github.com/fastly/go-fastly/v18/fastly/domainmanagement/v1/routingconfigs"
)

func testRoutingConfigName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("tf-test-%s", acctest.RandString(10))
}

func TestAccFastlyRoutingConfigAuto_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	routingConfigName := testRoutingConfigName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckFastlyRoutingConfigDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigFastlyRoutingConfig(serviceName, routingConfigName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "name", routingConfigName),
					resource.TestCheckResourceAttrSet("fastly_routing_config_auto.test", "id"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.#", "1"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.path", "/"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.#", "1"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.0.is_default", "true"),
					resource.TestCheckResourceAttrPair("fastly_routing_config_auto.test", "path.0.rule.0.action_value", "fastly_service_cdn.test", "id"),
				),
			},
			{
				Config: ConfigFastlyRoutingConfigUpdated(serviceName, routingConfigName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.#", "2"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.#", "2"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.0.condition.#", "1"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.0.condition.0.key", "X-Beta"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.1.path", "/api/*"),
				),
			},
			{
				ResourceName:      "fastly_routing_config_auto.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyRoutingConfigAuto_defaultFirstRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	routingConfigName := testRoutingConfigName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckFastlyRoutingConfigDestroy,
		Steps: []resource.TestStep{
			{
				Config:      ConfigFastlyRoutingConfigDefaultFirst(serviceName, routingConfigName),
				ExpectError: regexp.MustCompile(`Default Routing Config Rule Must Be Last`),
			},
		},
	})
}

func TestAccFastlyRoutingConfigDomainLink_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	routingConfigName := testRoutingConfigName(t)
	fqdn := testDomainFQDN(t)

	var domainID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckFastlyRoutingConfigDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigFastlyRoutingConfigWithDomainLink(serviceName, routingConfigName, fqdn),
				Check: resource.ComposeTestCheckFunc(
					captureResourceID("fastly_domain.test", &domainID),
					resource.TestCheckResourceAttrSet("fastly_routing_config_domain_link.test", "domain_id"),
					resource.TestCheckResourceAttrPair("fastly_routing_config_domain_link.test", "routing_config_id", "fastly_routing_config_auto.test", "id"),
				),
			},
			{
				// Drop only the link resource, keeping the domain alive, so we can
				// confirm Delete actually clears the domain's association.
				Config: ConfigFastlyRoutingConfigWithUnlinkedDomain(serviceName, routingConfigName, fqdn),
				Check: resource.ComposeTestCheckFunc(
					checkFastlyDomainRoutingConfigUnlinked(&domainID),
				),
			},
			{
				Config: ConfigFastlyRoutingConfigWithDomainLink(serviceName, routingConfigName, fqdn),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_routing_config_domain_link.test", "domain_id"),
				),
			},
			{
				ResourceName:      "fastly_routing_config_domain_link.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// checkFastlyDomainRoutingConfigUnlinked checks via the API, not state: the
// domain resource's schema doesn't expose this field.
func checkFastlyDomainRoutingConfigUnlinked(domainID *string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		data, err := domains.Get(context.Background(), client, &domains.GetInput{DomainID: domainID})
		if err != nil {
			return fmt.Errorf("error reading Domain %s: %w", *domainID, err)
		}
		if data.RoutingConfigurationID != nil {
			return fmt.Errorf("Domain %s still has routing_configuration_id %q set after unlinking", *domainID, *data.RoutingConfigurationID)
		}
		return nil
	}
}

func CheckFastlyRoutingConfigDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_routing_config_auto" {
			continue
		}

		id := rs.Primary.ID
		_, err := routingconfigs.Get(context.Background(), client, &routingconfigs.GetInput{RoutingConfigID: &id})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Routing Config was destroyed: %w", err)
		}

		return fmt.Errorf("Routing Config %s still exists", id)
	}

	return nil
}
