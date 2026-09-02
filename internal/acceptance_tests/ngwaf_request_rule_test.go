package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccFastlyNGWAFRequestRule_lifecycle(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-acct-req-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFRule("ngwaf_request_rule_basic.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "description", "Block a specific IP account-wide"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "enabled", "true"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "applies_to.#", "1"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "condition.0.field", "ip"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "condition.0.operator", "equals"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "condition.0.value", "127.0.0.1"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "action.0.type", "block"),
					// The account endpoint rejects custom responses, so the
					// schema exposes no redirect_url/response_code here.
					resource.TestCheckNoResourceAttr("fastly_ngwaf_request_rule.test", "action.0.redirect_url"),
					resource.TestCheckNoResourceAttr("fastly_ngwaf_request_rule.test", "action.0.response_code"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "request_logging", "sampled"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_request_rule.test", "id"),
				),
			},
			{
				Config: ConfigNGWAFRule("ngwaf_request_rule_updated.tf", name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						// applies_to is a body field, so widening it must not
						// force replacement the way workspace_id does.
						plancheck.ExpectResourceAction("fastly_ngwaf_request_rule.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "description", "Allow a specific path account-wide"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "enabled", "false"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "group_operator", "any"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "applies_to.#", "2"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "condition.0.field", "path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "action.0.type", "allow"),
				),
			},
			{
				// Account rules import from a bare rule ID: their endpoint has
				// no workspace path segment to recover.
				ResourceName:      "fastly_ngwaf_request_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyNGWAFRequestRule_wildcard pins the applies_to = ["*"] round
// trip. The wildcard is the form the documentation leads with, and it is the
// one case where the API could legitimately answer with something other than
// what was sent - expanding it into concrete workspace IDs, say - which would
// surface as "inconsistent result after apply" rather than as drift.
func TestAccFastlyNGWAFRequestRule_wildcard(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-acct-req-wildcard-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFRule("ngwaf_request_rule_wildcard.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "applies_to.#", "1"),
					resource.TestCheckTypeSetElemAttr("fastly_ngwaf_request_rule.test", "applies_to.*", "*"),
					resource.TestCheckResourceAttr("fastly_ngwaf_request_rule.test", "enabled", "false"),
				),
			},
			{
				Config:   ConfigNGWAFRule("ngwaf_request_rule_wildcard.tf", name),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_ngwaf_request_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
