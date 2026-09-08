package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyTLSPrivateKey_basic(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, _ := generateTLSKeyAndCert(t, "tf-test.fastly-example.com")

	resourceName := "fastly_tls_private_key.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSPrivateKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSPrivateKey(name, keyPEM),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "key_type", "RSA"),
					resource.TestCheckResourceAttrSet(resourceName, "key_length"),
					resource.TestCheckResourceAttrSet(resourceName, "public_key_sha1"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					testAccCheckTLSPrivateKeyExists(),
				),
			},
			{
				// key_pem is never returned by the API, so it cannot be verified on import.
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"private_key"},
			},
		},
	})
}

func TestAccFastlyTLSPrivateKey_replace(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	name1 := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	name2 := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM1, _ := generateTLSKeyAndCert(t, "tf-test.fastly-example.com")
	keyPEM2, _ := generateTLSKeyAndCert(t, "tf-test.fastly-example.com")

	resourceName := "fastly_tls_private_key.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSPrivateKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSPrivateKey(name1, keyPEM1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", name1),
					testAccCheckTLSPrivateKeyExists(),
				),
			},
			{
				// name and private_key both require replacement: no in-place update path exists.
				Config: ConfigTLSPrivateKey(name2, keyPEM2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", name2),
					testAccCheckTLSPrivateKeyExists(),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSPrivateKey(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, _ := generateTLSKeyAndCert(t, "tf-test.fastly-example.com")

	config := ConfigTLSPrivateKey(name, keyPEM) + `
data "fastly_tls_private_key" "by_name" {
  name       = fastly_tls_private_key.test.name
  depends_on = [fastly_tls_private_key.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSPrivateKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.fastly_tls_private_key.by_name", "name", name),
					resource.TestCheckResourceAttrPair("data.fastly_tls_private_key.by_name", "id", "fastly_tls_private_key.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSPrivateKeyIds(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, _ := generateTLSKeyAndCert(t, "tf-test.fastly-example.com")

	config := ConfigTLSPrivateKey(name, keyPEM) + `
data "fastly_tls_private_key_ids" "all" {
  depends_on = [fastly_tls_private_key.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSPrivateKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_tls_private_key_ids.all", "ids.#"),
				),
			},
		},
	})
}

func testAccCheckTLSPrivateKeyExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		rs, ok := s.RootModule().Resources["fastly_tls_private_key.test"]
		if !ok {
			return fmt.Errorf("not found: fastly_tls_private_key.test")
		}

		_, err = client.GetPrivateKey(context.Background(), &fastly.GetPrivateKeyInput{ID: rs.Primary.ID})
		return err
	}
}

func CheckTLSPrivateKeyDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_tls_private_key" {
			continue
		}

		_, err := client.GetPrivateKey(context.Background(), &fastly.GetPrivateKeyInput{ID: rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if TLS private key was destroyed: %w", err)
		}

		return fmt.Errorf("TLS private key %s still exists", rs.Primary.ID)
	}

	return nil
}
