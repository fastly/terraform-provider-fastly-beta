package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyAPISecurityOperationTagsDataSource_basic(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf_test_apisec_tags_ds_%s", acctest.RandString(10))
	tagName := fmt.Sprintf("tf-test-tag-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAPISecurityOperationTagAndServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigAPISecurityOperationTagsDataSource(serviceName, tagName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_api_security_operation_tags.example", "id"),
					resource.TestCheckResourceAttrSet("data.fastly_api_security_operation_tags.example", "total"),
					resource.TestCheckResourceAttr("data.fastly_api_security_operation_tags.example", "tags.#", "1"),
					resource.TestCheckResourceAttr("data.fastly_api_security_operation_tags.example", "tags.0.name", tagName),
					resource.TestCheckResourceAttrPair(
						"data.fastly_api_security_operation_tags.example", "tags.0.id",
						"fastly_api_security_operation_tag.example", "tag_id",
					),
				),
			},
		},
	})
}
