package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

// ngwafAccountRuleTypes are the resource types CheckNGWAFRuleDestroy looks
// for; every account rule resource reaches the same rules endpoint.
var ngwafAccountRuleTypes = []string{
	"fastly_ngwaf_request_rule",
	"fastly_ngwaf_signal_rule",
}

func TestAccFastlyNGWAFRules_dataSource(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-acct-rules-ds-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFRule("ngwaf_rules_data_source.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_rules.test", "id"),
					// The account endpoint reports every rule in the account,
					// so the rule under test is located by ID rather than by
					// asserting an exact count.
					checkNGWAFRulesDataSourceHasRule("data.fastly_ngwaf_rules.test", "fastly_ngwaf_request_rule.test"),
				),
			},
		},
	})
}

// checkNGWAFRulesDataSourceHasRule asserts the data source reported the given
// rule, with the type the resource created it under. It locates the rule by ID
// rather than asserting a count: the account list is shared, so other rules in
// the account are expected.
func checkNGWAFRulesDataSourceHasRule(dataSourceName, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rule, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("data source not found: %s", dataSourceName)
		}

		count, err := strconv.Atoi(ds.Primary.Attributes["rules.#"])
		if err != nil {
			return fmt.Errorf("reading %s rules.#: %w", dataSourceName, err)
		}

		for i := range count {
			if ds.Primary.Attributes[fmt.Sprintf("rules.%d.id", i)] != rule.Primary.ID {
				continue
			}
			if got := ds.Primary.Attributes[fmt.Sprintf("rules.%d.type", i)]; got != "request" {
				return fmt.Errorf("rule %s reported type %q, want %q", rule.Primary.ID, got, "request")
			}
			return nil
		}

		return fmt.Errorf("rule %s not reported by %s (%d rules listed)", rule.Primary.ID, dataSourceName, count)
	}
}

func ConfigNGWAFRule(blockFile, name string) string {
	raw, err := os.ReadFile("blocks/" + blockFile)
	if err != nil {
		panic(err)
	}
	return strings.ReplaceAll(string(raw), "{{.WORKSPACE_NAME}}", name)
}

func CheckNGWAFRuleDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if !slices.Contains(ngwafAccountRuleTypes, rs.Type) {
			continue
		}

		ruleID := rs.Primary.ID

		_, err := rules.Get(context.Background(), client, &rules.GetInput{
			RuleID: &ruleID,
			Scope:  &scope.Scope{Type: scope.ScopeTypeAccount},
		})
		if err == nil {
			return fmt.Errorf("NGWAF account rule %s still exists after destroy", ruleID)
		}
	}
	return nil
}
