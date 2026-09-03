package acceptancetests

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/fastly/go-fastly/v17/fastly"
)

func testTLSActivationDomain(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s.fastly-example.com", acctest.RandString(10))
}

// generateTLSKey creates a self-signed RSA private key, returning both the key itself (for
// signing one or more certificates via generateTLSCertForKey) and its PEM encoding.
func generateTLSKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating private key: %s", err)
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling private key: %s", err)
	}

	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}))
}

// generateTLSCertForKey issues a self-signed, PEM-encoded certificate for domain, signed by key.
// Calling this more than once for the same key produces distinct certificates (fresh serial
// number and validity window) that still share that key's fingerprint.
func generateTLSCertForKey(t *testing.T, key *rsa.PrivateKey, domain string) string {
	t.Helper()

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		t.Fatalf("generating serial number: %s", err)
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: domain},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{domain},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating certificate: %s", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// generateTLSKeyAndCert creates a self-signed, PEM-encoded RSA key and certificate for domain.
func generateTLSKeyAndCert(t *testing.T, domain string) (keyPEM, certPEM string) {
	t.Helper()

	key, keyPEM := generateTLSKey(t)
	return keyPEM, generateTLSCertForKey(t, key, domain)
}

func TestAccFastlyTLSActivation_basic(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	keyPEM1, certPEM1 := generateTLSKeyAndCert(t, domain)
	keyPEM2, certPEM2 := generateTLSKeyAndCert(t, domain)
	certName1 := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	certName2 := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	cert1 := ConfigTLSCertificatePair("cert1", certName1, keyPEM1, certPEM1)
	cert2 := ConfigTLSCertificatePair("cert2", certName2, keyPEM2, certPEM2)

	resourceName := "fastly_tls_activation.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSActivationDestroy,
		Steps: []resource.TestStep{
			{
				Config: joinBlocks(cert1, ConfigTLSActivation(serviceName, domain, backendName, "fastly_tls_certificate.cert1.id", ", fastly_tls_certificate.cert1")),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrPair(resourceName, "certificate_id", "fastly_tls_certificate.cert1", "id"),
					resource.TestCheckResourceAttr(resourceName, "domain", domain),
					resource.TestCheckResourceAttrSet(resourceName, "configuration_id"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					testAccCheckTLSActivationExists(),
				),
			},
			{
				// certificate_id has no RequiresReplace, so this updates in place. cert1 stays
				// declared alongside cert2 to mirror the create-alongside/repoint/delete-old
				// rotation workflow described in the fastly_tls_certificate documentation.
				Config: joinBlocks(cert1, cert2, ConfigTLSActivation(serviceName, domain, backendName, "fastly_tls_certificate.cert2.id", ", fastly_tls_certificate.cert1, fastly_tls_certificate.cert2")),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(resourceName, "certificate_id", "fastly_tls_certificate.cert2", "id"),
					testAccCheckTLSActivationExists(),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyTLSActivation_mtls(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	certName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	cert := ConfigTLSCertificatePair("test", certName, keyPEM, certPEM)
	_, mtlsCertBundle := generateTLSKeyAndCert(t, domain)

	resourceName := "fastly_tls_activation.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSActivationDestroy,
		Steps: []resource.TestStep{
			{
				Config: joinBlocks(cert, ConfigTLSActivation(serviceName, domain, backendName, "fastly_tls_certificate.test.id", ", fastly_tls_certificate.test")),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					testAccCheckTLSActivationExists(),
				),
			},
			{
				// mutual_authentication_id only settable via a follow-up update: fastly/terraform-provider-fastly#873
				Config: joinBlocks(cert, ConfigTLSActivationWithMutualAuthentication(serviceName, domain, backendName, "fastly_tls_certificate.test.id", ", fastly_tls_certificate.test", mtlsCertBundle)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(resourceName, "mutual_authentication_id", "fastly_tls_mutual_authentication.test", "id"),
					testAccCheckTLSActivationExists(),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyTLSActivation_missingCertificateID(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigTLSActivation(serviceName, domain, backendName, `""`, ""),
				ExpectError: regexp.MustCompile("certificate_id is empty"),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSActivation(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	certName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	cert := ConfigTLSCertificatePair("test", certName, keyPEM, certPEM)

	config := joinBlocks(cert, ConfigTLSActivation(serviceName, domain, backendName, "fastly_tls_certificate.test.id", ", fastly_tls_certificate.test")) + `
data "fastly_tls_activation" "by_domain" {
  domain     = fastly_tls_activation.test.domain
  depends_on = [fastly_tls_activation.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSActivationDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.fastly_tls_activation.by_domain", "domain", domain),
					resource.TestCheckResourceAttrPair("data.fastly_tls_activation.by_domain", "certificate_id", "fastly_tls_certificate.test", "id"),
					resource.TestCheckResourceAttrPair("data.fastly_tls_activation.by_domain", "id", "fastly_tls_activation.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSActivationIDs(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
	certName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	cert := ConfigTLSCertificatePair("test", certName, keyPEM, certPEM)

	config := joinBlocks(cert, ConfigTLSActivation(serviceName, domain, backendName, "fastly_tls_certificate.test.id", ", fastly_tls_certificate.test")) + `
data "fastly_tls_activation_ids" "by_cert" {
  certificate_id = fastly_tls_activation.test.certificate_id
  depends_on     = [fastly_tls_activation.test]
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSActivationDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.fastly_tls_activation_ids.by_cert", "ids.#", "1"),
				),
			},
		},
	})
}

func testAccCheckTLSActivationExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		rs, ok := s.RootModule().Resources["fastly_tls_activation.test"]
		if !ok {
			return fmt.Errorf("not found: fastly_tls_activation.test")
		}

		_, err = client.GetTLSActivation(context.Background(), &fastly.GetTLSActivationInput{ID: rs.Primary.ID})
		return err
	}
}

func CheckTLSActivationDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_tls_activation" {
			continue
		}

		_, err := client.GetTLSActivation(context.Background(), &fastly.GetTLSActivationInput{ID: rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if TLS activation was destroyed: %w", err)
		}

		return fmt.Errorf("TLS activation %s still exists", rs.Primary.ID)
	}

	return nil
}
