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
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

// TestAccFastlyTLSPlatformCertificate_lifecycle exercises create, update
// (re-uploading a new certificate/intermediates pair, which the API applies
// in place rather than issuing a new certificate ID), and import.
//
// The certificate uploaded here is self-signed (allow_untrusted_root=true),
// mirroring the legacy provider's own documented example: Platform TLS
// bulk-certificate upload has no domain-ownership validation step, unlike
// TLS Subscriptions, so a self-signed cert for an arbitrary test domain is
// enough to exercise the resource.
func TestAccFastlyTLSPlatformCertificate_lifecycle(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	suffix := acctest.RandString(10)
	domain := fmt.Sprintf("tf-test-%s.example.com", suffix)

	leafCertPEM, leafKeyPEM, rootCertPEM := newSelfSignedTestCertificate(t, domain)
	registerTestPrivateKey(t, client, leafKeyPEM, fmt.Sprintf("tf-test-platform-cert-%s", suffix))

	updatedLeafCertPEM, updatedLeafKeyPEM, updatedRootCertPEM := newSelfSignedTestCertificate(t, domain)
	registerTestPrivateKey(t, client, updatedLeafKeyPEM, fmt.Sprintf("tf-test-platform-cert-updated-%s", suffix))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSPlatformCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSPlatformCertificate(leafCertPEM, rootCertPEM),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_tls_platform_certificate.test", "id"),
					resource.TestCheckResourceAttrPair("fastly_tls_platform_certificate.test", "configuration_id", "data.fastly_tls_configuration.platform", "id"),
					resource.TestCheckResourceAttr("fastly_tls_platform_certificate.test", "allow_untrusted_root", "true"),
					resource.TestCheckResourceAttrSet("fastly_tls_platform_certificate.test", "not_before"),
					resource.TestCheckResourceAttrSet("fastly_tls_platform_certificate.test", "not_after"),
					resource.TestCheckResourceAttrSet("fastly_tls_platform_certificate.test", "created_at"),
					resource.TestCheckResourceAttrSet("fastly_tls_platform_certificate.test", "updated_at"),
					resource.TestCheckResourceAttr("fastly_tls_platform_certificate.test", "replace", "false"),
					resource.TestCheckTypeSetElemAttr("fastly_tls_platform_certificate.test", "domains.*", domain),
				),
			},
			{
				// Re-uploading a new certificate/intermediates pair updates the
				// existing certificate in place (its ID does not change) and only
				// SAN entries present in the replacement remain TLS-enabled.
				Config: ConfigTLSPlatformCertificate(updatedLeafCertPEM, updatedRootCertPEM),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_tls_platform_certificate.test", "id"),
					resource.TestCheckResourceAttrPair("fastly_tls_platform_certificate.test", "configuration_id", "data.fastly_tls_configuration.platform", "id"),
					resource.TestCheckTypeSetElemAttr("fastly_tls_platform_certificate.test", "domains.*", domain),
				),
			},
			{
				// certificate_body, intermediates_blob, and allow_untrusted_root are
				// never returned by the read endpoint, so a fresh import cannot
				// recover them - see FlattenToModel/carryForwardWriteOnly.
				ResourceName:            "fastly_tls_platform_certificate.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"certificate_body", "intermediates_blob", "allow_untrusted_root"},
			},
		},
	})
}

func TestAccFastlyDataSourceTLSPlatformCertificate_byID(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	suffix := acctest.RandString(10)
	domain := fmt.Sprintf("tf-test-%s.example.com", suffix)
	leafCertPEM, leafKeyPEM, rootCertPEM := newSelfSignedTestCertificate(t, domain)
	registerTestPrivateKey(t, client, leafKeyPEM, fmt.Sprintf("tf-test-platform-cert-ds-%s", suffix))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSPlatformCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSPlatformCertificateWithIDDataSource(leafCertPEM, rootCertPEM),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.fastly_tls_platform_certificate.by_id", "id", "fastly_tls_platform_certificate.test", "id"),
					resource.TestCheckResourceAttrPair("data.fastly_tls_platform_certificate.by_id", "configuration_id", "fastly_tls_platform_certificate.test", "configuration_id"),
					resource.TestCheckTypeSetElemAttr("data.fastly_tls_platform_certificate.by_id", "domains.*", domain),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSPlatformCertificate_byDomain(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	suffix := acctest.RandString(10)
	domain := fmt.Sprintf("tf-test-%s.example.com", suffix)
	leafCertPEM, leafKeyPEM, rootCertPEM := newSelfSignedTestCertificate(t, domain)
	registerTestPrivateKey(t, client, leafKeyPEM, fmt.Sprintf("tf-test-platform-cert-ds-domain-%s", suffix))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSPlatformCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSPlatformCertificateWithDomainDataSource(leafCertPEM, rootCertPEM, domain),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.fastly_tls_platform_certificate.by_domain", "id", "fastly_tls_platform_certificate.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSPlatformCertificateIDs(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	suffix := acctest.RandString(10)
	domain := fmt.Sprintf("tf-test-%s.example.com", suffix)
	leafCertPEM, leafKeyPEM, rootCertPEM := newSelfSignedTestCertificate(t, domain)
	registerTestPrivateKey(t, client, leafKeyPEM, fmt.Sprintf("tf-test-platform-cert-ds-ids-%s", suffix))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckTLSPlatformCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigTLSPlatformCertificateWithIDsDataSource(leafCertPEM, rootCertPEM),
				Check: resource.ComposeTestCheckFunc(
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_tls_platform_certificate.test"]
						if !ok {
							return fmt.Errorf("not found: fastly_tls_platform_certificate.test")
						}
						certID := rs.Primary.ID

						ds, ok := s.RootModule().Resources["data.fastly_tls_platform_certificate_ids.test"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_tls_platform_certificate_ids.test")
						}

						found := false
						for k, v := range ds.Primary.Attributes {
							if k != "id" && v == certID {
								found = true
								break
							}
						}
						if !found {
							return fmt.Errorf("expected ids set to contain %s, got attributes %v", certID, ds.Primary.Attributes)
						}
						return nil
					},
				),
			},
		},
	})
}

func ConfigTLSPlatformCertificate(certPEM, intermediatesPEM string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_platform_certificate_single.tf", map[string]string{
		"CERTIFICATE_BODY":   certPEM,
		"INTERMEDIATES_BLOB": intermediatesPEM,
	})
}

func ConfigTLSPlatformCertificateWithIDDataSource(certPEM, intermediatesPEM string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_platform_certificate_with_id_datasource.tf", map[string]string{
		"CERTIFICATE_BODY":   certPEM,
		"INTERMEDIATES_BLOB": intermediatesPEM,
	})
}

func ConfigTLSPlatformCertificateWithDomainDataSource(certPEM, intermediatesPEM, domain string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_platform_certificate_with_domain_datasource.tf", map[string]string{
		"CERTIFICATE_BODY":   certPEM,
		"INTERMEDIATES_BLOB": intermediatesPEM,
		"DOMAIN":             domain,
	})
}

func ConfigTLSPlatformCertificateWithIDsDataSource(certPEM, intermediatesPEM string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_platform_certificate_ids_datasource.tf", map[string]string{
		"CERTIFICATE_BODY":   certPEM,
		"INTERMEDIATES_BLOB": intermediatesPEM,
	})
}

func CheckTLSPlatformCertificateDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_tls_platform_certificate" {
			continue
		}

		if _, err := client.GetBulkCertificate(context.Background(), &fastly.GetBulkCertificateInput{ID: rs.Primary.ID}); err == nil {
			return fmt.Errorf("TLS platform certificate %s still exists after destroy", rs.Primary.ID)
		}
	}
	return nil
}

// registerTestPrivateKey uploads keyPEM to Fastly's TLS Private Keys store
// and schedules its deletion. The Platform TLS API requires a certificate's
// private key to be uploaded, and matched by fingerprint, before the
// certificate itself can be created.
func registerTestPrivateKey(t *testing.T, client *fastly.Client, keyPEM, name string) {
	t.Helper()

	key, err := client.CreatePrivateKey(context.Background(), &fastly.CreatePrivateKeyInput{
		Key:  keyPEM,
		Name: name,
	})
	if err != nil {
		t.Fatalf("uploading test private key: %s", err)
	}
	t.Cleanup(func() {
		if err := client.DeletePrivateKey(context.Background(), &fastly.DeletePrivateKeyInput{ID: key.ID}); err != nil {
			t.Logf("cleanup: deleting private key %s: %s", key.ID, err)
		}
	})
}

// newSelfSignedTestCertificate generates a throwaway root CA and a leaf
// certificate for domain, signed by that CA. It returns PEM-encoded
// (leaf certificate, leaf private key, root certificate).
func newSelfSignedTestCertificate(t *testing.T, domain string) (leafCertPEM, leafKeyPEM, rootCertPEM string) {
	t.Helper()

	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating root key: %s", err)
	}
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "tf-test-root-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("creating root certificate: %s", err)
	}
	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatalf("parsing root certificate: %s", err)
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating leaf key: %s", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() + 1),
		Subject:      pkix.Name{CommonName: domain},
		DNSNames:     []string{domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, rootCert, &leafKey.PublicKey, rootKey)
	if err != nil {
		t.Fatalf("creating leaf certificate: %s", err)
	}

	leafCertPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}))
	rootCertPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}))
	leafKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)}))
	return leafCertPEM, leafKeyPEM, rootCertPEM
}
