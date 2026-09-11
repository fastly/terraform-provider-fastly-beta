package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyDataSourceStagingIPs_Config(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-staging-ips-ds-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDataSourceStagingIPs(serviceName, domainName, backendName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("data.fastly_staging_ips.example", "domains.#", "1"),
					resource.TestCheckResourceAttrSet("data.fastly_staging_ips.example", "id"),
					resource.TestCheckTypeSetElemNestedAttrs("data.fastly_staging_ips.example", "domains.*", map[string]string{
						"name": domainName,
					}),
				),
			},
		},
	})
}
