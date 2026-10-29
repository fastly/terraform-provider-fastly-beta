package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v18/fastly/notifications/v1/eventmappings"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
)

func TestAccFastlyAuditLogEventMapping_account(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	integrationName := fmt.Sprintf("tf-test-integration-%s", suffix)
	name := fmt.Sprintf("tf-test-mapping-%s", suffix)
	updatedName := fmt.Sprintf("tf-test-mapping-updated-%s", suffix)
	var mappingID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAuditLogEventMappingDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigAuditLogEventMapping(integrationName, name, "created by acceptance test", "account", "[]", `["address.create"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "name", name),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "description", "created by acceptance test"),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "scope_type", "account"),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "scope_ids.#", "0"),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "event_types.#", "1"),
					resource.TestCheckTypeSetElemAttr("fastly_audit_log_event_mapping.test", "event_types.*", "address.create"),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "integration_ids.#", "1"),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "mapping_status", "active"),
					resource.TestCheckResourceAttrSet("fastly_audit_log_event_mapping.test", "created_at"),
					resource.TestCheckResourceAttrSet("fastly_audit_log_event_mapping.test", "updated_at"),
					CheckAuditLogEventMappingRemoteState("fastly_audit_log_event_mapping.test", name, "account"),
					captureResourceID("fastly_audit_log_event_mapping.test", &mappingID),
				),
			},
			{
				Config: ConfigAuditLogEventMapping(integrationName, updatedName, "updated by acceptance test", "account", "[]", `["address.create", "address.delete"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "name", updatedName),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "description", "updated by acceptance test"),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "event_types.#", "2"),
					CheckAuditLogEventMappingRemoteState("fastly_audit_log_event_mapping.test", updatedName, "account"),
					// Same ID across the update confirms this was an in-place update, not a delete+recreate.
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_audit_log_event_mapping.test"]
						if !ok {
							return fmt.Errorf("resource not found in state: fastly_audit_log_event_mapping.test")
						}
						if rs.Primary.ID != mappingID {
							return fmt.Errorf("expected Audit Log Event Mapping ID to remain %q across update, got %q", mappingID, rs.Primary.ID)
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "fastly_audit_log_event_mapping.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyAuditLogEventMapping_scopedToService(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	name := fmt.Sprintf("tf-test-mapping-%s", suffix)
	config := joinBlocks(
		ConfigCDNAutoWithBackend(fmt.Sprintf("tf-test-service-%s", suffix), fmt.Sprintf("tf-test-%s.example.com", suffix), "tf-test-backend"),
		ConfigAuditLogEventMapping(fmt.Sprintf("tf-test-integration-%s", suffix), name, "scoped to a service", "vcl", "[fastly_service_cdn_auto.test.id]", `["backend.delete"]`),
	)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAuditLogEventMappingDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "scope_type", "vcl"),
					resource.TestCheckResourceAttr("fastly_audit_log_event_mapping.test", "scope_ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair("fastly_audit_log_event_mapping.test", "scope_ids.*", "fastly_service_cdn_auto.test", "id"),
					CheckAuditLogEventMappingRemoteState("fastly_audit_log_event_mapping.test", name, "vcl"),
				),
			},
			{
				ResourceName:      "fastly_audit_log_event_mapping.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyAuditLogEventMapping_invalidScopeTypeValidationError(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigAuditLogEventMappingInvalid("tf-test-mapping", "bogus", "[]"),
				ExpectError: regexp.MustCompile("Attribute scope_type value must be one of"),
			},
		},
	})
}

func TestAccFastlyAuditLogEventMapping_accountScopeWithScopeIDsValidationError(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigAuditLogEventMappingInvalid("tf-test-mapping", "account", `["some-service-id"]`),
				ExpectError: regexp.MustCompile("must be empty when `scope_type` is\\s+`account`"),
			},
		},
	})
}

// TestAccFastlyAuditLogEventMapping_recreateAfterManualDelete confirms an out-of-band delete is recreated on apply.
func TestAccFastlyAuditLogEventMapping_recreateAfterManualDelete(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	name := fmt.Sprintf("tf-test-mapping-%s", suffix)
	config := ConfigAuditLogEventMapping(fmt.Sprintf("tf-test-integration-%s", suffix), name, "created by acceptance test", "account", "[]", `["address.create"]`)

	var mappingID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAuditLogEventMappingDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					CheckAuditLogEventMappingRemoteState("fastly_audit_log_event_mapping.test", name, "account"),
					captureResourceID("fastly_audit_log_event_mapping.test", &mappingID),
				),
			},
			{
				PreConfig: func() {
					client, err := NewFastlyClient()
					if err != nil {
						t.Fatalf("error creating Fastly client: %s", err)
					}
					if err := eventmappings.Delete(context.Background(), client, &eventmappings.DeleteInput{MappingID: new(mappingID)}); err != nil {
						t.Fatalf("error manually deleting Audit Log Event Mapping: %s", err)
					}
				},
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					CheckAuditLogEventMappingRemoteState("fastly_audit_log_event_mapping.test", name, "account"),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceAuditLogEventMapping(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	name := fmt.Sprintf("tf-test-mapping-%s", suffix)
	mapping := ConfigAuditLogEventMapping(fmt.Sprintf("tf-test-integration-%s", suffix), name, "created by acceptance test", "account", "[]", `["address.create"]`)

	byID := joinBlocks(mapping, ConfigAuditLogEventMappingDataSource(`id = fastly_audit_log_event_mapping.test.id`))
	byName := joinBlocks(mapping, ConfigAuditLogEventMappingDataSource(fmt.Sprintf(`name       = %q
  depends_on = [fastly_audit_log_event_mapping.test]`, name)))
	noMatch := ConfigAuditLogEventMappingDataSource(fmt.Sprintf(`name = "tf-test-does-not-exist-%s"`, suffix))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAuditLogEventMappingDestroy,
		Steps: []resource.TestStep{
			{
				Config: byID,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.fastly_audit_log_event_mapping.test", "id", "fastly_audit_log_event_mapping.test", "id"),
					resource.TestCheckResourceAttr("data.fastly_audit_log_event_mapping.test", "name", name),
					resource.TestCheckResourceAttr("data.fastly_audit_log_event_mapping.test", "scope_type", "account"),
					resource.TestCheckResourceAttr("data.fastly_audit_log_event_mapping.test", "event_types.#", "1"),
					resource.TestCheckResourceAttr("data.fastly_audit_log_event_mapping.test", "mapping_status", "active"),
				),
			},
			{
				Config: byName,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.fastly_audit_log_event_mapping.test", "id", "fastly_audit_log_event_mapping.test", "id"),
					resource.TestCheckResourceAttrPair("data.fastly_audit_log_event_mapping.test", "integration_ids.#", "fastly_audit_log_event_mapping.test", "integration_ids.#"),
				),
			},
			{
				Config:      noMatch,
				ExpectError: regexp.MustCompile("returned no results"),
			},
		},
	})
}

func TestAccFastlyDataSourceAuditLogEventMapping_idConflictsWithFilters(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: ConfigAuditLogEventMappingDataSource(`id   = "abc"
  name = "def"`),
				ExpectError: regexp.MustCompile("(?s)Invalid Attribute Combination"),
			},
		},
	})
}

func CheckAuditLogEventMappingDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_audit_log_event_mapping" {
			continue
		}

		_, err := eventmappings.Get(context.Background(), client, &eventmappings.GetInput{MappingID: new(rs.Primary.ID)})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Audit Log Event Mapping %q was destroyed: %w", rs.Primary.ID, err)
		}

		return fmt.Errorf("Audit Log Event Mapping %s still exists", rs.Primary.ID)
	}

	return nil
}

// CheckAuditLogEventMappingRemoteState verifies name/scope_type directly against the live API.
func CheckAuditLogEventMappingRemoteState(resourceName, expectedName, expectedScopeType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		m, err := eventmappings.Get(context.Background(), client, &eventmappings.GetInput{MappingID: new(rs.Primary.ID)})
		if err != nil {
			return fmt.Errorf("error reading Audit Log Event Mapping %q: %w", rs.Primary.ID, err)
		}

		if m.Name != expectedName {
			return fmt.Errorf("unexpected remote Audit Log Event Mapping name: got %q, want %q", m.Name, expectedName)
		}
		if m.ScopeType != expectedScopeType {
			return fmt.Errorf("unexpected remote Audit Log Event Mapping scope_type: got %q, want %q", m.ScopeType, expectedScopeType)
		}

		return nil
	}
}
