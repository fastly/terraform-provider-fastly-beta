package acceptancetests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccFastlyDataSourceServices_basic(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-services-ds-%s", acctest.RandString(10))

	config := BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
		},
	) + `
data "fastly_services" "example" {
  depends_on = [fastly_service_cdn.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_services.example", "id"),
					CheckServicesDataSourceContains("data.fastly_services.example", "fastly_service_cdn.test"),
				),
			},
		},
	})
}

// CheckServicesDataSourceContains scans the fastly_services data source's ids and
// details.*.id attributes (both sets, so indices aren't stable) for the ID of resourceName.
func CheckServicesDataSourceContains(dataSourceName, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		service, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}
		serviceID := service.Primary.ID

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("not found: %s", dataSourceName)
		}

		for key, value := range ds.Primary.Attributes {
			if strings.HasPrefix(key, "ids.") && value == serviceID {
				return nil
			}
		}

		return fmt.Errorf("expected %s.ids to contain %s, got %v", dataSourceName, serviceID, ds.Primary.Attributes)
	}
}
