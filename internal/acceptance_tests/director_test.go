package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyServiceDirector_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	directorName := fmt.Sprintf("director-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDirectorBasic(serviceName, domainName, backendName, directorName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "name", directorName),
					resource.TestCheckResourceAttr("fastly_service_director.test", "backends.#", "1"),
					resource.TestCheckTypeSetElemAttr("fastly_service_director.test", "backends.*", backendName),
					resource.TestCheckResourceAttr("fastly_service_director.test", "comment", ""),
					resource.TestCheckResourceAttr("fastly_service_director.test", "quorum", "75"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "retries", "5"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "shield", ""),
					resource.TestCheckResourceAttr("fastly_service_director.test", "type", "random"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_director.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_director.test", "id"),
				),
			},
			{
				// The provider canonicalizes omitted optional fields to their documented
				// defaults - re-applying the same config must not show drift.
				Config:   ConfigDirectorBasic(serviceName, domainName, backendName, directorName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceDirector_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	directorName := fmt.Sprintf("director-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDirectorBasic(serviceName, domainName, backendName, directorName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_director.test", "comment", ""),
					resource.TestCheckResourceAttr("fastly_service_director.test", "quorum", "75"),
				),
			},
			{
				Config: ConfigDirectorUpdated(serviceName, domainName, backendName, directorName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_director.test", "comment", "updated director"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "quorum", "30"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "retries", "10"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "shield", "sjc-ca-us"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "type", "hash"),
				),
			},
		},
	})
}

// TestAccFastlyServiceDirector_computeServiceRejected verifies that fastly_service_director, a
// VCL-only resource (directors are not supported for Compute services in the legacy provider
// either), is rejected when targeting a Compute service.
func TestAccFastlyServiceDirector_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	directorName := fmt.Sprintf("director-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigDirectorOnComputeService(serviceName, directorName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_director does not support Fastly service.*of type "Compute"`),
			},
		},
	})
}

// TestAccFastlyServiceDirector_lockedVersion verifies that the provider refuses to write a
// director to an activated (locked) service version. Version 1, which holds the service, domain,
// and backend the test cleans up afterward, is never activated - only a cloned version 2 is - so
// the locked-version write attempt itself never lands in state and cleanup of version 1 is
// unaffected.
func TestAccFastlyServiceDirector_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	directorName := fmt.Sprintf("director-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceCDNWithDomain(serviceName, domainName, 1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_cdn.test"]
						if !ok {
							return fmt.Errorf("service resource not found")
						}

						client, err := NewFastlyClient()
						if err != nil {
							return fmt.Errorf("error creating Fastly client: %w", err)
						}

						ctx := context.Background()
						cloned, err := client.CloneVersion(ctx, &fastly.CloneVersionInput{
							ServiceID:      rs.Primary.ID,
							ServiceVersion: 1,
						})
						if err != nil {
							return fmt.Errorf("error cloning version: %w", err)
						}

						_, err = client.ActivateVersion(ctx, &fastly.ActivateVersionInput{
							ServiceID:      rs.Primary.ID,
							ServiceVersion: fastly.ToValue(cloned.Number),
						})
						if err != nil {
							return fmt.Errorf("error activating cloned version: %w", err)
						}

						return nil
					},
				),
			},
			{
				Config:      ConfigDirectorOnLockedVersion(serviceName, domainName, backendName, directorName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceDirector_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	directorName := fmt.Sprintf("director-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDirectorForImport(serviceName, domainName, backendName, directorName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "name", directorName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_director.test"]
						if !ok {
							return fmt.Errorf("director resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_director.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, directorName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceDirector_nameForcesReplace verifies changing name forces destroy/create
// rather than an in-place update.
func TestAccFastlyServiceDirector_nameForcesReplace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	directorName1 := fmt.Sprintf("director-%s", acctest.RandString(10))
	directorName2 := fmt.Sprintf("director-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDirectorBasic(serviceName, domainName, backendName, directorName1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_director.test", "name", directorName1),
				),
			},
			{
				Config: ConfigDirectorBasic(serviceName, domainName, backendName, directorName2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_director.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("fastly_service_director.test", "name", directorName2),
			},
		},
	})
}
