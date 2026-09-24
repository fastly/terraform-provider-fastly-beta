package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyDataSourceDictionaries_Config(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dictionaries-ds-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	dictionaryName1 := fmt.Sprintf("dict_1_%s", acctest.RandString(10))
	dictionaryName2 := fmt.Sprintf("dict_2_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDataSourceDictionaries(serviceName, domainName, dictionaryName1, dictionaryName2),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("data.fastly_dictionaries.example", "dictionaries.#", "2"),
					resource.TestCheckResourceAttrSet("data.fastly_dictionaries.example", "id"),
					resource.TestCheckTypeSetElemNestedAttrs("data.fastly_dictionaries.example", "dictionaries.*", map[string]string{
						"name":       dictionaryName1,
						"write_only": "false",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("data.fastly_dictionaries.example", "dictionaries.*", map[string]string{
						"name":       dictionaryName2,
						"write_only": "true",
					}),
				),
			},
		},
	})
}
