package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyServiceDictionaryItems_lifecycle(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	dictionaryName := fmt.Sprintf("dict_%s", acctest.RandString(10))

	managedV1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	managedV2 := map[string]string{
		"key1": "value1_updated",
		"key3": "value3",
	}
	// AddDictionaryItem seeds this fixed key/value directly through the API, simulating an
	// item managed outside Terraform.
	external := map[string]string{
		"test-key": "test-value",
	}
	externalWithSecond := map[string]string{
		"test-key":   "test-value",
		"external-2": "also-outside-terraform",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				// Seed an item outside Terraform before the items resource exists.
				// Creating fastly_service_dictionary_items must leave that item untouched.
				Config: ConfigCDNAutoWithDictionary(serviceName, domainName, dictionaryName),
				Check:  AddDictionaryItem("fastly_service_cdn_auto.test", "dictionary.0"),
			},
			{
				Config: ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, managedV1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_service_dictionary_items.items", "id"),
					resource.TestCheckResourceAttrPair("fastly_service_dictionary_items.items", "service_id", "fastly_service_cdn_auto.test", "id"),
					resource.TestCheckResourceAttrPair("fastly_service_dictionary_items.items", "dictionary_id", "fastly_service_cdn_auto.test", "dictionary.0.dictionary_id"),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items.items", "items.%", "2"),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items.items", "items.key1", "value1"),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items.items", "items.key2", "value2"),
					CheckServiceDictionaryItemsRemoteState("fastly_service_dictionary_items.items", mergeStringMaps(external, managedV1)),
				),
			},
			{
				// Exercise create, update, and delete in one reconciliation:
				// key1 changes, key2 is removed, and key3 is added.
				Config: ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, managedV2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_dictionary_items.items", "items.%", "2"),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items.items", "items.key1", "value1_updated"),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items.items", "items.key3", "value3"),
					CheckServiceDictionaryItemsRemoteState("fastly_service_dictionary_items.items", mergeStringMaps(external, managedV2)),
				),
			},
			{
				Config:             ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Introduce out-of-band drift on a Terraform-managed key. The Check
				// intentionally mutates remote state after apply, so the test step must
				// expect the automatic post-apply refresh plan to be non-empty.
				Config:             ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, managedV2),
				ExpectNonEmptyPlan: true,
				Check:              UpdateDictionaryItemDirect("fastly_service_dictionary_items.items", "key1", "drifted-outside-terraform"),
			},
			{
				// Managed drift must be visible in the Terraform plan.
				Config:             ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Applying the unchanged configuration repairs managed drift.
				Config: ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, managedV2),
				Check: CheckServiceDictionaryItemsRemoteState(
					"fastly_service_dictionary_items.items",
					mergeStringMaps(external, managedV2),
				),
			},
			{
				// Add another key outside Terraform. Because it is not declared in
				// items, the resource must leave it alone.
				Config: ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, managedV2),
				Check:  InsertDictionaryItem("fastly_service_dictionary_items.items", "external-2", "also-outside-terraform"),
			},
			{
				// Unmanaged external additions must not create Terraform drift.
				Config:             ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Removing only the items resource deletes Terraform-owned keys while
				// preserving externally managed items in the Dictionary.
				Config: ConfigCDNAutoWithDictionary(serviceName, domainName, dictionaryName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckDictionaryRemoteState("fastly_service_cdn_auto.test", "dictionary.0", externalWithSecond),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionaryItems_import(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	dictionaryName := fmt.Sprintf("dict_%s", acctest.RandString(10))
	items := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, items),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_dictionary_items.items", "items.%", "2"),
					CheckServiceDictionaryItemsRemoteState("fastly_service_dictionary_items.items", items),
				),
			},
			{
				ResourceName:      "fastly_service_dictionary_items.items",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:             ConfigServiceDictionaryItems(serviceName, domainName, dictionaryName, items),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}
