package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

// ngwafWorkspaceRuleTypes are the resource types CheckNGWAFWorkspaceRuleDestroy
// looks for; every workspace rule resource reaches the same rules endpoint.
var ngwafWorkspaceRuleTypes = []string{
	"fastly_ngwaf_workspace_request_rule",
	"fastly_ngwaf_workspace_signal_rule",
	"fastly_ngwaf_workspace_rate_limit_rule",
	"fastly_ngwaf_workspace_templated_signal_rule",
}

func TestAccFastlyNGWAFWorkspaceRules_dataSource(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-rules-ds-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_rules_data_source.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_workspace_rules.test", "id"),
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_workspace_rules.test", "workspace_id"),
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_rules.test", "rules.#", "1"),
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_rules.test", "rules.0.type", "request"),
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_rules.test", "rules.0.description", "Block a specific IP"),
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_workspace_rules.test", "rules.0.id"),
				),
			},
		},
	})
}

func importStateIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}
		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["workspace_id"], rs.Primary.ID), nil
	}
}

func ConfigNGWAFWorkspaceRule(blockFile, name string) string {
	raw, err := os.ReadFile("blocks/" + blockFile)
	if err != nil {
		panic(err)
	}
	return strings.ReplaceAll(string(raw), "{{.WORKSPACE_NAME}}", name)
}

func CheckNGWAFWorkspaceRuleDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if !slices.Contains(ngwafWorkspaceRuleTypes, rs.Type) {
			continue
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		ruleID := rs.Primary.ID

		_, err := rules.Get(context.Background(), client, &rules.GetInput{
			RuleID: &ruleID,
			Scope: &scope.Scope{
				Type:      scope.ScopeTypeWorkspace,
				AppliesTo: []string{workspaceID},
			},
		})
		if err == nil {
			return fmt.Errorf("NGWAF workspace rule %s still exists after destroy", ruleID)
		}
	}
	return nil
}
