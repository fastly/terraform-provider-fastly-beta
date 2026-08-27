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

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

type ngwafWorkspaceListAcceptanceCase struct {
	name           string
	resourceType   string
	resourceName   string
	listType       string
	initialEntry   string
	updatedEntries []string
	basicBlock     string
	updatedBlock   string
}

func TestAccFastlyNGWAFWorkspaceIPList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFWorkspaceListLifecycle(t, ngwafWorkspaceListAcceptanceCase{
		name:           "ip",
		resourceType:   "fastly_ngwaf_workspace_ip_list",
		resourceName:   "fastly_ngwaf_workspace_ip_list.test",
		listType:       "ip",
		initialEntry:   "10.0.0.1",
		updatedEntries: []string{"192.168.1.1", "172.16.0.1"},
		basicBlock:     "ngwaf_workspace_ip_list_basic.tf",
		updatedBlock:   "ngwaf_workspace_ip_list_updated.tf",
	})
}

func TestAccFastlyNGWAFWorkspaceStringList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFWorkspaceListLifecycle(t, ngwafWorkspaceListAcceptanceCase{
		name:           "string",
		resourceType:   "fastly_ngwaf_workspace_string_list",
		resourceName:   "fastly_ngwaf_workspace_string_list.test",
		listType:       "string",
		initialEntry:   "admin",
		updatedEntries: []string{"admin", "login"},
		basicBlock:     "ngwaf_workspace_string_list_basic.tf",
		updatedBlock:   "ngwaf_workspace_string_list_updated.tf",
	})
}

func TestAccFastlyNGWAFWorkspaceWildcardList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFWorkspaceListLifecycle(t, ngwafWorkspaceListAcceptanceCase{
		name:           "wildcard",
		resourceType:   "fastly_ngwaf_workspace_wildcard_list",
		resourceName:   "fastly_ngwaf_workspace_wildcard_list.test",
		listType:       "wildcard",
		initialEntry:   "admin-*",
		updatedEntries: []string{"admin-*", "login-*"},
		basicBlock:     "ngwaf_workspace_wildcard_list_basic.tf",
		updatedBlock:   "ngwaf_workspace_wildcard_list_updated.tf",
	})
}

func TestAccFastlyNGWAFWorkspaceCountryList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFWorkspaceListLifecycle(t, ngwafWorkspaceListAcceptanceCase{
		name:           "country",
		resourceType:   "fastly_ngwaf_workspace_country_list",
		resourceName:   "fastly_ngwaf_workspace_country_list.test",
		listType:       "country",
		initialEntry:   "US",
		updatedEntries: []string{"US", "CA"},
		basicBlock:     "ngwaf_workspace_country_list_basic.tf",
		updatedBlock:   "ngwaf_workspace_country_list_updated.tf",
	})
}

func TestAccFastlyNGWAFWorkspaceSignalList_lifecycle(t *testing.T) {
	t.Parallel()

	testAccFastlyNGWAFWorkspaceListLifecycle(t, ngwafWorkspaceListAcceptanceCase{
		name:           "signal",
		resourceType:   "fastly_ngwaf_workspace_signal_list",
		resourceName:   "fastly_ngwaf_workspace_signal_list.test",
		listType:       "signal",
		initialEntry:   "XSS",
		updatedEntries: []string{"XSS", "SQLI"},
		basicBlock:     "ngwaf_workspace_signal_list_basic.tf",
		updatedBlock:   "ngwaf_workspace_signal_list_updated.tf",
	})
}

func testAccFastlyNGWAFWorkspaceListLifecycle(t *testing.T, tc ngwafWorkspaceListAcceptanceCase) {
	t.Helper()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-%s-list-%s", tc.name, suffix)
	listName := fmt.Sprintf("tf-%s-list-%s", tc.name, suffix)
	listDescription := fmt.Sprintf("Initial %s list", tc.name)
	updatedListDescription := fmt.Sprintf("Updated %s list", tc.name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceListAndWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: configNGWAFWorkspaceList(tc.basicBlock, workspaceName, listName, listDescription, []string{tc.initialEntry}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "name", workspaceName),
					resource.TestCheckResourceAttr(tc.resourceName, "name", listName),
					resource.TestCheckResourceAttr(tc.resourceName, "description", listDescription),
					resource.TestCheckResourceAttr(tc.resourceName, "entries.0", tc.initialEntry),
					resource.TestCheckResourceAttrSet(tc.resourceName, "id"),
					resource.TestCheckResourceAttrSet(tc.resourceName, "reference_id"),
					CheckNGWAFWorkspaceListExists(tc.resourceName, tc.listType, listName, listDescription, []string{tc.initialEntry}),
				),
			},
			{
				Config: configNGWAFWorkspaceList(tc.updatedBlock, workspaceName, listName, updatedListDescription, tc.updatedEntries),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tc.resourceName, "name", listName),
					resource.TestCheckResourceAttr(tc.resourceName, "description", updatedListDescription),
					CheckNGWAFWorkspaceListEntries(tc.resourceName, tc.updatedEntries),
					CheckNGWAFWorkspaceListExists(tc.resourceName, tc.listType, listName, updatedListDescription, tc.updatedEntries),
				),
			},
			{
				Config:   configNGWAFWorkspaceList(tc.updatedBlock, workspaceName, listName, updatedListDescription, tc.updatedEntries),
				PlanOnly: true,
			},
			{
				ResourceName:      tc.resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					list := s.RootModule().Resources[tc.resourceName]
					workspace := s.RootModule().Resources["fastly_ngwaf_workspace.test"]
					return fmt.Sprintf("%s/%s", workspace.Primary.ID, list.Primary.ID), nil
				},
			},
		},
	})
}

func TestAccFastlyNGWAFWorkspaceLists_byTypeAndDataSource(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-lists-ds-%s", suffix)

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
		CheckDestroy:             CheckNGWAFWorkspaceListAndWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceListsByType(workspaceName, names),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_lists.test", "lists.#", "5"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["data.fastly_ngwaf_workspace_lists.test"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_ngwaf_workspace_lists.test")
						}

						wantNames := []string{names["ip"], names["string"], names["wildcard"], names["country"], names["signal"]}
						wantTypes := []string{"ip", "string", "wildcard", "country", "signal"}

						var foundNames int
						var foundTypes int
						var gotNames []string
						var gotTypes []string

						for k, v := range rs.Primary.Attributes {
							if strings.HasSuffix(k, ".name") {
								gotNames = append(gotNames, v)
								if slices.Contains(wantNames, v) {
									foundNames++
								}
							}
							if strings.HasSuffix(k, ".type") {
								gotTypes = append(gotTypes, v)
								if slices.Contains(wantTypes, v) {
									foundTypes++
								}
							}
							if strings.HasSuffix(k, ".reference_id") && v == "" {
								return fmt.Errorf("expected data source list %s to have reference_id", k)
							}
						}

						if foundNames != len(wantNames) {
							return fmt.Errorf("want names: %v, got: %v", wantNames, gotNames)
						}
						if foundTypes != len(wantTypes) {
							return fmt.Errorf("want types: %v, got: %v", wantTypes, gotTypes)
						}

						return nil
					},
				),
			},
		},
	})
}

func configNGWAFWorkspaceList(blockFile, workspaceName, listName, listDescription string, entries []string) string {
	return RenderBlock("internal/acceptance_tests/blocks/"+blockFile, map[string]string{
		"WORKSPACE_NAME":           workspaceName,
		"LIST_NAME":                listName,
		"LIST_DESCRIPTION":         listDescription,
		"LIST_DESCRIPTION_UPDATED": listDescription,
		"LIST_ENTRIES":             terraformStringList(entries),
		"LIST_ENTRIES_UPDATED":     terraformStringList(entries),
	})
}

func terraformStringList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}

func CheckNGWAFWorkspaceListEntries(resourceName string, expected []string) resource.TestCheckFunc {
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

func CheckNGWAFWorkspaceListExists(resourceName, expectedType, expectedName, expectedDescription string, expectedEntries []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		if workspaceID == "" {
			return fmt.Errorf("workspace_id is not set for %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		list, err := lists.Get(context.Background(), client, &lists.GetInput{
			ListID: &rs.Primary.ID,
			Scope: &scope.Scope{
				Type:      scope.ScopeTypeWorkspace,
				AppliesTo: []string{workspaceID},
			},
		})
		if err != nil {
			return fmt.Errorf("error reading NGWAF workspace list %s: %w", rs.Primary.ID, err)
		}

		if list.Type != expectedType {
			return fmt.Errorf("NGWAF workspace list type = %q, want %q", list.Type, expectedType)
		}
		if list.Name != expectedName {
			return fmt.Errorf("NGWAF workspace list name = %q, want %q", list.Name, expectedName)
		}
		if list.Description != expectedDescription {
			return fmt.Errorf("NGWAF workspace list description = %q, want %q", list.Description, expectedDescription)
		}
		if !slices.Equal(list.Entries, expectedEntries) {
			return fmt.Errorf("NGWAF workspace list entries = %v, want %v", list.Entries, expectedEntries)
		}
		if list.ReferenceID == "" {
			return fmt.Errorf("NGWAF workspace list %s has empty reference_id", rs.Primary.ID)
		}

		return nil
	}
}

func CheckNGWAFWorkspaceListDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if !strings.HasPrefix(rs.Type, "fastly_ngwaf_workspace_") || !strings.HasSuffix(rs.Type, "_list") {
			continue
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		if workspaceID == "" {
			continue
		}

		_, err := lists.Get(context.Background(), client, &lists.GetInput{
			ListID: &rs.Primary.ID,
			Scope: &scope.Scope{
				Type:      scope.ScopeTypeWorkspace,
				AppliesTo: []string{workspaceID},
			},
		})
		if err == nil {
			return fmt.Errorf("NGWAF workspace list %s still exists after destroy", rs.Primary.ID)
		}
	}

	return nil
}

func CheckNGWAFWorkspaceListAndWorkspaceDestroy(s *terraform.State) error {
	if err := CheckNGWAFWorkspaceListDestroy(s); err != nil {
		return err
	}
	return CheckNGWAFWorkspaceDestroy(s)
}
