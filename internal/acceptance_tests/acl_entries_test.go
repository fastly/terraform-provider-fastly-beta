package acceptancetests

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/computeacls"
)

const (
	standaloneACLEntriesPollInterval = 500 * time.Millisecond
	standaloneACLEntriesPollTimeout  = 30 * time.Second
)

func TestAccFastlyACLEntries_lifecycle(t *testing.T) {
	t.Parallel()

	aclName := fmt.Sprintf("tf_test_acl_entries_%s", acctest.RandString(10))

	managedV1 := map[string]string{
		"192.0.2.0/24":    "ALLOW",
		"198.51.100.0/24": "BLOCK",
	}
	managedV2 := map[string]string{
		"192.0.2.0/24":   "BLOCK",
		"203.0.113.0/24": "ALLOW",
	}
	external := map[string]string{
		"10.0.0.0/8": "ALLOW",
	}
	externalWithSecond := map[string]string{
		"10.0.0.0/8":    "ALLOW",
		"172.16.0.0/12": "BLOCK",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckACLDestroy,
		Steps: []resource.TestStep{
			{
				// Seed an entry outside Terraform before the entries resource exists.
				// Creating fastly_acl_entries must leave that entry untouched.
				Config: ConfigACL(aclName),
				Check: InsertStandaloneACLEntry(
					"fastly_acl.acl",
					"10.0.0.0/8",
					"ALLOW",
				),
			},
			{
				Config: ConfigACLEntries(aclName, managedV1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_acl_entries.acl_entries", "id"),
					resource.TestCheckResourceAttrPair("fastly_acl_entries.acl_entries", "acl_id", "fastly_acl.acl", "id"),
					resource.TestCheckResourceAttr("fastly_acl_entries.acl_entries", "entries.%", "2"),
					resource.TestCheckResourceAttr("fastly_acl_entries.acl_entries", "entries.192.0.2.0/24", "ALLOW"),
					resource.TestCheckResourceAttr("fastly_acl_entries.acl_entries", "entries.198.51.100.0/24", "BLOCK"),
					CheckStandaloneACLEntriesRemoteState(
						"fastly_acl_entries.acl_entries",
						mergeStringMaps(external, managedV1),
					),
				),
			},
			{
				// Exercise create, update, and delete in one reconciliation:
				// one prefix changes action, one is removed, and one is added.
				Config: ConfigACLEntries(aclName, managedV2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_acl_entries.acl_entries", "entries.%", "2"),
					resource.TestCheckResourceAttr("fastly_acl_entries.acl_entries", "entries.192.0.2.0/24", "BLOCK"),
					resource.TestCheckResourceAttr("fastly_acl_entries.acl_entries", "entries.203.0.113.0/24", "ALLOW"),
					CheckStandaloneACLEntriesRemoteState(
						"fastly_acl_entries.acl_entries",
						mergeStringMaps(external, managedV2),
					),
				),
			},
			{
				Config:             ConfigACLEntries(aclName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Introduce out-of-band drift on a Terraform-managed prefix.
				// The Check intentionally mutates remote state after apply, so the
				// automatic post-apply refresh plan must be non-empty.
				Config:             ConfigACLEntries(aclName, managedV2),
				ExpectNonEmptyPlan: true,
				Check: UpdateStandaloneACLEntry(
					"fastly_acl_entries.acl_entries",
					"192.0.2.0/24",
					"ALLOW",
				),
			},
			{
				// Managed drift must be visible in the Terraform plan.
				Config:             ConfigACLEntries(aclName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Applying unchanged configuration repairs managed drift.
				Config: ConfigACLEntries(aclName, managedV2),
				Check: CheckStandaloneACLEntriesRemoteState(
					"fastly_acl_entries.acl_entries",
					mergeStringMaps(external, managedV2),
				),
			},
			{
				// Add another prefix outside Terraform. Because it is not declared
				// in entries, the resource must leave it alone.
				Config: ConfigACLEntries(aclName, managedV2),
				Check: InsertStandaloneACLEntry(
					"fastly_acl_entries.acl_entries",
					"172.16.0.0/12",
					"BLOCK",
				),
			},
			{
				// Unmanaged external additions must not create Terraform drift.
				Config:             ConfigACLEntries(aclName, managedV2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Removing only the entries resource deletes Terraform-owned
				// prefixes while preserving externally managed ACL entries.
				Config: ConfigACL(aclName),
				Check: CheckStandaloneACLEntriesRemoteState(
					"fastly_acl.acl",
					externalWithSecond,
				),
			},
		},
	})
}

func TestAccFastlyACLEntries_import(t *testing.T) {
	t.Parallel()

	aclName := fmt.Sprintf("tf_test_acl_entries_import_%s", acctest.RandString(10))
	entries := map[string]string{
		"192.0.2.0/24":    "ALLOW",
		"198.51.100.0/24": "BLOCK",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckACLDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntries(aclName, entries),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_acl_entries.acl_entries", "entries.%", "2"),
					CheckStandaloneACLEntriesRemoteState("fastly_acl_entries.acl_entries", entries),
				),
			},
			{
				ResourceName:      "fastly_acl_entries.acl_entries",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:             ConfigACLEntries(aclName, entries),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccFastlyACLEntries_invalidPrefix(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntries("tf_test_acl_invalid_config", map[string]string{
					"not_a_cidr": "ALLOW",
				}),
				ExpectError: regexp.MustCompile("not a valid CIDR prefix"),
			},
		},
	})
}

func TestAccFastlyACLEntries_invalidAction(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntries("tf_test_acl_invalid_config", map[string]string{
					"192.0.2.0/24": "PERMIT",
				}),
				ExpectError: regexp.MustCompile("must be either ALLOW or BLOCK"),
			},
		},
	})
}

func InsertStandaloneACLEntry(resourceName, prefix, action string) resource.TestCheckFunc {
	return mutateStandaloneACLEntry(resourceName, prefix, action, "create")
}

func UpdateStandaloneACLEntry(resourceName, prefix, action string) resource.TestCheckFunc {
	return mutateStandaloneACLEntry(resourceName, prefix, action, "update")
}

func mutateStandaloneACLEntry(resourceName, prefix, action, operation string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		aclID, err := standaloneACLIDFromState(s, resourceName)
		if err != nil {
			return err
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		if err := computeacls.Update(context.Background(), client, &computeacls.UpdateInput{
			ComputeACLID: &aclID,
			Entries: []*computeacls.BatchComputeACLEntry{
				{
					Prefix:    &prefix,
					Action:    &action,
					Operation: &operation,
				},
			},
		}); err != nil {
			return fmt.Errorf("error applying external ACL entry %s for %q: %w", operation, prefix, err)
		}

		deadline := time.Now().Add(standaloneACLEntriesPollTimeout)
		for {
			remote, err := standaloneACLEntriesRemoteState(s, resourceName)
			if err != nil {
				return err
			}
			if remoteAction, ok := remote[prefix]; ok && remoteAction == action {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for external ACL entry %q to converge to %q", prefix, action)
			}
			time.Sleep(standaloneACLEntriesPollInterval)
		}
	}
}

func CheckStandaloneACLEntriesRemoteState(resourceName string, want map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got, err := standaloneACLEntriesRemoteState(s, resourceName)
		if err != nil {
			return err
		}

		if !maps.Equal(got, want) {
			return fmt.Errorf("unexpected ACL entries:\ngot:  %#v\nwant: %#v", got, want)
		}
		return nil
	}
}

func standaloneACLEntriesRemoteState(s *terraform.State, resourceName string) (map[string]string, error) {
	aclID, err := standaloneACLIDFromState(s, resourceName)
	if err != nil {
		return nil, err
	}

	client, err := NewFastlyClient()
	if err != nil {
		return nil, fmt.Errorf("error creating Fastly client: %w", err)
	}

	result := make(map[string]string)
	var cursor *string

	for {
		resp, err := computeacls.ListEntries(context.Background(), client, &computeacls.ListEntriesInput{
			ComputeACLID: &aclID,
			Cursor:       cursor,
		})
		if err != nil {
			return nil, fmt.Errorf("error listing ACL entries: %w", err)
		}

		for _, entry := range resp.Entries {
			result[entry.Prefix] = entry.Action
		}

		if resp.Meta.NextCursor == "" {
			break
		}
		cursor = new(resp.Meta.NextCursor)
	}

	return result, nil
}

func standaloneACLIDFromState(s *terraform.State, resourceName string) (string, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return "", fmt.Errorf("not found: %s", resourceName)
	}

	if aclID := rs.Primary.Attributes["acl_id"]; aclID != "" {
		return aclID, nil
	}
	if rs.Primary.ID != "" {
		return rs.Primary.ID, nil
	}

	return "", fmt.Errorf("%s has no ACL ID", resourceName)
}
