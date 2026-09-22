package acceptancetests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyServiceDictionary_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	dictionaryName := fmt.Sprintf("dict_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDictionaryExplicit(serviceName, domainName, dictionaryName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "name", dictionaryName),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "version", "1"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "write_only", "false"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "force_destroy", "true"),
					resource.TestCheckResourceAttrSet("fastly_service_dictionary.test", "dictionary_id"),
				),
			},
			{
				Config:   ConfigDictionaryExplicit(serviceName, domainName, dictionaryName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceDictionary_writeOnly(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	dictionaryName := fmt.Sprintf("dict_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDictionaryExplicitWriteOnly(serviceName, domainName, dictionaryName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "name", dictionaryName),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "write_only", "true"),
					resource.TestCheckResourceAttrSet("fastly_service_dictionary.test", "dictionary_id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionary_computeService(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	dictionaryName := fmt.Sprintf("dict_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDictionaryOnComputeService(serviceName, dictionaryName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "name", dictionaryName),
					resource.TestCheckResourceAttrSet("fastly_service_dictionary.test", "dictionary_id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionary_import(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	dictionaryName := fmt.Sprintf("dict_%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDictionaryExplicit(serviceName, domainName, dictionaryName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "name", dictionaryName),
					resource.TestCheckResourceAttrSet("fastly_service_dictionary.test", "dictionary_id"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "version", "1"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_dictionary.test"]
						if !ok {
							return fmt.Errorf("dictionary resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_dictionary.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, dictionaryName), nil
				},
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force_destroy"},
			},
		},
	})
}

func TestAccFastlyServiceDictionary_importWithUnderscores(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	dictionaryName := fmt.Sprintf("dict_%s_with_underscores", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDictionaryExplicit(serviceName, domainName, dictionaryName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "name", dictionaryName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_dictionary.test"]
						if !ok {
							return fmt.Errorf("dictionary resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_dictionary.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, dictionaryName), nil
				},
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force_destroy"},
			},
		},
	})
}

// TestAccFastlyServiceDictionary_versionUpdateInPlace verifies that changing only the version
// attribute on an explicit fastly_service_dictionary resource -- pointing it at a version that
// was cloned from the one it started on -- applies successfully and refreshes id/dictionary_id
// from the new version, rather than leaving them stale from the version the resource was created
// against.
func TestAccFastlyServiceDictionary_versionUpdateInPlace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	dictionaryName := fmt.Sprintf("dict_%s", acctest.RandString(10))

	var serviceID string
	var dictIDAtV1 string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigDictionaryAtVersion(serviceName, domainName, dictionaryName, 1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "name", dictionaryName),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_dictionary.test", "dictionary_id"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_dictionary.test"]
						if !ok {
							return fmt.Errorf("dictionary resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						dictIDAtV1 = rs.Primary.Attributes["dictionary_id"]
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					client, err := NewFastlyClient()
					if err != nil {
						t.Fatalf("error creating Fastly client: %s", err)
					}
					if _, err := client.CloneVersion(context.Background(), &fastly.CloneVersionInput{
						ServiceID:      serviceID,
						ServiceVersion: 1,
					}); err != nil {
						t.Fatalf("error cloning version 1: %s", err)
					}
				},
				Config: ConfigDictionaryAtVersion(serviceName, domainName, dictionaryName, 2),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "name", dictionaryName),
					resource.TestCheckResourceAttr("fastly_service_dictionary.test", "version", "2"),
					resource.TestCheckResourceAttrSet("fastly_service_dictionary.test", "dictionary_id"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_dictionary.test"]
						if !ok {
							return fmt.Errorf("dictionary resource not found")
						}

						gotID := rs.Primary.Attributes["id"]
						wantID := fmt.Sprintf("%s-2-%s", serviceID, dictionaryName)
						if gotID != wantID {
							return fmt.Errorf("expected id %q to reflect version 2, got %q", wantID, gotID)
						}

						client, err := NewFastlyClient()
						if err != nil {
							return fmt.Errorf("error creating Fastly client: %w", err)
						}
						remote, err := client.GetDictionary(context.Background(), &fastly.GetDictionaryInput{
							ServiceID:      serviceID,
							ServiceVersion: 2,
							Name:           dictionaryName,
						})
						if err != nil {
							return fmt.Errorf("error fetching dictionary at version 2: %w", err)
						}

						gotDictID := rs.Primary.Attributes["dictionary_id"]
						if gotDictID != *remote.DictionaryID {
							return fmt.Errorf("state dictionary_id %q does not match version 2's dictionary id %q (stale from version 1: %q)", gotDictID, *remote.DictionaryID, dictIDAtV1)
						}

						return nil
					},
				),
			},
		},
	})
}
