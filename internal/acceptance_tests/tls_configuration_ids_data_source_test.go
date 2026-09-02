package acceptancetests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyDataSourceTLSConfigurationIDs(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `data "fastly_tls_configuration_ids" "subject" {}`,
				Check:  resource.TestCheckResourceAttrSet("data.fastly_tls_configuration_ids.subject", "ids.#"),
			},
		},
	})
}
