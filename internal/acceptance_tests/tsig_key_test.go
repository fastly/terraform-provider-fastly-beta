package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/fastly/go-fastly/v17/fastly/dns/v1/tsigkeys"
)

func testTSIGKeyName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("tf-test-%s", acctest.RandString(10))
}

func TestAccFastlyTSIGKey_basic(t *testing.T) {
	t.Parallel()
	name := testTSIGKeyName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTSIGKey(name, "hmac-sha256", "dGVzdHNlY3JldA=="),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_tsig_key.test", "name", name),
					resource.TestCheckResourceAttr("fastly_tsig_key.test", "algorithm", "hmac-sha256"),
					resource.TestCheckResourceAttr("fastly_tsig_key.test", "secret.value", "dGVzdHNlY3JldA=="),
					resource.TestCheckResourceAttrSet("fastly_tsig_key.test", "id"),
				),
			},
			{
				Config: ConfigTSIGKeyWithDescription(name, "hmac-sha256", "updated description", "dGVzdHNlY3JldA=="),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_tsig_key.test", "description", "updated description"),
				),
			},
			{
				ResourceName:            "fastly_tsig_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

func TestAccFastlyTSIGKey_algorithms(t *testing.T) {
	t.Parallel()

	for _, algorithm := range []string{"hmac-sha224", "hmac-sha256", "hmac-sha384", "hmac-sha512"} {
		t.Run(algorithm, func(t *testing.T) {
			t.Parallel()
			name := testTSIGKeyName(t)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { PreCheck(t) },
				ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
				CheckDestroy:             CheckTSIGKeyDestroy,
				Steps: []resource.TestStep{
					{
						Config: ConfigTSIGKey(name, algorithm, "dGVzdHNlY3JldA=="),
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr("fastly_tsig_key.test", "algorithm", algorithm),
						),
					},
				},
			})
		})
	}
}

func TestAccFastlyTSIGKey_invalidName(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigTSIGKey("has space", "hmac-sha256", "dGVzdHNlY3JldA=="),
				ExpectError: regexp.MustCompile("must not contain spaces"),
			},
		},
	})
}

func TestAccFastlyTSIGKey_invalidSecret(t *testing.T) {
	t.Parallel()
	name := testTSIGKeyName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigTSIGKey(name, "hmac-sha256", "not base64!!"),
				ExpectError: regexp.MustCompile("must be valid Base64"),
			},
		},
	})
}

func TestAccFastlyDataSourceTSIGKeys(t *testing.T) {
	t.Parallel()
	h := acctest.RandString(10)
	name1 := fmt.Sprintf("tf-%s-1", h)
	name2 := fmt.Sprintf("tf-%s-2", h)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: RenderBlock("internal/acceptance_tests/blocks/tsig_keys_two_with_datasource.tf", map[string]string{
					"NAME_1": name1,
					"NAME_2": name2,
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_tsig_keys.example", "total"),
				),
			},
		},
	})
}

func CheckTSIGKeyDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_tsig_key" {
			continue
		}

		id := rs.Primary.ID
		_, err := tsigkeys.Get(context.Background(), client, &tsigkeys.GetInput{TSIGKeyID: &id})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if TSIG Key was destroyed: %w", err)
		}

		return fmt.Errorf("TSIG Key %s still exists", id)
	}

	return nil
}
