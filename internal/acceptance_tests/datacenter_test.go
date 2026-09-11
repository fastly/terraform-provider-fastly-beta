package acceptancetests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccFastlyDataSourceDatacenters(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `data "fastly_datacenters" "example" {}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_datacenters.example", "id"),
					CheckDatacentersDataSourceNonEmpty("data.fastly_datacenters.example"),
				),
			},
			{
				Config:   `data "fastly_datacenters" "example" {}`,
				PlanOnly: true,
			},
		},
	})
}

func CheckDatacentersDataSourceNonEmpty(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		// pops is a set because the API does not guarantee POP order.
		for key, value := range rs.Primary.Attributes {
			if strings.HasPrefix(key, "pops.") && strings.HasSuffix(key, ".code") && value != "" {
				return nil
			}
		}

		return fmt.Errorf("expected data source to contain at least one POP with a code, got %v", rs.Primary.Attributes)
	}
}
