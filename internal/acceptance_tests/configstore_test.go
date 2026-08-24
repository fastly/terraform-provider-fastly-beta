package acceptancetests

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/terraform-provider-fastly/internal/errors"
)

func TestAccFastlyConfigStore_lifecycle(t *testing.T) {
	storeName := fmt.Sprintf("tf_test_configstore_%s", acctest.RandString(10))
	storeNameUpdated := fmt.Sprintf("tf_test_configstore_updated_%s", acctest.RandString(10))
	var storeID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckConfigStoreDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigConfigStore(storeName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_configstore.store", "name", storeName),
					resource.TestCheckResourceAttrSet("fastly_configstore.store", "id"),
					CaptureConfigStoreID("fastly_configstore.store", &storeID),
					CheckConfigStoreRemoteState("fastly_configstore.store", storeName),
					InsertConfigStoreItem("fastly_configstore.store", "test-key", "test-value"),
					CheckConfigStoreItemExists("fastly_configstore.store", "test-key", "test-value"),
				),
			},
			{
				// The current Fastly API supports renaming Config Stores in place.
				Config: ConfigConfigStore(storeNameUpdated),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_configstore.store", "name", storeNameUpdated),
					CheckConfigStoreIDUnchanged("fastly_configstore.store", &storeID),
					CheckConfigStoreRemoteState("fastly_configstore.store", storeNameUpdated),
					CheckConfigStoreItemExists("fastly_configstore.store", "test-key", "test-value"),
				),
			},
			{
				Config:   ConfigConfigStore(storeNameUpdated),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_configstore.store",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:   ConfigConfigStore(storeNameUpdated),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyConfigStore_computeAutoResourceLink(t *testing.T) {
	storeName := fmt.Sprintf("tf_test_configstore_%s", acctest.RandString(10))
	storeNameUpdated := fmt.Sprintf("tf_test_configstore_updated_%s", acctest.RandString(10))
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	linkName := fmt.Sprintf("config_store_link_%s", acctest.RandString(10))
	var storeID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAndConfigStoreDestroy("fastly_service_compute_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigConfigStoreWithComputeAutoResourceLink(storeName, serviceName, domainName, linkName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					resource.TestCheckResourceAttr("fastly_configstore.store", "name", storeName),
					resource.TestCheckResourceAttrSet("fastly_configstore.store", "id"),
					CaptureConfigStoreID("fastly_configstore.store", &storeID),
					CheckConfigStoreRemoteState("fastly_configstore.store", storeName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "resource_link.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "resource_link.0.name", linkName),
					resource.TestCheckResourceAttrPair("fastly_service_compute_auto.test", "resource_link.0.resource_id", "fastly_configstore.store", "id"),
					resource.TestCheckResourceAttrSet("fastly_service_compute_auto.test", "resource_link.0.link_id"),
					CheckComputeAutoResourceLinkRemoteState("fastly_service_compute_auto.test", "fastly_configstore.store", linkName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "1"),
				),
			},
			{
				// Rename the account-level Config Store while it is still linked. The store ID
				// must remain stable and the Compute service's resource link must still point to
				// the same ID without requiring a new service version.
				Config: ConfigConfigStoreWithComputeAutoResourceLink(storeNameUpdated, serviceName, domainName, linkName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					resource.TestCheckResourceAttr("fastly_configstore.store", "name", storeNameUpdated),
					CheckConfigStoreIDUnchanged("fastly_configstore.store", &storeID),
					CheckConfigStoreRemoteState("fastly_configstore.store", storeNameUpdated),
					resource.TestCheckResourceAttrPair("fastly_service_compute_auto.test", "resource_link.0.resource_id", "fastly_configstore.store", "id"),
					CheckComputeAutoResourceLinkRemoteState("fastly_service_compute_auto.test", "fastly_configstore.store", linkName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "1"),
				),
			},
			{
				ResourceName:      "fastly_configstore.store",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Config Stores must be unlinked from services before deletion. Keep the store
				// declared while removing the Compute resource_link, so the unlink settles in
				// its own service-version update before the final step deletes the store.
				Config: ConfigConfigStoreWithComputeAutoUnlinked(storeNameUpdated, serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute_auto.test"),
					resource.TestCheckResourceAttrSet("fastly_configstore.store", "id"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "resource_link.#", "0"),
					CheckComputeAutoResourceLinkAbsent("fastly_service_compute_auto.test", linkName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "managed_version", "2"),
				),
			},
			{
				// Now that the Config Store has been unlinked, deleting it is safe.
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

func TestAccFastlyDataSourceConfigStores(t *testing.T) {
	h := acctest.RandString(10)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckConfigStoreDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigConfigStoresDataSource(h),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_configstores.example", "id"),
					CheckConfigStoresDataSourceContains(
						"data.fastly_configstores.example",
						[]string{fmt.Sprintf("tf_%s", h)},
					),
				),
			},
			{
				Config:   ConfigConfigStoresDataSource(h),
				PlanOnly: true,
			},
		},
	})
}

func CaptureConfigStoreID(resourceName string, target *string) resource.TestCheckFunc {
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

func CheckConfigStoreIDUnchanged(resourceName string, expected *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}
		if *expected == "" {
			return fmt.Errorf("expected Config Store ID was not captured")
		}
		if rs.Primary.ID != *expected {
			return fmt.Errorf("%s changed identity: got %q, want %q", resourceName, rs.Primary.ID, *expected)
		}

		return nil
	}
}

func CheckConfigStoreRemoteState(resourceName, expectedName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		store, err := client.GetConfigStore(context.Background(), &fastly.GetConfigStoreInput{
			StoreID: rs.Primary.ID,
		})
		if err != nil {
			return fmt.Errorf("error reading Config Store %q: %w", rs.Primary.ID, err)
		}
		if store.Name != expectedName {
			return fmt.Errorf("unexpected Config Store name: got %q, want %q", store.Name, expectedName)
		}

		return nil
	}
}

func CheckComputeAutoResourceLinkRemoteState(serviceResourceName, configStoreResourceName, expectedLinkName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		link, expectedResourceID, err := findComputeAutoResourceLink(s, serviceResourceName, configStoreResourceName, expectedLinkName)
		if err != nil {
			return err
		}

		if got := fastly.ToValue(link.ResourceID); got != expectedResourceID {
			return fmt.Errorf("resource link %q points at resource_id %q, want %q", expectedLinkName, got, expectedResourceID)
		}

		return nil
	}
}

func CheckComputeAutoResourceLinkAbsent(serviceResourceName, linkName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		link, serviceState, version, err := findComputeAutoResourceLinkByName(s, serviceResourceName, linkName)
		if err != nil {
			return err
		}
		if link != nil {
			return fmt.Errorf("resource link %q still exists on service %q version %d", linkName, serviceState.Primary.ID, version)
		}

		return nil
	}
}

func findComputeAutoResourceLink(s *terraform.State, serviceResourceName, configStoreResourceName, expectedLinkName string) (*fastly.Resource, string, error) {
	link, serviceState, version, err := findComputeAutoResourceLinkByName(s, serviceResourceName, expectedLinkName)
	if err != nil {
		return nil, "", err
	}
	if link == nil {
		return nil, "", fmt.Errorf("resource link %q was not found on service %q version %d", expectedLinkName, serviceState.Primary.ID, version)
	}

	storeState, ok := s.RootModule().Resources[configStoreResourceName]
	if !ok {
		return nil, "", fmt.Errorf("not found: %s", configStoreResourceName)
	}

	return link, storeState.Primary.ID, nil
}

func findComputeAutoResourceLinkByName(s *terraform.State, serviceResourceName, linkName string) (*fastly.Resource, *terraform.ResourceState, int, error) {
	serviceState, version, err := serviceAndVersion(s, serviceResourceName)
	if err != nil {
		return nil, nil, 0, err
	}

	client, err := NewFastlyClient()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("error creating Fastly client: %w", err)
	}

	links, err := client.ListResources(context.Background(), &fastly.ListResourcesInput{
		ServiceID:      serviceState.Primary.ID,
		ServiceVersion: version,
	})
	if err != nil {
		return nil, nil, 0, fmt.Errorf("error listing resource links for service %q version %d: %w", serviceState.Primary.ID, version, err)
	}

	for _, link := range links {
		if fastly.ToValue(link.Name) == linkName {
			return link, serviceState, version, nil
		}
	}

	return nil, serviceState, version, nil
}

func serviceAndVersion(s *terraform.State, serviceResourceName string) (*terraform.ResourceState, int, error) {
	serviceState, ok := s.RootModule().Resources[serviceResourceName]
	if !ok {
		return nil, 0, fmt.Errorf("not found: %s", serviceResourceName)
	}

	version, err := strconv.Atoi(serviceState.Primary.Attributes["active_version"])
	if err != nil {
		return nil, 0, fmt.Errorf("error parsing active_version: %w", err)
	}

	return serviceState, version, nil
}

func InsertConfigStoreItem(resourceName, key, value string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		_, err = client.CreateConfigStoreItem(context.Background(), &fastly.CreateConfigStoreItemInput{
			StoreID: rs.Primary.ID,
			Key:     key,
			Value:   value,
		})
		if err != nil {
			return fmt.Errorf("error creating Config Store item: %w", err)
		}

		return nil
	}
}

func CheckConfigStoreItemExists(resourceName, key, expectedValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		items, err := client.ListConfigStoreItems(context.Background(), &fastly.ListConfigStoreItemsInput{
			StoreID: rs.Primary.ID,
		})
		if err != nil {
			return fmt.Errorf("error listing Config Store items: %w", err)
		}

		for _, item := range items {
			if item != nil && item.Key == key {
				if item.Value != expectedValue {
					return fmt.Errorf("unexpected Config Store item value: got %q, want %q", item.Value, expectedValue)
				}
				return nil
			}
		}

		return fmt.Errorf("Config Store item %q was not found", key)
	}
}

func CheckConfigStoresDataSourceContains(resourceName string, want []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		var found int
		var got []string

		// stores is a set because the API does not guarantee Config Store order.
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

func CheckConfigStoreDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_configstore" {
			continue
		}

		_, err := client.GetConfigStore(context.Background(), &fastly.GetConfigStoreInput{
			StoreID: rs.Primary.ID,
		})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Config Store %q was destroyed: %w", rs.Primary.ID, err)
		}

		return fmt.Errorf("Config Store %s still exists", rs.Primary.ID)
	}

	return nil
}

func CheckServiceAndConfigStoreDestroy(serviceResourceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := CheckConfigStoreDestroy(s); err != nil {
			return err
		}
		return CheckServiceDestroy(serviceResourceType)(s)
	}
}
