package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyNGWAFSignalRule_lifecycle(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-acct-sig-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFRule("ngwaf_signal_rule.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_signal_rule.test", "description", "Exclude a false-positive signal account-wide"),
					resource.TestCheckResourceAttr("fastly_ngwaf_signal_rule.test", "applies_to.#", "1"),
					resource.TestCheckResourceAttr("fastly_ngwaf_signal_rule.test", "condition.0.field", "path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_signal_rule.test", "condition.0.operator", "like"),
					resource.TestCheckResourceAttr("fastly_ngwaf_signal_rule.test", "action.0.signal", "XSS"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_signal_rule.test", "id"),
				),
			},
			{
				ResourceName:      "fastly_ngwaf_signal_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
