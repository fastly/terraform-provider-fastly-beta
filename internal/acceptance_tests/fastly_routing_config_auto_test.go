package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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
			{
				Config: ConfigFastlyRoutingConfigOrdering(serviceName, routingConfigName, "X-A", "X-B"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.path", "/z"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.0.condition.0.key", "X-A"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.1.condition.0.key", "X-B"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.2.is_default", "true"),
				),
			},
			{
				// Rules are evaluated in order, so swapping them must be applied and converge.
				Config: ConfigFastlyRoutingConfigOrdering(serviceName, routingConfigName, "X-B", "X-A"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.0.condition.0.key", "X-B"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.1.condition.0.key", "X-A"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.2.is_default", "true"),
				),
			},
			{
				// Config order is /z then /a; ImportStateVerify can't compare that
				// positionally, so assert import sorts paths instead.
				ResourceName:     "fastly_routing_config_auto.test",
				ImportState:      true,
				ImportStateCheck: checkImportedRoutingConfigPathsSorted,
			},
			{
				// Shrinking exercises path and rule deletion in Update.
				Config: ConfigFastlyRoutingConfig(serviceName, routingConfigName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.#", "1"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.path", "/"),
					resource.TestCheckResourceAttr("fastly_routing_config_auto.test", "path.0.rule.#", "1"),
				),
			},
		},
	})
}

func checkImportedRoutingConfigPathsSorted(states []*terraform.InstanceState) error {
	if len(states) != 1 {
		return fmt.Errorf("expected 1 imported instance, got %d", len(states))
	}
	attrs := states[0].Attributes
	if first, second := attrs["path.0.path"], attrs["path.1.path"]; first != "/a" || second != "/z" {
		return fmt.Errorf("expected imported paths sorted as /a, /z; got %q, %q", first, second)
	}
	return nil
}

func TestAccFastlyRoutingConfigAuto_invalidConfig(t *testing.T) {
	t.Parallel()

	rule := func(conditions string) string {
		return fmt.Sprintf(`
    rule {
      action_type  = "service"
      action_value = "svc"
%s    }
`, conditions)
	}
	condition := func(key string) string {
		keyLine := ""
		if key != "" {
			keyLine = fmt.Sprintf("        key      = %q\n", key)
		}
		return fmt.Sprintf(`
      condition {
        type     = "header"
%s        operator = "equals"
        value    = "true"
      }
`, keyLine)
	}
	config := func(name, paths string) string {
		return fmt.Sprintf("resource \"fastly_routing_config_auto\" \"test\" {\n  name = %q\n%s}\n", name, paths)
	}
	path := func(p, rules string) string {
		return fmt.Sprintf("  path {\n    path = %q\n%s  }\n", p, rules)
	}

	sixConditions := ""
	for i := 0; i < 6; i++ {
		sixConditions += condition(fmt.Sprintf("X-%d", i))
	}

	tests := map[string]struct {
		config string
		expect string
	}{
		"no path":             {config("rc", ""), `Missing Routing Config Path`},
		"no rule":             {config("rc", path("/", "")), `Missing Routing Config Rule`},
		"no default rule":     {config("rc", path("/", rule(condition("X-A")))), `Missing Default Routing Config Rule`},
		"header without key":  {config("rc", path("/", rule(condition(""))+rule(""))), `Missing Condition Key`},
		"too many conditions": {config("rc", path("/", rule(sixConditions)+rule(""))), `at most 5`},
		"path without slash":  {config("rc", path("nope", rule(""))), `must start with`},
		"name too long":       {config(strings.Repeat("a", 256), path("/", rule(""))), `at most 255`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { PreCheck(t) },
				ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
				Steps: []resource.TestStep{{
					Config:      tc.config,
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(tc.expect),
				}},
			})
		})
	}
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
