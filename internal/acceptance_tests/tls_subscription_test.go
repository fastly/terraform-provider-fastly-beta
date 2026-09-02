package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyTLSSubscription_basic(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain1 := testTLSActivationDomain(t)
	domain2 := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	resourceName := "fastly_tls_subscription.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSSubscriptionDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSSubscription(serviceName, domain1, domain2, backendName, domain1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "certificate_authority", "lets-encrypt"),
					resource.TestCheckResourceAttr(resourceName, "common_name", domain1),
					resource.TestCheckResourceAttr(resourceName, "domains.#", "2"),
					resource.TestCheckResourceAttrSet(resourceName, "configuration_id"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					resource.TestCheckResourceAttrSet(resourceName, "updated_at"),
					resource.TestCheckResourceAttrSet(resourceName, "state"),
					testAccCheckTLSSubscriptionExists(),
				),
			},
			{
				// common_name change only: subscription is expected to stay "pending", so this
				// is an in-place update, not a replace.
				Config: ConfigTLSSubscription(serviceName, domain1, domain2, backendName, domain2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "common_name", domain2),
					testAccCheckTLSSubscriptionExists(),
				),
			},
			{
				// Changing only configuration_id must reach the API - if not, this step's
				// automatic post-apply plan check fails because the diff reappears on refresh.
				Config: ConfigTLSSubscriptionWithConfigurationID(serviceName, domain1, domain2, backendName, domain2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "configuration_id"),
					testAccCheckTLSSubscriptionExists(),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force_destroy", "force_update"},
			},
		},
	})
}

func TestAccFastlyTLSSubscription_invalidCommonName(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSActivationDomain(t)
	other := testTLSActivationDomain(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fastly_tls_subscription" "test" {
  domains               = [%q]
  common_name           = %q
  certificate_authority = "lets-encrypt"
}
`, domain, other),
				// Terraform's diagnostic renderer may word-wrap the message, so tolerate a
				// newline anywhere whitespace would otherwise be.
				ExpectError: regexp.MustCompile(`(?s)domain specified as common_name .* must also be\s+in domains`),
			},
		},
	})
}

func TestAccFastlyTLSSubscription_uppercaseDomain(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := fmt.Sprintf("%s.FASTLY-EXAMPLE.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "fastly_tls_subscription" "test" {
  domains               = [%q]
  certificate_authority = "lets-encrypt"
}
`, domain),
				ExpectError: regexp.MustCompile("must not contain uppercase letters"),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSSubscription(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain1 := testTLSActivationDomain(t)
	domain2 := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	config := ConfigTLSSubscription(serviceName, domain1, domain2, backendName, domain1) + `
data "fastly_tls_subscription" "by_id" {
  id         = fastly_tls_subscription.test.id
  depends_on = [fastly_tls_subscription.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSSubscriptionDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.fastly_tls_subscription.by_id", "id", "fastly_tls_subscription.test", "id"),
					resource.TestCheckResourceAttr("data.fastly_tls_subscription.by_id", "certificate_authority", "lets-encrypt"),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSSubscriptionIDs(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain1 := testTLSActivationDomain(t)
	domain2 := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	config := ConfigTLSSubscription(serviceName, domain1, domain2, backendName, domain1) + `
data "fastly_tls_subscription_ids" "all" {
  depends_on = [fastly_tls_subscription.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSSubscriptionDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_tls_subscription_ids.all", "ids.#"),
				),
			},
		},
	})
}

func testAccCheckTLSSubscriptionExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		rs, ok := s.RootModule().Resources["fastly_tls_subscription.test"]
		if !ok {
			return fmt.Errorf("not found: fastly_tls_subscription.test")
		}

		_, err = client.GetTLSSubscription(context.Background(), &fastly.GetTLSSubscriptionInput{ID: rs.Primary.ID})
		return err
	}
}

func CheckTLSSubscriptionDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_tls_subscription" {
			continue
		}

		_, err := client.GetTLSSubscription(context.Background(), &fastly.GetTLSSubscriptionInput{ID: rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if TLS subscription was destroyed: %w", err)
		}

		return fmt.Errorf("TLS subscription %s still exists", rs.Primary.ID)
	}

	return nil
}
