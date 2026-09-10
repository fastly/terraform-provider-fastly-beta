package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFastlyAPISecurityDiscoveredOperations_basic only validates that the provider can
// call the discovered-operations endpoint and read its pagination metadata. Discovered
// operations depend on observed traffic, so the list is typically empty in this test.
func TestAccFastlyAPISecurityDiscoveredOperations_basic(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf_test_apisec_discovered_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigAPISecurityDiscoveredOperationsDataSource(serviceName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_api_security_discovered_operations.example", "id"),
					resource.TestCheckResourceAttrSet("data.fastly_api_security_discovered_operations.example", "total"),
					resource.TestCheckResourceAttrSet("data.fastly_api_security_discovered_operations.example", "operations.#"),
				),
			},
		},
	})
}
