package acceptancetests

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
)

func testDomainFQDN(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s.fastly-example.com", acctest.RandString(10))
}

func TestAccFastlyDomain_basic(t *testing.T) {
	t.Parallel()
	fqdn := testDomainFQDN(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckFastlyDomainDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigFastlyDomain(fqdn, "initial description"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_domain.test", "fqdn", fqdn),
					resource.TestCheckResourceAttr("fastly_domain.test", "description", "initial description"),
					resource.TestCheckResourceAttrSet("fastly_domain.test", "id"),
					resource.TestCheckResourceAttr("fastly_domain.test", "service_id", ""),
				),
			},
			{
				Config: ConfigFastlyDomain(fqdn, "updated description"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_domain.test", "description", "updated description"),
				),
			},
			{
				ResourceName:      "fastly_domain.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// fqdn changes force replace - the API can't rename a domain.
func TestAccFastlyDomain_fqdnForcesReplace(t *testing.T) {
	t.Parallel()
	fqdn := testDomainFQDN(t)
	renamed := testDomainFQDN(t)

	var firstID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckFastlyDomainDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigFastlyDomainMinimal(fqdn),
				Check: resource.ComposeTestCheckFunc(
					captureResourceID("fastly_domain.test", &firstID),
				),
			},
			{
				Config: ConfigFastlyDomainMinimal(renamed),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_domain.test", "fqdn", renamed),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_domain.test"]
						if !ok {
							return fmt.Errorf("not found: fastly_domain.test")
						}
						if rs.Primary.ID == firstID {
							return fmt.Errorf("expected a new domain ID after renaming, got the same ID %s", firstID)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccFastlyDomainServiceLink_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	fqdn := testDomainFQDN(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckFastlyDomainDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigFastlyDomainWithServiceLink(serviceName, fqdn),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_domain_service_link.test", "domain_id"),
					resource.TestCheckResourceAttrPair("fastly_domain_service_link.test", "service_id", "fastly_service_cdn.test", "id"),
				),
			},
			{
				// fastly_domain.test only picks up the link on a refresh.
				Config: ConfigFastlyDomainWithServiceLink(serviceName, fqdn),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("fastly_domain.test", "service_id", "fastly_service_cdn.test", "id"),
				),
			},
			{
				ResourceName:      "fastly_domain_service_link.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyDataSourceDomains(t *testing.T) {
	t.Parallel()
	h := acctest.RandString(10)
	fqdn1 := fmt.Sprintf("tf-%s-1.fastly-example.com", h)
	fqdn2 := fmt.Sprintf("tf-%s-2.fastly-example.com", h)
	fqdn3 := fmt.Sprintf("tf-%s-3.fastly-example.com", h)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckFastlyDomainDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigFastlyDomainsDataSource(fqdn1, fqdn2, fqdn3),
				Check: resource.ComposeTestCheckFunc(
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["data.fastly_domains.example"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_domains.example")
						}

						want := []string{fqdn1, fqdn2, fqdn3}

						var found int
						var got []string
						for k, v := range rs.Primary.Attributes {
							if strings.HasSuffix(k, ".fqdn") {
								got = append(got, v)
								if slices.Contains(want, v) {
									found++
								}
							}
						}

						if found != len(want) {
							return fmt.Errorf("want: %v, got: %v", want, got)
						}

						return nil
					},
				),
			},
		},
	})
}

func CheckFastlyDomainDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_domain" {
			continue
		}

		id := rs.Primary.ID
		_, err := domains.Get(context.Background(), client, &domains.GetInput{DomainID: &id})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Domain was destroyed: %w", err)
		}

		return fmt.Errorf("Domain %s still exists", id)
	}

	return nil
}
