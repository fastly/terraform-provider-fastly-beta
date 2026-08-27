package acceptancetests

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyConfigStoreItems_lifecycle(t *testing.T) {
	storeName := fmt.Sprintf("tf_test_configstore_items_%s", acctest.RandString(10))

	managedV1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	managedV2 := map[string]string{
		"key1": "value1_updated",
		"key3": "value3",
	}
	external := map[string]string{
		"external": "outside-terraform",
	}
	externalWithSecond := map[string]string{
		"external":   "outside-terraform",
		"external-2": "also-outside-terraform",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckConfigStoreDestroy,
		Steps: []resource.TestStep{
			{
				// Seed an item outside Terraform before the items resource exists.
				// Creating fastly_configstore_items must leave that item untouched.
				Config: ConfigConfigStore(storeName),
				Check: InsertConfigStoreItem(
					"fastly_configstore.store",
					"external",
					"outside-terraform",
				),
			},
			{
				Config: ConfigConfigStoreItems(storeName, managedV1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_configstore_items.items", "id"),
					resource.TestCheckResourceAttrPair("fastly_configstore_items.items", "store_id", "fastly_configstore.store", "id"),
					resource.TestCheckResourceAttr("fastly_configstore_items.items", "items.%", "2"),
					resource.TestCheckResourceAttr("fastly_configstore_items.items", "items.key1", "value1"),
					resource.TestCheckResourceAttr("fastly_configstore_items.items", "items.key2", "value2"),
					CheckConfigStoreItemsRemoteState("fastly_configstore.store", mergeStringMaps(external, managedV1)),
				),
			},
			{
				// Exercise create, update, and delete in one reconciliation:
				// key1 changes, key2 is removed, and key3 is added.
				Config: ConfigConfigStoreItems(storeName, managedV2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_configstore_items.items", "items.%", "2"),
					resource.TestCheckResourceAttr("fastly_configstore_items.items", "items.key1", "value1_updated"),
					resource.TestCheckResourceAttr("fastly_configstore_items.items", "items.key3", "value3"),
					CheckConfigStoreItemsRemoteState("fastly_configstore.store", mergeStringMaps(external, managedV2)),
				),
			},
			{
				Config:             ConfigConfigStoreItems(storeName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Introduce out-of-band drift on a Terraform-managed key.
				// The Check intentionally mutates remote state after apply, so the
				// test step must expect the automatic post-apply refresh plan to
				// be non-empty.
				Config:             ConfigConfigStoreItems(storeName, managedV2),
				ExpectNonEmptyPlan: true,
				Check: UpdateConfigStoreItem(
					"fastly_configstore.store",
					"key1",
					"drifted-outside-terraform",
				),
			},
			{
				// Managed drift must be visible in the Terraform plan.
				Config:             ConfigConfigStoreItems(storeName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Applying the unchanged configuration repairs managed drift.
				Config: ConfigConfigStoreItems(storeName, managedV2),
				Check: CheckConfigStoreItemsRemoteState(
					"fastly_configstore.store",
					mergeStringMaps(external, managedV2),
				),
			},
			{
				// Add another key outside Terraform. Because it is not declared
				// in items, the resource must leave it alone.
				Config: ConfigConfigStoreItems(storeName, managedV2),
				Check: InsertConfigStoreItem(
					"fastly_configstore.store",
					"external-2",
					"also-outside-terraform",
				),
			},
			{
				// Unmanaged external additions must not create Terraform drift.
				Config:             ConfigConfigStoreItems(storeName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Removing only the items resource deletes Terraform-owned keys
				// while preserving externally managed items in the Config Store.
				Config: ConfigConfigStore(storeName),
				Check: CheckConfigStoreItemsRemoteState(
					"fastly_configstore.store",
					externalWithSecond,
				),
			},
		},
	})
}

func TestAccFastlyConfigStoreItems_import(t *testing.T) {
	storeName := fmt.Sprintf("tf_test_configstore_items_import_%s", acctest.RandString(10))
	items := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckConfigStoreDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigConfigStoreItems(storeName, items),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_configstore_items.items", "items.%", "2"),
					CheckConfigStoreItemsRemoteState("fastly_configstore.store", items),
				),
			},
			{
				ResourceName:      "fastly_configstore_items.items",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:             ConfigConfigStoreItems(storeName, items),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func InsertConfigStoreItem(resourceName, key, value string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		storeID, err := configStoreIDFromState(s, resourceName)
		if err != nil {
			return err
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		_, err = client.CreateConfigStoreItem(context.Background(), &fastly.CreateConfigStoreItemInput{
			StoreID: storeID,
			Key:     key,
			Value:   value,
		})
		if err != nil {
			return fmt.Errorf("error creating Config Store item %q: %w", key, err)
		}

		return nil
	}
}

func UpdateConfigStoreItem(resourceName, key, value string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		storeID, err := configStoreIDFromState(s, resourceName)
		if err != nil {
			return err
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		err = client.BatchModifyConfigStoreItems(context.Background(), &fastly.BatchModifyConfigStoreItemsInput{
			StoreID: storeID,
			Items: []*fastly.BatchConfigStoreItem{
				{
					Operation: fastly.UpdateBatchOperation,
					ItemKey:   key,
					ItemValue: value,
				},
			},
		})
		if err != nil {
			return fmt.Errorf("error updating Config Store item %q: %w", key, err)
		}

		return nil
	}
}

func CheckConfigStoreItemExists(resourceName, key, expectedValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		items, err := configStoreItemsRemoteState(s, resourceName)
		if err != nil {
			return err
		}

		value, ok := items[key]
		if !ok {
			return fmt.Errorf("Config Store item %q was not found", key)
		}
		if value != expectedValue {
			return fmt.Errorf("unexpected Config Store item value: got %q, want %q", value, expectedValue)
		}

		return nil
	}
}

func CheckConfigStoreItemsRemoteState(resourceName string, want map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got, err := configStoreItemsRemoteState(s, resourceName)
		if err != nil {
			return err
		}

		if !maps.Equal(got, want) {
			return fmt.Errorf("unexpected Config Store items:\ngot:  %#v\nwant: %#v", got, want)
		}

		return nil
	}
}

func configStoreItemsRemoteState(s *terraform.State, resourceName string) (map[string]string, error) {
	storeID, err := configStoreIDFromState(s, resourceName)
	if err != nil {
		return nil, err
	}

	client, err := NewFastlyClient()
	if err != nil {
		return nil, fmt.Errorf("error creating Fastly client: %w", err)
	}

	items, err := client.ListConfigStoreItems(context.Background(), &fastly.ListConfigStoreItemsInput{
		StoreID: storeID,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing Config Store items: %w", err)
	}

	result := make(map[string]string, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result[item.Key] = item.Value
	}

	return result, nil
}

func configStoreIDFromState(s *terraform.State, resourceName string) (string, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return "", fmt.Errorf("not found: %s", resourceName)
	}
	if rs.Primary.ID == "" {
		return "", fmt.Errorf("%s has no ID", resourceName)
	}
	return rs.Primary.ID, nil
}

func mergeStringMaps(mapsToMerge ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, values := range mapsToMerge {
		for key, value := range values {
			result[key] = value
		}
	}
	return result
}
