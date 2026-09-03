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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

var ngwafAccountListResourceTypes = []string{
	"fastly_ngwaf_country_list",
	"fastly_ngwaf_ip_list",
	"fastly_ngwaf_signal_list",
	"fastly_ngwaf_string_list",
	"fastly_ngwaf_wildcard_list",
}

type ngwafAccountListAcceptanceCase struct {
	name           string
	resourceName   string
	listType       string
	initialEntry   string
	updatedEntries []string
	basicBlock     string
	updatedBlock   string
}

func TestAccFastlyNGWAFIPList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFAccountListLifecycle(t, ngwafAccountListAcceptanceCase{
		name:           "ip",
		resourceName:   "fastly_ngwaf_ip_list.test",
		listType:       "ip",
		initialEntry:   "10.0.0.1",
		updatedEntries: []string{"192.168.1.1", "172.16.0.1"},
		basicBlock:     "ngwaf_ip_list_basic.tf",
		updatedBlock:   "ngwaf_ip_list_updated.tf",
	})
}

func TestAccFastlyNGWAFStringList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFAccountListLifecycle(t, ngwafAccountListAcceptanceCase{
		name:           "string",
		resourceName:   "fastly_ngwaf_string_list.test",
		listType:       "string",
		initialEntry:   "admin",
		updatedEntries: []string{"admin", "login"},
		basicBlock:     "ngwaf_string_list_basic.tf",
		updatedBlock:   "ngwaf_string_list_updated.tf",
	})
}

func TestAccFastlyNGWAFWildcardList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFAccountListLifecycle(t, ngwafAccountListAcceptanceCase{
		name:           "wildcard",
		resourceName:   "fastly_ngwaf_wildcard_list.test",
		listType:       "wildcard",
		initialEntry:   "admin-*",
		updatedEntries: []string{"admin-*", "login-*"},
		basicBlock:     "ngwaf_wildcard_list_basic.tf",
		updatedBlock:   "ngwaf_wildcard_list_updated.tf",
	})
}

func TestAccFastlyNGWAFCountryList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFAccountListLifecycle(t, ngwafAccountListAcceptanceCase{
		name:           "country",
		resourceName:   "fastly_ngwaf_country_list.test",
		listType:       "country",
		initialEntry:   "US",
		updatedEntries: []string{"US", "CA"},
		basicBlock:     "ngwaf_country_list_basic.tf",
		updatedBlock:   "ngwaf_country_list_updated.tf",
	})
}

func TestAccFastlyNGWAFSignalList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFAccountListLifecycle(t, ngwafAccountListAcceptanceCase{
		name:           "signal",
		resourceName:   "fastly_ngwaf_signal_list.test",
		listType:       "signal",
		initialEntry:   "XSS",
		updatedEntries: []string{"XSS", "SQLI"},
		basicBlock:     "ngwaf_signal_list_basic.tf",
		updatedBlock:   "ngwaf_signal_list_updated.tf",
	})
}

func testAccFastlyNGWAFAccountListLifecycle(t *testing.T, tc ngwafAccountListAcceptanceCase) {
	t.Helper()

	suffix := acctest.RandString(10)
	listName := fmt.Sprintf("tf-%s-list-%s", tc.name, suffix)
	listDescription := fmt.Sprintf("Initial %s list", tc.name)
	updatedListDescription := fmt.Sprintf("Updated %s list", tc.name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFAccountListDestroy,
		Steps: []resource.TestStep{
			{
				Config: configNGWAFAccountList(tc.basicBlock, listName, listDescription, []string{tc.initialEntry}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tc.resourceName, "name", listName),
					resource.TestCheckResourceAttr(tc.resourceName, "description", listDescription),
					resource.TestCheckResourceAttr(tc.resourceName, "entries.0", tc.initialEntry),
					resource.TestCheckResourceAttrSet(tc.resourceName, "id"),
					resource.TestCheckResourceAttrSet(tc.resourceName, "reference_id"),
					CheckNGWAFAccountListExists(tc.resourceName, tc.listType, listName, listDescription, []string{tc.initialEntry}),
				),
			},
			{
				Config: configNGWAFAccountList(tc.updatedBlock, listName, updatedListDescription, tc.updatedEntries),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(tc.resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tc.resourceName, "name", listName),
					resource.TestCheckResourceAttr(tc.resourceName, "description", updatedListDescription),
					CheckNGWAFAccountListEntries(tc.resourceName, tc.updatedEntries),
					CheckNGWAFAccountListExists(tc.resourceName, tc.listType, listName, updatedListDescription, tc.updatedEntries),
				),
			},
			{
				Config:   configNGWAFAccountList(tc.updatedBlock, listName, updatedListDescription, tc.updatedEntries),
				PlanOnly: true,
			},
			{
				ResourceName:      tc.resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyNGWAFLists_dataSource(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	names := map[string]string{
		"ip":       fmt.Sprintf("tf-ip-list-%s", suffix),
		"string":   fmt.Sprintf("tf-string-list-%s", suffix),
		"wildcard": fmt.Sprintf("tf-wildcard-list-%s", suffix),
		"country":  fmt.Sprintf("tf-country-list-%s", suffix),
		"signal":   fmt.Sprintf("tf-signal-list-%s", suffix),
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFAccountListDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFListsDataSource(names),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_lists.test", "id"),
					checkNGWAFListsDataSourceHasList("fastly_ngwaf_ip_list.ip", "ip"),
					checkNGWAFListsDataSourceHasList("fastly_ngwaf_string_list.string", "string"),
					checkNGWAFListsDataSourceHasList("fastly_ngwaf_wildcard_list.wildcard", "wildcard"),
					checkNGWAFListsDataSourceHasList("fastly_ngwaf_country_list.country", "country"),
					checkNGWAFListsDataSourceHasList("fastly_ngwaf_signal_list.signal", "signal"),
				),
			},
		},
	})
}

func configNGWAFAccountList(blockFile, listName, listDescription string, entries []string) string {
	return RenderBlock("internal/acceptance_tests/blocks/"+blockFile, map[string]string{
		"LIST_NAME":                listName,
		"LIST_DESCRIPTION":         listDescription,
		"LIST_DESCRIPTION_UPDATED": listDescription,
		"LIST_ENTRIES":             terraformAccountStringList(entries),
		"LIST_ENTRIES_UPDATED":     terraformAccountStringList(entries),
	})
}

func ConfigNGWAFListsDataSource(names map[string]string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_lists_data_source.tf", map[string]string{
		"IP_LIST_NAME":       names["ip"],
		"STRING_LIST_NAME":   names["string"],
		"WILDCARD_LIST_NAME": names["wildcard"],
		"COUNTRY_LIST_NAME":  names["country"],
		"SIGNAL_LIST_NAME":   names["signal"],
	})
}

func terraformAccountStringList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}

func CheckNGWAFAccountListEntries(resourceName string, expected []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		got := make([]string, 0, len(expected))
		for i := range expected {
			got = append(got, rs.Primary.Attributes[fmt.Sprintf("entries.%d", i)])
		}

		if !slices.Equal(got, expected) {
			return fmt.Errorf("%s entries = %v, want %v", resourceName, got, expected)
		}

		return nil
	}
}

func CheckNGWAFAccountListExists(resourceName, expectedType, expectedName, expectedDescription string, expectedEntries []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		list, err := lists.Get(context.Background(), client, &lists.GetInput{
			ListID: &rs.Primary.ID,
			Scope:  &scope.Scope{Type: scope.ScopeTypeAccount},
		})
		if err != nil {
			return fmt.Errorf("error reading NGWAF account list %s: %w", rs.Primary.ID, err)
		}

		if list.Type != expectedType {
			return fmt.Errorf("NGWAF account list type = %q, want %q", list.Type, expectedType)
		}
		if list.Name != expectedName {
			return fmt.Errorf("NGWAF account list name = %q, want %q", list.Name, expectedName)
		}
		if list.Description != expectedDescription {
			return fmt.Errorf("NGWAF account list description = %q, want %q", list.Description, expectedDescription)
		}
		if !slices.Equal(list.Entries, expectedEntries) {
			return fmt.Errorf("NGWAF account list entries = %v, want %v", list.Entries, expectedEntries)
		}
		if list.ReferenceID == "" {
			return fmt.Errorf("NGWAF account list %s has empty reference_id", rs.Primary.ID)
		}
		if scope.Type(list.Scope.Type) != scope.ScopeTypeAccount {
			return fmt.Errorf("NGWAF list scope type = %q, want %q", list.Scope.Type, scope.ScopeTypeAccount)
		}

		return nil
	}
}

func CheckNGWAFAccountListDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if !slices.Contains(ngwafAccountListResourceTypes, rs.Type) {
			continue
		}

		_, err := lists.Get(context.Background(), client, &lists.GetInput{
			ListID: &rs.Primary.ID,
			Scope:  &scope.Scope{Type: scope.ScopeTypeAccount},
		})
		if err == nil {
			return fmt.Errorf("NGWAF account list %s still exists after destroy", rs.Primary.ID)
		}
	}

	return nil
}

func checkNGWAFListsDataSourceHasList(resourceName, expectedType string) resource.TestCheckFunc {
	const dataSourceName = "data.fastly_ngwaf_lists.test"
	return func(s *terraform.State) error {
		listResource, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("data source not found: %s", dataSourceName)
		}

		count, err := strconv.Atoi(ds.Primary.Attributes["lists.#"])
		if err != nil {
			return fmt.Errorf("reading %s lists.#: %w", dataSourceName, err)
		}

		for i := range count {
			prefix := fmt.Sprintf("lists.%d.", i)
			if ds.Primary.Attributes[prefix+"id"] != listResource.Primary.ID {
				continue
			}

			if got := ds.Primary.Attributes[prefix+"type"]; got != expectedType {
				return fmt.Errorf("list %s reported type %q, want %q", listResource.Primary.ID, got, expectedType)
			}
			if got, want := ds.Primary.Attributes[prefix+"name"], listResource.Primary.Attributes["name"]; got != want {
				return fmt.Errorf("list %s reported name %q, want %q", listResource.Primary.ID, got, want)
			}
			if got := ds.Primary.Attributes[prefix+"reference_id"]; got == "" {
				return fmt.Errorf("list %s reported an empty reference_id", listResource.Primary.ID)
			}
			return nil
		}

		return fmt.Errorf("list %s not reported by %s (%d lists listed)", listResource.Primary.ID, dataSourceName, count)
	}
}
