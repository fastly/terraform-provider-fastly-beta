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

// generateTLSKeyAndCert creates a self-signed, PEM-encoded RSA key and certificate for domain.
func generateTLSKeyAndCert(t *testing.T, domain string) (keyPEM, certPEM string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating private key: %s", err)
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling private key: %s", err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}))

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
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))

	return keyPEM, certPEM
}

// createOutOfBandCertificate creates a private key + certificate for domain via the raw
// go-fastly client, since no fastly_tls_private_key/fastly_tls_certificate resource exists
// yet (CDTOOL-1586, CDTOOL-1583). Registers cleanup, returns the certificate ID.
//
// TODO: rework via Terraform config once those resources exist.
func createOutOfBandCertificate(t *testing.T, client *fastly.Client, domain string) string {
	t.Helper()

	keyPEM, certPEM := generateTLSKeyAndCert(t, domain)
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

	cert, err := client.CreateCustomTLSCertificate(context.Background(), &fastly.CreateCustomTLSCertificateInput{
		CertBlob: certPEM,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("creating out-of-band certificate: %s", err)
	}
	t.Cleanup(func() {
		if err := client.DeleteCustomTLSCertificate(context.Background(), &fastly.DeleteCustomTLSCertificateInput{ID: cert.ID}); err != nil {
			t.Logf("cleanup: deleting out-of-band certificate %s: %s", cert.ID, err)
		}
	})

	return cert.ID
}

// createOutOfBandMutualAuthentication creates a mutual authentication via the raw go-fastly
// client, since no fastly_tls_mutual_authentication resource exists yet (CDTOOL-1584).
// Registers cleanup, returns its ID.
//
// TODO: rework via Terraform config once that resource exists.
func createOutOfBandMutualAuthentication(t *testing.T, client *fastly.Client, domain string) string {
	t.Helper()

	_, certPEM := generateTLSKeyAndCert(t, domain)
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	mtls, err := client.CreateTLSMutualAuthentication(context.Background(), &fastly.CreateTLSMutualAuthenticationInput{
		CertBundle: certPEM,
		Name:       name,
	})
	if err != nil {
		t.Fatalf("creating out-of-band mutual authentication: %s", err)
	}
	t.Cleanup(func() {
		if err := client.DeleteTLSMutualAuthentication(context.Background(), &fastly.DeleteTLSMutualAuthenticationInput{ID: mtls.ID}); err != nil {
			t.Logf("cleanup: deleting out-of-band mutual authentication %s: %s", mtls.ID, err)
		}
	})

	return mtls.ID
}

func TestAccFastlyTLSActivation_basic(t *testing.T) {
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

	certificateID1 := createOutOfBandCertificate(t, client, domain)
	certificateID2 := createOutOfBandCertificate(t, client, domain)

	resourceName := "fastly_tls_activation.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSActivationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSActivation(serviceName, domain, backendName, certificateID1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "certificate_id", certificateID1),
					resource.TestCheckResourceAttr(resourceName, "domain", domain),
					resource.TestCheckResourceAttrSet(resourceName, "configuration_id"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					testAccCheckTLSActivationExists(),
				),
			},
			{
				// certificate_id has no RequiresReplace, so this updates in place.
				Config: ConfigTLSActivation(serviceName, domain, backendName, certificateID2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "certificate_id", certificateID2),
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

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	certificateID := createOutOfBandCertificate(t, client, domain)
	mtlsID := createOutOfBandMutualAuthentication(t, client, domain)

	resourceName := "fastly_tls_activation.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSActivationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSActivation(serviceName, domain, backendName, certificateID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					testAccCheckTLSActivationExists(),
				),
			},
			{
				// mutual_authentication_id only settable via a follow-up update: fastly/terraform-provider-fastly#873
				Config: ConfigTLSActivationWithMTLS(serviceName, domain, backendName, certificateID, mtlsID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "mutual_authentication_id", mtlsID),
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
				Config:      ConfigTLSActivation(serviceName, domain, backendName, ""),
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

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	certificateID := createOutOfBandCertificate(t, client, domain)

	config := ConfigTLSActivation(serviceName, domain, backendName, certificateID) + `
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
					resource.TestCheckResourceAttr("data.fastly_tls_activation.by_domain", "certificate_id", certificateID),
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

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	domain := testTLSActivationDomain(t)
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	backendName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	certificateID := createOutOfBandCertificate(t, client, domain)

	config := ConfigTLSActivation(serviceName, domain, backendName, certificateID) + `
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
