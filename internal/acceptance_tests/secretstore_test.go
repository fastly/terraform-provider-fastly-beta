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

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
)

func TestAccFastlySecretStore_basic(t *testing.T) {
	t.Parallel()
	storeName := fmt.Sprintf("tf_test_secretstore_%s", acctest.RandString(10))
	storeNameUpdated := fmt.Sprintf("tf_test_secretstore_updated_%s", acctest.RandString(10))
	var storeID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckSecretStoreDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigSecretStore(storeName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_secretstore.store", "name", storeName),
					resource.TestCheckResourceAttrSet("fastly_secretstore.store", "id"),
					CaptureSecretStoreID("fastly_secretstore.store", &storeID),
					CheckSecretStoreRemoteState("fastly_secretstore.store", storeName),
				),
			},
			{
				ResourceName:      "fastly_secretstore.store",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// There is no API endpoint to rename a Secret Store, so changing the name
				// must force replacement, discarding the previous store and its ID.
				Config: ConfigSecretStore(storeNameUpdated),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_secretstore.store", "name", storeNameUpdated),
					CheckSecretStoreIDChanged("fastly_secretstore.store", &storeID),
					CheckSecretStoreRemoteState("fastly_secretstore.store", storeNameUpdated),
				),
			},
		},
	})
}

func TestAccFastlySecretStore_computeAutoResourceLink(t *testing.T) {
	t.Parallel()
	storeName := fmt.Sprintf("tf_test_secretstore_%s", acctest.RandString(10))
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	linkName := fmt.Sprintf("secret_store_link_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndSecretStoreDestroy("fastly_service_compute_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigSecretStoreWithComputeAutoResourceLink(storeName, serviceName, domainName, linkName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					resource.TestCheckResourceAttr("fastly_secretstore.store", "name", storeName),
					resource.TestCheckResourceAttrSet("fastly_secretstore.store", "id"),
					CheckSecretStoreRemoteState("fastly_secretstore.store", storeName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "resource_link.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "resource_link.0.name", linkName),
					resource.TestCheckResourceAttrPair("fastly_service_compute_auto.test", "resource_link.0.resource_id", "fastly_secretstore.store", "id"),
					resource.TestCheckResourceAttrSet("fastly_service_compute_auto.test", "resource_link.0.link_id"),
					CheckComputeAutoResourceLinkRemoteState("fastly_service_compute_auto.test", "fastly_secretstore.store", linkName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "1"),
				),
			},
			{
				ResourceName:      "fastly_secretstore.store",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Secret Stores must be unlinked from services before deletion. Keep the store
				// declared while removing the Compute resource_link, so the unlink settles in
				// its own service-version update before the final step deletes the store.
				Config: ConfigSecretStoreWithComputeAutoUnlinked(storeName, serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					resource.TestCheckResourceAttrSet("fastly_secretstore.store", "id"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "resource_link.#", "0"),
					CheckComputeAutoResourceLinkAbsent("fastly_service_compute_auto.test", linkName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "2"),
				),
			},
			{
				// Now that the Secret Store has been unlinked, deleting it is safe.
				Config: ConfigComputeAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "resource_link.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceSecretStores(t *testing.T) {
	t.Parallel()
	h := acctest.RandString(10)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckSecretStoreDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigSecretStoresDataSource(h),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_secretstores.example", "id"),
					CheckSecretStoresDataSourceContains(
						"data.fastly_secretstores.example",
						[]string{fmt.Sprintf("tf_%s", h)},
					),
				),
			},
			{
				Config:   ConfigSecretStoresDataSource(h),
				PlanOnly: true,
			},
		},
	})
}

func CaptureSecretStoreID(resourceName string, target *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("%s has no ID", resourceName)
		}

		*target = rs.Primary.ID
		return nil
	}
}

func CheckSecretStoreIDChanged(resourceName string, previous *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}
		if *previous == "" {
			return fmt.Errorf("previous Secret Store ID was not captured")
		}
		if rs.Primary.ID == *previous {
			return fmt.Errorf("%s kept the same ID (%q) after a name change, expected replacement", resourceName, rs.Primary.ID)
		}

		return nil
	}
}

func CheckSecretStoreRemoteState(resourceName, expectedName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		store, err := client.GetSecretStore(context.Background(), &fastly.GetSecretStoreInput{
			StoreID: rs.Primary.ID,
		})
		if err != nil {
			return fmt.Errorf("error reading Secret Store %q: %w", rs.Primary.ID, err)
		}
		if store.Name != expectedName {
			return fmt.Errorf("unexpected Secret Store name: got %q, want %q", store.Name, expectedName)
		}

		return nil
	}
}

func CheckSecretStoresDataSourceContains(resourceName string, want []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		var found int
		var got []string

		// stores is a set because the API does not guarantee Secret Store order.
		for key, value := range rs.Primary.Attributes {
			if !strings.HasPrefix(key, "stores.") || !strings.HasSuffix(key, ".name") {
				continue
			}

			got = append(got, value)
			if slices.Contains(want, value) {
				found++
			}
		}

		if found != len(want) {
			return fmt.Errorf("expected data source to contain %v, got %v", want, got)
		}

		return nil
	}
}

func CheckSecretStoreDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_secretstore" {
			continue
		}

		_, err := client.GetSecretStore(context.Background(), &fastly.GetSecretStoreInput{
			StoreID: rs.Primary.ID,
		})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Secret Store %q was destroyed: %w", rs.Primary.ID, err)
		}

		return fmt.Errorf("Secret Store %s still exists", rs.Primary.ID)
	}

	return nil
}

func CheckServiceAndSecretStoreDestroy(serviceResourceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := CheckSecretStoreDestroy(s); err != nil {
			return err
		}
		return CheckServiceDestroy(serviceResourceType)(s)
	}
}
