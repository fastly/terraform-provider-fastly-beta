package acceptancetests

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyDataSourceTLSDomain_basic(t *testing.T) {
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

	config := joinBlocks(cert, ConfigTLSActivation(serviceName, domain, backendName, "fastly_tls_certificate.test.id", "fastly_tls_certificate.test")) + `
data "fastly_tls_domain" "subject" {
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
					resource.TestCheckResourceAttr("data.fastly_tls_domain.subject", "domain", domain),
					resource.TestCheckResourceAttr("data.fastly_tls_domain.subject", "tls_certificate_ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(
						"data.fastly_tls_domain.subject", "tls_certificate_ids.*",
						"fastly_tls_certificate.test", "id",
					),
					resource.TestCheckResourceAttr("data.fastly_tls_domain.subject", "tls_activation_ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(
						"data.fastly_tls_domain.subject", "tls_activation_ids.*",
						"fastly_tls_activation.test", "id",
					),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceTLSDomain_noMatch(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	domain := testTLSActivationDomain(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "fastly_tls_domain" "subject" {
  domain = %q
}
`, domain),
				ExpectError: regexp.MustCompile("your query returned no results"),
			},
		},
	})
}
