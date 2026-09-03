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

// uploadOutOfBandPrivateKey uploads keyPEM via the raw go-fastly client and registers cleanup.
// The Fastly API rejects creating a certificate until its matching private key is uploaded, and
// no fastly_tls_private_key resource exists yet.
//
// TODO(CDTOOL-1586): rework via Terraform config once fastly_tls_private_key exists.
func uploadOutOfBandPrivateKey(t *testing.T, client *fastly.Client, keyPEM string) {
	t.Helper()

	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	key, err := client.CreatePrivateKey(context.Background(), &fastly.CreatePrivateKeyInput{
		Key:  keyPEM,
		Name: name,
	})
	if err != nil {
		t.Fatalf("creating out-of-band private key: %s", err)
	}
	t.Cleanup(func() {
		if err := client.DeletePrivateKey(context.Background(), &fastly.DeletePrivateKeyInput{ID: key.ID}); err != nil {
			t.Logf("cleanup: deleting out-of-band private key %s: %s", key.ID, err)
		}
	})
}

func TestAccFastlyTLSCertificate_withName(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	domain := testTLSCertificateDomain(t)
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	updatedName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	keyPEM2, certPEM2 := generateTLSKeyAndCert(t, domain)
	uploadOutOfBandPrivateKey(t, client, keyPEM)
	uploadOutOfBandPrivateKey(t, client, keyPEM2)

	resourceName := "fastly_tls_certificate.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSCertificate(certPEM, name),
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
				Config: ConfigTLSCertificate(certPEM2, updatedName),
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

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	domain := testTLSCertificateDomain(t)
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	uploadOutOfBandPrivateKey(t, client, keyPEM)

	resourceName := "fastly_tls_certificate.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSCertificateWithoutName(certPEM),
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

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	domain := testTLSCertificateDomain(t)
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	uploadOutOfBandPrivateKey(t, client, keyPEM)

	config := ConfigTLSCertificate(certPEM, name) + `
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

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	domain := testTLSCertificateDomain(t)
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	uploadOutOfBandPrivateKey(t, client, keyPEM)

	config := ConfigTLSCertificate(certPEM, name) + `
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
