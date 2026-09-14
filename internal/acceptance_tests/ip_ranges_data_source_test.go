package acceptancetests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyDataSourceIPRanges(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `data "fastly_ip_ranges" "example" {}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_ip_ranges.example", "id"),
					resource.TestCheckResourceAttrSet("data.fastly_ip_ranges.example", "cidr_blocks.0"),
					resource.TestCheckResourceAttrSet("data.fastly_ip_ranges.example", "ipv6_cidr_blocks.0"),
				),
			},
		},
	})
}
