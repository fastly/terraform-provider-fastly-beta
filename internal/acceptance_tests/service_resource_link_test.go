package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyServiceResourceLink_ACL(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("tf_test_acl_%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndACLDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeWithACLResourceLink(serviceName, aclName, linkName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					resource.TestCheckResourceAttr("fastly_acl.acl", "name", aclName),
					resource.TestCheckResourceAttrSet("fastly_acl.acl", "id"),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "name", linkName),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "version", "1"),
					resource.TestCheckResourceAttrPair("fastly_service_resource_link.test", "resource_id", "fastly_acl.acl", "id"),
					resource.TestCheckResourceAttrSet("fastly_service_resource_link.test", "link_id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceResourceLink_ACLRename(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("tf_test_acl_%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))
	linkNameRenamed := fmt.Sprintf("tf_test_link_renamed_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndACLDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeWithACLResourceLink(serviceName, aclName, linkName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "name", linkName),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "version", "1"),
				),
			},
			{
				// Renaming the alias is applied in place via UpdateResource; the underlying
				// ACL and the same service version are untouched.
				Config: ConfigServiceComputeWithACLResourceLink(serviceName, aclName, linkNameRenamed),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "name", linkNameRenamed),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "version", "1"),
					resource.TestCheckResourceAttrPair("fastly_service_resource_link.test", "resource_id", "fastly_acl.acl", "id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceResourceLink_ACLRetarget(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("tf_test_acl_%s", acctest.RandString(10))
	aclNameOther := fmt.Sprintf("tf_test_acl_other_%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndACLDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeWithACLResourceLink(serviceName, aclName, linkName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_acl.acl", "name", aclName),
					resource.TestCheckResourceAttrPair("fastly_service_resource_link.test", "resource_id", "fastly_acl.acl", "id"),
				),
			},
			{
				// Pointing the link at a different ACL forces replacement of the link
				// (resource_id is RequiresReplace); the original ACL is then destroyed
				// since nothing else references it.
				Config: ConfigServiceComputeWithACLResourceLink(serviceName, aclNameOther, linkName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_acl.acl", "name", aclNameOther),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "name", linkName),
					resource.TestCheckResourceAttrPair("fastly_service_resource_link.test", "resource_id", "fastly_acl.acl", "id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceResourceLink_ACLImport(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("tf_test_acl_%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))

	var serviceID, version string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndACLDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeWithACLResourceLink(serviceName, aclName, linkName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_service_resource_link.test", "link_id"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_resource_link.test"]
						if !ok {
							return fmt.Errorf("resource link resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						version = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_resource_link.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, version, linkName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyServiceResourceLink_KVStore(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	kvStoreName := fmt.Sprintf("tf-test-kv-%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndKVStoreDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkName, 1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "name", linkName),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "version", "1"),
					resource.TestCheckResourceAttrPair("fastly_service_resource_link.test", "resource_id", "fastly_kvstore.store", "id"),
					resource.TestCheckResourceAttrSet("fastly_service_resource_link.test", "link_id"),
					resource.TestCheckResourceAttrSet("fastly_service_resource_link.test", "id"),
					CheckExplicitResourceLinkInFastly("fastly_service_resource_link.test", 1),
				),
			},
			{
				Config:   ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkName, 1),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceResourceLink_KVStoreRename(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	kvStoreName := fmt.Sprintf("tf-test-kv-%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))
	linkNameRenamed := fmt.Sprintf("tf_test_link_renamed_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndKVStoreDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkName, 1),
				Check:  resource.TestCheckResourceAttr("fastly_service_resource_link.test", "name", linkName),
			},
			{
				Config: ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkNameRenamed, 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_resource_link.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "name", linkNameRenamed),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "version", "1"),
					resource.TestCheckResourceAttrPair("fastly_service_resource_link.test", "resource_id", "fastly_kvstore.store", "id"),
					CheckExplicitResourceLinkInFastly("fastly_service_resource_link.test", 1),
				),
			},
		},
	})
}

// TestAccFastlyServiceResourceLink_KVStoreImport covers both import paths: the
// service_id/version/name ID string, and an import block keyed on the resource identity
// (service_id/resource_id), where Read has to resolve the link from resource_id alone.
func TestAccFastlyServiceResourceLink_KVStoreImport(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	kvStoreName := fmt.Sprintf("tf-test-kv-%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))

	var serviceID, version string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndKVStoreDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkName, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_service_resource_link.test", "link_id"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_resource_link.test"]
						if !ok {
							return fmt.Errorf("resource link resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						version = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_resource_link.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, version, linkName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:    "fastly_service_resource_link.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
			},
		},
	})
}

// TestAccFastlyServiceResourceLink_versionMove verifies that pointing the link at a newer draft
// version (cloned out-of-band, so it already carries the link) is an in-place update that adopts
// the cloned link rather than creating a new one.
func TestAccFastlyServiceResourceLink_versionMove(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	kvStoreName := fmt.Sprintf("tf-test-kv-%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndKVStoreDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkName, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "version", "1"),
					cloneServiceVersionOutOfBand("fastly_service_compute.test", 1, false),
				),
			},
			{
				Config: ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkName, 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_resource_link.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "version", "2"),
					resource.TestCheckResourceAttr("fastly_service_resource_link.test", "name", linkName),
					resource.TestCheckResourceAttrPair("fastly_service_resource_link.test", "resource_id", "fastly_kvstore.store", "id"),
					CheckExplicitResourceLinkInFastly("fastly_service_resource_link.test", 2),
				),
			},
			{
				Config:   ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkName, 2),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceResourceLink_lockedVersion verifies that the provider refuses to write a
// resource link to a locked service version. Compute versions can't be activated without a
// package, so version 2 is cloned and locked directly instead.
func TestAccFastlyServiceResourceLink_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	kvStoreName := fmt.Sprintf("tf-test-kv-%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndKVStoreDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceComputeBasic(serviceName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					cloneServiceVersionOutOfBand("fastly_service_compute.test", 1, true),
				),
			},
			{
				Config:      ConfigServiceComputeWithKVStoreResourceLink(serviceName, kvStoreName, linkName, 2),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

// TestAccFastlyServiceResourceLink_cdnServiceRejected verifies that fastly_service_resource_link,
// a Compute-only resource, is rejected when targeting a CDN (VCL) service.
func TestAccFastlyServiceResourceLink_cdnServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	linkName := fmt.Sprintf("tf_test_link_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigResourceLinkOnCDNService(serviceName, linkName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_resource_link does not support Fastly service.*of type "CDN"`),
			},
		},
	})
}

// CheckExplicitResourceLinkInFastly verifies the link exists remotely at the version in state, and
// that the provider neither cloned past wantVersions nor activated any version on its own.
func CheckExplicitResourceLinkInFastly(resourceName string, wantVersions int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		serviceID := rs.Primary.Attributes["service_id"]
		version, err := strconv.Atoi(rs.Primary.Attributes["version"])
		if err != nil {
			return fmt.Errorf("invalid version in state: %w", err)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}
		ctx := context.Background()

		links, err := client.ListResources(ctx, &fastly.ListResourcesInput{ServiceID: serviceID, ServiceVersion: version})
		if err != nil {
			return fmt.Errorf("error listing resource links: %w", err)
		}
		var found *fastly.Resource
		for _, l := range links {
			if fastly.ToValue(l.LinkID) == rs.Primary.Attributes["link_id"] {
				found = l
				break
			}
		}
		if found == nil {
			return fmt.Errorf("resource link %s not found on service %s version %d", rs.Primary.Attributes["link_id"], serviceID, version)
		}
		if got := fastly.ToValue(found.Name); got != rs.Primary.Attributes["name"] {
			return fmt.Errorf("remote resource link name = %q, want %q", got, rs.Primary.Attributes["name"])
		}
		if got := fastly.ToValue(found.ResourceID); got != rs.Primary.Attributes["resource_id"] {
			return fmt.Errorf("remote resource link resource_id = %q, want %q", got, rs.Primary.Attributes["resource_id"])
		}

		versions, err := client.ListVersions(ctx, &fastly.ListVersionsInput{ServiceID: serviceID})
		if err != nil {
			return fmt.Errorf("error listing service versions: %w", err)
		}
		if len(versions) != wantVersions {
			return fmt.Errorf("service %s has %d versions, want %d", serviceID, len(versions), wantVersions)
		}
		for _, v := range versions {
			if fastly.ToValue(v.Active) {
				return fmt.Errorf("service %s version %d was unexpectedly activated", serviceID, fastly.ToValue(v.Number))
			}
		}

		return nil
	}
}

func cloneServiceVersionOutOfBand(serviceResourceName string, version int, lock bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[serviceResourceName]
		if !ok {
			return fmt.Errorf("service resource %s not found", serviceResourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		ctx := context.Background()
		cloned, err := client.CloneVersion(ctx, &fastly.CloneVersionInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
		})
		if err != nil {
			return fmt.Errorf("error cloning version: %w", err)
		}

		if lock {
			_, err = client.LockVersion(ctx, &fastly.LockVersionInput{
				ServiceID:      rs.Primary.ID,
				ServiceVersion: fastly.ToValue(cloned.Number),
			})
			if err != nil {
				return fmt.Errorf("error locking cloned version: %w", err)
			}
		}

		return nil
	}
}
