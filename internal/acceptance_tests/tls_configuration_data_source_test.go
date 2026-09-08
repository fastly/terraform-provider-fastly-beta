package acceptancetests

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyDataSourceTLSConfiguration_basic(t *testing.T) {
	t.Parallel()

	resourceName := "data.fastly_tls_configuration.subject"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccFastlyDataSourceTLSConfigurationBasic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "name"),
					resource.TestCheckResourceAttrSet(resourceName, "tls_protocols.#"),
					resource.TestCheckResourceAttrSet(resourceName, "http_protocols.#"),
					resource.TestMatchResourceAttr(resourceName, "tls_service", regexp.MustCompile(`^(PLATFORM|CUSTOM)$`)),
					resource.TestCheckResourceAttrSet(resourceName, "default"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					resource.TestCheckResourceAttrSet(resourceName, "updated_at"),
					resource.TestCheckResourceAttrSet(resourceName, "dns_records.#"),
				),
			},
		},
	})
}

const testAccFastlyDataSourceTLSConfigurationBasic = `
data "fastly_tls_configuration" "subject" {
  default = true
}
`

func TestAccFastlyDataSourceTLSConfiguration_withIDLookup(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccFastlyDataSourceTLSConfigurationWithIDLookup,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.fastly_tls_configuration.subject", "name",
						"data.fastly_tls_configuration.default", "name",
					),
				),
			},
		},
	})
}

const testAccFastlyDataSourceTLSConfigurationWithIDLookup = `
data "fastly_tls_configuration" "default" {
  default = true
}
data "fastly_tls_configuration" "subject" {
  id = data.fastly_tls_configuration.default.id
}
`
