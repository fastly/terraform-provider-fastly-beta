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

func testTLSCertificateDomain(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s.fastly-example.com", acctest.RandString(10))
}

func TestAccFastlyTLSCertificate_withName(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSCertificateDomain(t)
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	updatedName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	key, keyPEM := generateTLSKey(t)
	certPEM := generateTLSCertForKey(t, key, domain)
	certPEM2 := generateTLSCertForKey(t, key, domain)
	privateKey := ConfigTLSPrivateKey(keyName, keyPEM)

	resourceName := "fastly_tls_certificate.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: joinBlocks(privateKey, ConfigTLSCertificateWithPrivateKey(certPEM, name)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					resource.TestCheckResourceAttrSet(resourceName, "updated_at"),
					resource.TestCheckResourceAttr(resourceName, "issued_to", domain),
					resource.TestCheckResourceAttrSet(resourceName, "issuer"),
					resource.TestCheckResourceAttrSet(resourceName, "serial_number"),
					resource.TestCheckResourceAttrSet(resourceName, "signature_algorithm"),
					resource.TestCheckResourceAttr(resourceName, "domains.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "domains.0", domain),
					testAccCheckTLSCertificateExists(),
				),
			},
			{
				// name and certificate_body have no RequiresReplace, so this updates in place.
				// certPEM2 is signed by the same key as certPEM (privateKey is unchanged across
				// steps), so there's no private-key replacement to order around.
				Config: joinBlocks(privateKey, ConfigTLSCertificateWithPrivateKey(certPEM2, updatedName)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", updatedName),
					testAccCheckTLSCertificateExists(),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"certificate_body"},
			},
		},
	})
}

func TestAccFastlyTLSCertificate_withoutName(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSCertificateDomain(t)
	keyName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	privateKey := ConfigTLSPrivateKey(keyName, keyPEM)

	resourceName := "fastly_tls_certificate.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: joinBlocks(privateKey, ConfigTLSCertificateWithPrivateKey(certPEM, "")),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", domain),
					resource.TestCheckResourceAttr(resourceName, "issued_to", domain),
					testAccCheckTLSCertificateExists(),
				),
			},
		},
	})
}

func TestAccFastlyTLSCertificate_invalidCertificateBody(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigTLSCertificate("not a certificate", "test"),
				ExpectError: regexp.MustCompile("expected certificate_body to be valid PEM-format blocks"),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSCertificate(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSCertificateDomain(t)
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	privateKey := ConfigTLSPrivateKey(keyName, keyPEM)

	config := joinBlocks(privateKey, ConfigTLSCertificateWithPrivateKey(certPEM, name)) + `
data "fastly_tls_certificate" "by_name" {
  name       = fastly_tls_certificate.test.name
  depends_on = [fastly_tls_certificate.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.fastly_tls_certificate.by_name", "issued_to", domain),
					resource.TestCheckResourceAttrPair("data.fastly_tls_certificate.by_name", "id", "fastly_tls_certificate.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSCertificateIDs(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSCertificateDomain(t)
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	privateKey := ConfigTLSPrivateKey(keyName, keyPEM)

	config := joinBlocks(privateKey, ConfigTLSCertificateWithPrivateKey(certPEM, name)) + `
data "fastly_tls_certificate_ids" "all" {
  depends_on = [fastly_tls_certificate.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.TestCheckTypeSetElemAttrPair(
					"data.fastly_tls_certificate_ids.all", "ids.*", "fastly_tls_certificate.test", "id",
				),
			},
		},
	})
}

func testAccCheckTLSCertificateExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		rs, ok := s.RootModule().Resources["fastly_tls_certificate.test"]
		if !ok {
			return fmt.Errorf("not found: fastly_tls_certificate.test")
		}

		_, err = client.GetCustomTLSCertificate(context.Background(), &fastly.GetCustomTLSCertificateInput{ID: rs.Primary.ID})
		return err
	}
}

func CheckTLSCertificateDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_tls_certificate" {
			continue
		}

		_, err := client.GetCustomTLSCertificate(context.Background(), &fastly.GetCustomTLSCertificateInput{ID: rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if TLS certificate was destroyed: %w", err)
		}

		return fmt.Errorf("TLS certificate %s still exists", rs.Primary.ID)
	}

	return nil
}
