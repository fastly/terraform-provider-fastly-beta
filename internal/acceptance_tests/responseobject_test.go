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

func TestAccFastlyServiceResponseObject_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	responseObjectName := fmt.Sprintf("response-object-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigResponseObjectBasic(serviceName, domainName, responseObjectName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "name", responseObjectName),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "status", "200"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "response", "OK"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "content", "test content"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "content_type", "text/html"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "request_condition", ""),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "cache_condition", ""),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_response_object.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_response_object.test", "id"),
				),
			},
			{
				Config:   ConfigResponseObjectBasic(serviceName, domainName, responseObjectName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceResponseObject_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	responseObjectName := fmt.Sprintf("response-object-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigResponseObjectBasic(serviceName, domainName, responseObjectName),
				Check:  resource.TestCheckResourceAttr("fastly_service_response_object.test", "status", "200"),
			},
			{
				Config: ConfigResponseObjectUpdated(serviceName, domainName, responseObjectName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_response_object.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "name", responseObjectName),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "status", "404"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "response", "Not Found"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "content", "updated response content"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "content_type", "text/csv"),
				),
			},
			{
				Config:   ConfigResponseObjectUpdated(serviceName, domainName, responseObjectName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceResponseObject_minimalDefaults(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	responseObjectName := fmt.Sprintf("response-object-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigResponseObjectMinimal(serviceName, domainName, responseObjectName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "name", responseObjectName),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "status", "200"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "response", "OK"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "content", ""),
					resource.TestCheckNoResourceAttr("fastly_service_response_object.test", "content_type"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "request_condition", ""),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "cache_condition", ""),
				),
			},
			{
				Config:   ConfigResponseObjectMinimal(serviceName, domainName, responseObjectName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceResponseObject_withConditions(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	responseObjectName := fmt.Sprintf("response-object-%s", acctest.RandString(10))
	requestConditionName := fmt.Sprintf("request-condition-%s", acctest.RandString(10))
	cacheConditionName := fmt.Sprintf("cache-condition-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigResponseObjectWithConditions(serviceName, domainName, responseObjectName, requestConditionName, cacheConditionName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_condition.request", "type", "REQUEST"),
					resource.TestCheckResourceAttr("fastly_service_condition.cache", "type", "CACHE"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "request_condition", requestConditionName),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "cache_condition", cacheConditionName),
				),
			},
		},
	})
}

// TestAccFastlyServiceResponseObject_computeServiceRejected verifies Response Objects remain
// VCL/CDN-only when represented as a first-class explicit resource.
func TestAccFastlyServiceResponseObject_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	responseObjectName := fmt.Sprintf("response-object-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigResponseObjectOnComputeService(serviceName, responseObjectName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_response_object does not support Fastly service.*of type "Compute"`),
			},
		},
	})
}

// TestAccFastlyServiceResponseObject_lockedVersion verifies the provider refuses to write to an
// activated (locked) service version.
func TestAccFastlyServiceResponseObject_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	responseObjectName := fmt.Sprintf("response-object-%s", acctest.RandString(10))

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
				Config:      ConfigResponseObjectOnLockedVersion(serviceName, domainName, responseObjectName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceResponseObject_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	responseObjectName := fmt.Sprintf("response-object-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigResponseObjectForImport(serviceName, domainName, responseObjectName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "name", responseObjectName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_response_object.test"]
						if !ok {
							return fmt.Errorf("response object resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_response_object.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, responseObjectName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceResponseObject_nameForcesReplace verifies changing the stable name identity
// replaces the resource instead of attempting the API's rename path in place.
func TestAccFastlyServiceResponseObject_nameForcesReplace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	responseObjectName1 := fmt.Sprintf("response-object-%s", acctest.RandString(10))
	responseObjectName2 := fmt.Sprintf("response-object-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigResponseObjectBasic(serviceName, domainName, responseObjectName1),
				Check:  resource.TestCheckResourceAttr("fastly_service_response_object.test", "name", responseObjectName1),
			},
			{
				Config: ConfigResponseObjectBasic(serviceName, domainName, responseObjectName2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_response_object.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("fastly_service_response_object.test", "name", responseObjectName2),
			},
		},
	})
}

// TestAccFastlyServiceResponseObject_delete verifies removing only the first-class Response Object
// deletes the remote object while leaving the explicit service/version in place.
func TestAccFastlyServiceResponseObject_delete(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	responseObjectName := fmt.Sprintf("response-object-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigResponseObjectBasic(serviceName, domainName, responseObjectName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_response_object.test", "name", responseObjectName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_response_object.test"]
						if !ok {
							return fmt.Errorf("response object resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						if _, err := fmt.Sscanf(rs.Primary.Attributes["version"], "%d", &versionNumber); err != nil {
							return fmt.Errorf("error parsing response object version: %w", err)
						}
						return nil
					},
				),
			},
			{
				Config: ConfigServiceCDNWithDomain(serviceName, domainName, 1),
				Check: func(_ *terraform.State) error {
					client, err := NewFastlyClient()
					if err != nil {
						return fmt.Errorf("error creating Fastly client: %w", err)
					}
					items, err := client.ListResponseObjects(context.Background(), &fastly.ListResponseObjectsInput{
						ServiceID:      serviceID,
						ServiceVersion: versionNumber,
					})
					if err != nil {
						return fmt.Errorf("error listing response objects after delete: %w", err)
					}
					for _, item := range items {
						if item != nil && fastly.ToValue(item.Name) == responseObjectName {
							return fmt.Errorf("response object %q still exists after Terraform delete", responseObjectName)
						}
					}
					return nil
				},
			},
		},
	})
}
