package acceptancetests

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyDataSourcePackageHash_filename(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "fastly_package_hash" "example" {
  filename = %q
}
`, GetPackagePath()),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_package_hash.example", "hash"),
					resource.TestCheckResourceAttrPair(
						"data.fastly_package_hash.example", "id",
						"data.fastly_package_hash.example", "hash",
					),
				),
			},
		},
	})
}

// TestAccFastlyDataSourcePackageHash_content proves the content and filename inputs are
// equivalent: hashing the same package via a base64-encoded content string must produce
// the same hash as hashing it by filename.
func TestAccFastlyDataSourcePackageHash_content(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(GetPackagePath())
	if err != nil {
		t.Fatalf("reading package fixture: %s", err)
	}
	content := base64.StdEncoding.EncodeToString(data)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "fastly_package_hash" "by_filename" {
  filename = %q
}

data "fastly_package_hash" "by_content" {
  content = %q
}
`, GetPackagePath(), content),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.fastly_package_hash.by_content", "hash",
						"data.fastly_package_hash.by_filename", "hash",
					),
				),
			},
		},
	})
}
