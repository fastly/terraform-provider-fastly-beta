package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyNGWAFWorkspaceSignalRule_lifecycle(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-sig-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_signal_rule.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_signal_rule.test", "description", "Exclude a false-positive signal"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_signal_rule.test", "condition.0.field", "path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_signal_rule.test", "condition.0.operator", "like"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_signal_rule.test", "action.0.signal", "XSS"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_signal_rule.test", "id"),
				),
			},
			{
				ResourceName:      "fastly_ngwaf_workspace_signal_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDFunc("fastly_ngwaf_workspace_signal_rule.test"),
			},
		},
	})
}
