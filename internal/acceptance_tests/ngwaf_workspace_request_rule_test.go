package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyNGWAFWorkspaceRequestRule_lifecycle(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-req-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_request_rule_basic.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "description", "Block a specific IP"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "enabled", "true"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "condition.0.field", "ip"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "condition.0.operator", "equals"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "condition.0.value", "127.0.0.1"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "action.0.type", "block"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "action.0.redirect_url", "https://example.com/blocked"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "action.0.response_code", "302"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_request_rule.test", "id"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_request_rule.test", "workspace_id"),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_request_rule_updated.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "description", "Allow a specific path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "enabled", "false"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "group_operator", "any"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "request_logging", "sampled"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "condition.0.field", "path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "action.0.type", "allow"),
				),
			},
			{
				ResourceName:      "fastly_ngwaf_workspace_request_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDFunc("fastly_ngwaf_workspace_request_rule.test"),
			},
		},
	})
}

func TestAccFastlyNGWAFWorkspaceRequestRule_groupAndMultivalConditions(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-req-rule-group-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_request_rule_group_condition.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "group_condition.0.group_operator", "any"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "group_condition.0.condition.0.field", "ip"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "group_condition.0.multival_condition.0.field", "request_header"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "group_condition.0.multival_condition.0.condition.0.value", "X-Forwarded-For"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "multival_condition.0.field", "query_parameter"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_request_rule.test", "multival_condition.0.condition.0.value", "debug"),
				),
			},
		},
	})
}
