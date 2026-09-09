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

	"github.com/fastly/go-fastly/v17/fastly/objectstorage/accesskeys"
)

func TestAccFastlyObjectStorageAccessKey_basic(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	description := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	resourceName := "fastly_object_storage_access_keys.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckObjectStorageAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigObjectStorageAccessKey(description, accesskeys.ReadWriteObject, []string{"bucket1", "bucket2"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectStorageAccessKeyExists(),
					resource.TestCheckResourceAttr(resourceName, "description", description),
					resource.TestCheckResourceAttr(resourceName, "permission", accesskeys.ReadWriteObject),
					resource.TestCheckResourceAttr(resourceName, "buckets.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "buckets.0", "bucket1"),
					resource.TestCheckResourceAttr(resourceName, "buckets.1", "bucket2"),
					resource.TestCheckResourceAttrSet(resourceName, "access_key_id"),
					resource.TestCheckResourceAttrSet(resourceName, "authentication.secret_key"),
					resource.TestCheckResourceAttrPair(resourceName, "id", resourceName, "access_key_id"),
				),
			},
			{
				// authentication.secret_key is only ever returned by the create response, so
				// it cannot be verified against a value the import read would repopulate.
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"authentication"},
			},
		},
	})
}

func TestAccFastlyObjectStorageAccessKey_noBuckets(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	description := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	resourceName := "fastly_object_storage_access_keys.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckObjectStorageAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigObjectStorageAccessKey(description, accesskeys.ReadOnlyAdmin, nil),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectStorageAccessKeyExists(),
					resource.TestCheckResourceAttr(resourceName, "permission", accesskeys.ReadOnlyAdmin),
					resource.TestCheckResourceAttr(resourceName, "buckets.#", "0"),
				),
			},
		},
	})
}

func TestAccFastlyObjectStorageAccessKey_replace(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	description1 := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	description2 := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	resourceName := "fastly_object_storage_access_keys.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckObjectStorageAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigObjectStorageAccessKey(description1, accesskeys.ReadWriteObject, []string{"bucket1"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", description1),
					testAccCheckObjectStorageAccessKeyExists(),
				),
			},
			{
				// description, permission, and buckets all require replacement: no
				// in-place update path exists.
				Config: ConfigObjectStorageAccessKey(description2, accesskeys.ReadOnlyObjects, []string{"bucket2"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "description", description2),
					resource.TestCheckResourceAttr(resourceName, "permission", accesskeys.ReadOnlyObjects),
					testAccCheckObjectStorageAccessKeyExists(),
				),
			},
		},
	})
}

func TestAccFastlyObjectStorageAccessKey_invalidPermission(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	description := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigObjectStorageAccessKey(description, "not-a-real-permission", nil),
				ExpectError: regexp.MustCompile(`Attribute permission value must be one of`),
			},
		},
	})
}

func testAccCheckObjectStorageAccessKeyExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		resourceName := "fastly_object_storage_access_keys.test"
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		_, err = accesskeys.Get(context.Background(), client, &accesskeys.GetInput{AccessKeyID: &rs.Primary.ID})
		return err
	}
}

func CheckObjectStorageAccessKeyDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_object_storage_access_keys" {
			continue
		}

		_, err := accesskeys.Get(context.Background(), client, &accesskeys.GetInput{AccessKeyID: &rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Object Storage Access Key was destroyed: %w", err)
		}

		return fmt.Errorf("Object Storage Access Key %s still exists", rs.Primary.ID)
	}

	return nil
}
