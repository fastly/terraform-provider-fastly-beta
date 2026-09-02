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

func TestAccFastlyTLSMutualAuthentication_basic(t *testing.T) {
	t.Parallel()

	domain := testTLSActivationDomain(t)
	_, certBundle1 := generateTLSKeyAndCert(t, domain)
	_, certBundle2 := generateTLSKeyAndCert(t, domain)
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	resourceName := "fastly_tls_mutual_authentication.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSMutualAuthenticationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSMutualAuthentication(certBundle1, "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "enforced", "false"),
					resource.TestCheckResourceAttrSet(resourceName, "name"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					resource.TestCheckResourceAttrSet(resourceName, "updated_at"),
					testAccCheckTLSMutualAuthenticationExists(),
				),
			},
			{
				Config: ConfigTLSMutualAuthentication(certBundle2, "true", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "enforced", "true"),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					testAccCheckTLSMutualAuthenticationExists(),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"cert_bundle"}, // write-only: never returned by the API
			},
		},
	})
}

// TestAccFastlyTLSMutualAuthentication_withActivation links mTLS to a real activation, then
// unlinks it. Checks hit the API directly since the cross-resource side effect isn't visible
// to Terraform's own state within the same apply.
func TestAccFastlyTLSMutualAuthentication_withActivation(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	certificateID := createOutOfBandCertificate(t, client, domain)
	_, certBundle := generateTLSKeyAndCert(t, domain)

	activationResourceName := "fastly_tls_activation.test"
	mtlsResourceName := "fastly_tls_mutual_authentication.test"
	base := ConfigTLSActivation(serviceName, domain, backendName, certificateID)

	withActivation := base + fmt.Sprintf(`
resource "fastly_tls_mutual_authentication" "test" {
  cert_bundle    = <<-EOT
%s
EOT
  activation_ids = [fastly_tls_activation.test.id]
}
`, certBundle)

	withoutActivation := base + fmt.Sprintf(`
resource "fastly_tls_mutual_authentication" "test" {
  cert_bundle = <<-EOT
%s
EOT
}
`, certBundle)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSMutualAuthenticationDestroy,
		Steps: []resource.TestStep{
			{
				Config: withActivation,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(mtlsResourceName, "tls_activations.#", "1"),
					resource.TestCheckResourceAttrPair(mtlsResourceName, "tls_activations.0", activationResourceName, "id"),
					testAccCheckActivationMTLSLinked(activationResourceName, mtlsResourceName),
				),
			},
			{
				Config: withoutActivation,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(mtlsResourceName, "tls_activations.#", "0"),
					testAccCheckActivationMTLSUnlinked(activationResourceName),
				),
			},
		},
	})
}

func testAccCheckActivationMTLSLinked(activationResourceName, mtlsResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		activationRS, ok := s.RootModule().Resources[activationResourceName]
		if !ok {
			return fmt.Errorf("not found: %s", activationResourceName)
		}
		mtlsRS, ok := s.RootModule().Resources[mtlsResourceName]
		if !ok {
			return fmt.Errorf("not found: %s", mtlsResourceName)
		}

		activation, err := client.GetTLSActivation(context.Background(), &fastly.GetTLSActivationInput{ID: activationRS.Primary.ID})
		if err != nil {
			return err
		}
		if activation.MutualAuthentication == nil || activation.MutualAuthentication.ID != mtlsRS.Primary.ID {
			return fmt.Errorf("expected activation %s to have mutual_authentication_id %s, got %+v",
				activationRS.Primary.ID, mtlsRS.Primary.ID, activation.MutualAuthentication)
		}
		return nil
	}
}

func testAccCheckActivationMTLSUnlinked(activationResourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		activationRS, ok := s.RootModule().Resources[activationResourceName]
		if !ok {
			return fmt.Errorf("not found: %s", activationResourceName)
		}

		activation, err := client.GetTLSActivation(context.Background(), &fastly.GetTLSActivationInput{ID: activationRS.Primary.ID})
		if err != nil {
			return err
		}
		if activation.MutualAuthentication != nil {
			return fmt.Errorf("expected activation %s to have no mutual authentication, got %+v",
				activationRS.Primary.ID, activation.MutualAuthentication)
		}
		return nil
	}
}

func testAccCheckTLSMutualAuthenticationExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		rs, ok := s.RootModule().Resources["fastly_tls_mutual_authentication.test"]
		if !ok {
			return fmt.Errorf("not found: fastly_tls_mutual_authentication.test")
		}

		_, err = client.GetTLSMutualAuthentication(context.Background(), &fastly.GetTLSMutualAuthenticationInput{ID: rs.Primary.ID})
		return err
	}
}

func CheckTLSMutualAuthenticationDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_tls_mutual_authentication" {
			continue
		}

		_, err := client.GetTLSMutualAuthentication(context.Background(), &fastly.GetTLSMutualAuthenticationInput{ID: rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if TLS mutual authentication was destroyed: %w", err)
		}

		return fmt.Errorf("TLS mutual authentication %s still exists", rs.Primary.ID)
	}

	return nil
}
