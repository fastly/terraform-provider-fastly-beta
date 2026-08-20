package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccFastlyCustomDashboard_lifecycle(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	name := fmt.Sprintf("tf-test-dashboard-%s", suffix)
	description := fmt.Sprintf("created-by-tf-%s", suffix)
	updatedName := fmt.Sprintf("tf-test-dashboard-updated-%s", suffix)
	updatedDescription := fmt.Sprintf("updated-by-tf-%s", suffix)

	var item2ID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckCustomDashboardDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigCustomDashboard("custom_dashboard_basic.tf", name, description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "name", name),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "description", description),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.#", "2"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.0.key", "item1"),
					resource.TestCheckResourceAttrSet("fastly_custom_dashboard.test", "dashboard_item.0.id"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.0.span", "4"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.0.visualization.0.config.0.format", "number"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.1.key", "item2"),
					resource.TestCheckResourceAttrSet("fastly_custom_dashboard.test", "dashboard_item.1.id"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.1.visualization.0.config.0.calculation_method", "avg"),
					resource.TestCheckResourceAttrSet("fastly_custom_dashboard.test", "id"),
					captureResourceAttr("fastly_custom_dashboard.test", "dashboard_item.1.id", &item2ID),
				),
			},
			{
				Config: ConfigCustomDashboard("custom_dashboard_updated.tf", updatedName, updatedDescription),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "name", updatedName),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "description", updatedDescription),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.#", "2"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.0.key", "item2"),
					resource.TestCheckResourceAttrSet("fastly_custom_dashboard.test", "dashboard_item.0.id"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.0.visualization.0.config.0.plot_type", "donut"),
					checkResourceAttrEqualsCaptured("fastly_custom_dashboard.test", "dashboard_item.0.id", &item2ID),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.1.key", "item3"),
					resource.TestCheckResourceAttrSet("fastly_custom_dashboard.test", "dashboard_item.1.id"),
				),
			},
			{
				// Removing description and all items must explicitly clear them
				// remotely rather than preserving stale API values.
				Config: ConfigCustomDashboard("custom_dashboard_minimal.tf", updatedName, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("fastly_custom_dashboard.test", "description"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.#", "0"),
				),
			},
			{
				Config:   ConfigCustomDashboard("custom_dashboard_minimal.tf", updatedName, ""),
				PlanOnly: true,
			},
			{
				// Explicit empty string is distinct Terraform configuration from an
				// omitted description, even though the API represents both as "".
				Config: ConfigCustomDashboard("custom_dashboard_empty_description.tf", updatedName, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "description", ""),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.#", "0"),
				),
			},
			{
				Config:   ConfigCustomDashboard("custom_dashboard_empty_description.tf", updatedName, ""),
				PlanOnly: true,
			},
			{
				// Return to omitted description so import also exercises the null
				// representation of the API's empty string.
				Config: ConfigCustomDashboard("custom_dashboard_minimal.tf", updatedName, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("fastly_custom_dashboard.test", "description"),
				),
			},
			{
				ResourceName:      "fastly_custom_dashboard.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyCustomDashboard_importWithItems(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	name := fmt.Sprintf("tf-test-dashboard-import-%s", suffix)
	description := fmt.Sprintf("created-for-import-%s", suffix)

	var item1ID string
	var item2ID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckCustomDashboardDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigCustomDashboard("custom_dashboard_basic.tf", name, description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.#", "2"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.0.key", "item1"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.1.key", "item2"),
					captureResourceAttr("fastly_custom_dashboard.test", "dashboard_item.0.id", &item1ID),
					captureResourceAttr("fastly_custom_dashboard.test", "dashboard_item.1.id", &item2ID),
				),
			},
			{
				ResourceName:            "fastly_custom_dashboard.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"dashboard_item.0.key", "dashboard_item.1.key"},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					return checkImportedDashboardItemIdentity(states, item1ID, item2ID)
				},
			},
			{
				// Import synthesizes key=id because Fastly does not store Terraform's
				// friendly key. Re-applying the original configuration must adopt those
				// imported items rather than replacing them with new Fastly IDs.
				Config: ConfigCustomDashboard("custom_dashboard_basic.tf", name, description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.0.key", "item1"),
					resource.TestCheckResourceAttr("fastly_custom_dashboard.test", "dashboard_item.1.key", "item2"),
					checkResourceAttrEqualsCaptured("fastly_custom_dashboard.test", "dashboard_item.0.id", &item1ID),
					checkResourceAttrEqualsCaptured("fastly_custom_dashboard.test", "dashboard_item.1.id", &item2ID),
				),
			},
			{
				Config:   ConfigCustomDashboard("custom_dashboard_basic.tf", name, description),
				PlanOnly: true,
			},
		},
	})
}

func checkImportedDashboardItemIdentity(states []*terraform.InstanceState, expectedIDs ...string) error {
	if len(states) != 1 {
		return fmt.Errorf("expected exactly one imported state, got %d", len(states))
	}

	attrs := states[0].Attributes
	countRaw, ok := attrs["dashboard_item.#"]
	if !ok {
		return fmt.Errorf("imported state does not contain dashboard_item.#")
	}
	count, err := strconv.Atoi(countRaw)
	if err != nil {
		return fmt.Errorf("parsing imported dashboard item count %q: %w", countRaw, err)
	}
	if count != len(expectedIDs) {
		return fmt.Errorf("imported dashboard item count = %d, want %d", count, len(expectedIDs))
	}

	expected := make(map[string]struct{}, len(expectedIDs))
	for _, id := range expectedIDs {
		if id == "" {
			return fmt.Errorf("expected Fastly dashboard item ID was not captured before import")
		}
		expected[id] = struct{}{}
	}

	seen := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		idAttr := fmt.Sprintf("dashboard_item.%d.id", i)
		keyAttr := fmt.Sprintf("dashboard_item.%d.key", i)

		id := attrs[idAttr]
		key := attrs[keyAttr]
		if id == "" {
			return fmt.Errorf("imported state attribute %s is empty", idAttr)
		}
		if key == "" {
			return fmt.Errorf("imported state attribute %s is empty", keyAttr)
		}
		if key != id {
			return fmt.Errorf("imported dashboard item %d synthesized key %q, want Fastly ID %q", i, key, id)
		}
		if _, ok := expected[id]; !ok {
			return fmt.Errorf("imported dashboard item %d has unexpected Fastly ID %q", i, id)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("imported Fastly dashboard item ID %q appears more than once", id)
		}
		seen[id] = struct{}{}
	}

	for id := range expected {
		if _, ok := seen[id]; !ok {
			return fmt.Errorf("Fastly dashboard item ID %q was not preserved by import", id)
		}
	}

	return nil
}

func ConfigCustomDashboard(blockFile, name, description string) string {
	raw, err := os.ReadFile("blocks/" + blockFile)
	if err != nil {
		panic(err)
	}
	config := string(raw)
	config = strings.ReplaceAll(config, "{{.DASHBOARD_NAME}}", name)
	config = strings.ReplaceAll(config, "{{.DASHBOARD_DESCRIPTION}}", description)
	return config
}

func captureResourceAttr(resourceName, attr string, target *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceName)
		}
		value, ok := rs.Primary.Attributes[attr]
		if !ok || value == "" {
			return fmt.Errorf("attribute %s.%s is not set", resourceName, attr)
		}
		*target = value
		return nil
	}
}

func checkResourceAttrEqualsCaptured(resourceName, attr string, expected *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceName)
		}
		got := rs.Primary.Attributes[attr]
		if got != *expected {
			return fmt.Errorf("%s.%s changed Fastly identity: got %q, want %q", resourceName, attr, got, *expected)
		}
		return nil
	}
}

func CheckCustomDashboardDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	dashboards, err := client.ListObservabilityCustomDashboards(context.Background(), &fastly.ListObservabilityCustomDashboardsInput{})
	if err != nil {
		return fmt.Errorf("error listing custom dashboards during destroy check: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_custom_dashboard" {
			continue
		}
		for _, dashboard := range dashboards.Data {
			if dashboard.ID == rs.Primary.ID {
				return fmt.Errorf("custom dashboard %s still exists after destroy", rs.Primary.ID)
			}
		}
	}
	return nil
}
