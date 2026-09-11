package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyAPISecurityOperationsDataSource_basic(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf_test_apisec_ops_ds_%s", acctest.RandString(10))
	domainName := fmt.Sprintf("tf-test-%s.example.com", acctest.RandString(10))
	path := "/v1/things"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAPISecurityOperationAndServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigAPISecurityOperationsDataSource(serviceName, "GET", domainName, path),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_api_security_operations.example", "id"),
					resource.TestCheckResourceAttrSet("data.fastly_api_security_operations.example", "total"),
					resource.TestCheckResourceAttr("data.fastly_api_security_operations.example", "operations.#", "1"),
					resource.TestCheckResourceAttr("data.fastly_api_security_operations.example", "operations.0.method", "GET"),
					resource.TestCheckResourceAttr("data.fastly_api_security_operations.example", "operations.0.domain", domainName),
					resource.TestCheckResourceAttr("data.fastly_api_security_operations.example", "operations.0.path", path),
					resource.TestCheckResourceAttrPair(
						"data.fastly_api_security_operations.example", "operations.0.id",
						"fastly_api_security_operation.example", "operation_id",
					),
				),
			},
			{
				// method filters down to the single matching operation.
				Config: ConfigAPISecurityOperationsDataSource(serviceName, "GET", domainName, path) + `
data "fastly_api_security_operations" "filtered" {
  service_id = fastly_service_cdn.test.id
  method     = ["GET"]
  domain     = [` + fmt.Sprintf("%q", domainName) + `]
  depends_on = [fastly_api_security_operation.example]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.fastly_api_security_operations.filtered", "operations.#", "1"),
				),
			},
		},
	})
}
