package acceptancetests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyServiceCDNACLEntries_create(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	aclName := fmt.Sprintf("acl_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntriesCreate(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "1"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 1),
				),
			},
			{
				ResourceName:      "fastly_service_cdn_acl_entries.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyServiceCDNACLEntries_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	aclName := fmt.Sprintf("acl_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntriesCreate(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "1"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 1),
				),
			},
			{
				Config: ConfigACLEntriesUpdate(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "2"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 2),
				),
			},
		},
	})
}

func TestAccFastlyServiceCDNACLEntries_delete(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	aclName := fmt.Sprintf("acl_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntriesCreate(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "1"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 1),
				),
			},
			{
				Config: ConfigACLEntriesDelete(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "0"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 0),
				),
			},
		},
	})
}

// TestAccFastlyServiceCDNACLEntries_coexistsWithExternalEntries verifies the
// versionless container ownership model: Terraform must manage only the
// entries declared in its `entry` blocks, leaving ACL entries created outside
// Terraform untouched, and it must not treat unmanaged remote entries as
// drift.
func TestAccFastlyServiceCDNACLEntries_coexistsWithExternalEntries(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	aclName := fmt.Sprintf("acl_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				// Seed an entry outside Terraform before the entries resource
				// exists. Creating fastly_service_cdn_acl_entries afterward must
				// leave it untouched.
				Config: ConfigACLEntriesACLOnly(serviceName, domainName, aclName),
				Check:  InsertACLEntry("fastly_service_cdn.test", aclName, "203.0.113.5", 32, "external"),
			},
			{
				Config: ConfigACLEntriesCreate(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "1"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 2),
					CheckACLEntryPresent("fastly_service_cdn.test", aclName, "203.0.113.5", 32),
				),
			},
			{
				// Exercise create, update, and delete in one reconciliation:
				// a second managed entry is added while the external entry
				// must survive untouched.
				Config: ConfigACLEntriesUpdate(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "2"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 3),
					CheckACLEntryPresent("fastly_service_cdn.test", aclName, "203.0.113.5", 32),
				),
			},
			{
				Config:             ConfigACLEntriesUpdate(serviceName, domainName, aclName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Introduce out-of-band drift on a Terraform-managed entry.
				// The Check intentionally mutates remote state after apply, so
				// this step must expect the automatic post-apply refresh plan
				// to be non-empty.
				Config:             ConfigACLEntriesUpdate(serviceName, domainName, aclName),
				ExpectNonEmptyPlan: true,
				Check:              DriftACLEntryComment("fastly_service_cdn.test", aclName, "127.0.0.1", 24, "drifted-outside-terraform"),
			},
			{
				// Managed drift must be visible in the Terraform plan.
				Config:             ConfigACLEntriesUpdate(serviceName, domainName, aclName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Applying the unchanged configuration repairs managed drift,
				// and the external entry remains untouched.
				Config: ConfigACLEntriesUpdate(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					CheckACLEntryComment("fastly_service_cdn.test", aclName, "127.0.0.1", 24, "Test entry"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 3),
					CheckACLEntryPresent("fastly_service_cdn.test", aclName, "203.0.113.5", 32),
				),
			},
			{
				// Add another entry outside Terraform. Because it is not
				// declared in `entry`, the resource must leave it alone.
				Config: ConfigACLEntriesUpdate(serviceName, domainName, aclName),
				Check:  InsertACLEntry("fastly_service_cdn.test", aclName, "198.51.100.9", 32, "also-external"),
			},
			{
				// Unmanaged external additions must not create Terraform drift.
				Config:             ConfigACLEntriesUpdate(serviceName, domainName, aclName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Removing the entries resource must delete only the entries
				// it manages, leaving external entries in place.
				Config: ConfigACLEntriesACLOnly(serviceName, domainName, aclName),
				Check:  CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 2),
			},
		},
	})
}

func TestAccFastlyServiceCDNACLEntries_manyEntries(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	aclName := fmt.Sprintf("acl_%s", acctest.RandString(10))
	entryCount := 250

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntriesManyEntries(serviceName, domainName, aclName, entryCount),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", fmt.Sprintf("%d", entryCount)),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, entryCount),
				),
			},
		},
	})
}

// TestAccFastlyServiceACLEntries_omittedOptionalFields exercises an entry
// block that leaves negated, subnet, and comment unset. The API returns an
// explicit value for negated (false) even when it wasn't configured, so this
// guards against the provider reporting "inconsistent result after apply"
// for optional, non-computed entry attributes.
func TestAccFastlyServiceCDNACLEntries_omittedOptionalFields(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	aclName := fmt.Sprintf("acl_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntriesMinimalEntry(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.0.negated", "false"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 1),
				),
			},
			{
				// Re-plan with the same minimal config to confirm no drift
				// diff appears for the fields left unset.
				Config:   ConfigACLEntriesMinimalEntry(serviceName, domainName, aclName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceCDNACLEntries_sameIPDifferentSubnet verifies that two
// entries sharing the same IP but different subnets are created and read back
// correctly, exercising the flatten key-collision fix.
func TestAccFastlyServiceCDNACLEntries_sameIPDifferentSubnet(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	aclName := fmt.Sprintf("acl_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntriesSameIPDifferentSubnet(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "2"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 2),
				),
			},
			{
				// Re-plan with identical config to confirm no spurious diff.
				Config:             ConfigACLEntriesSameIPDifferentSubnet(serviceName, domainName, aclName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccFastlyServiceCDNACLEntries_modifyExistingEntry changes only the comment on an
// existing entry, keeping its ip/subnet fixed. This must apply as an in-place update of
// that entry, not a delete-and-recreate at the same ip/subnet -- Fastly's batch ACL
// entries API rejects a create that collides with an entry not yet deleted in the same
// batch, so a content-keyed (rather than identity-keyed) diff would fail here.
func TestAccFastlyServiceCDNACLEntries_modifyExistingEntry(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	aclName := fmt.Sprintf("acl_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigACLEntriesCreate(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.0.comment", "Test entry"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 1),
				),
			},
			{
				Config: ConfigACLEntriesCommentChanged(serviceName, domainName, aclName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.0.ip", "127.0.0.1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_acl_entries.test", "entry.0.comment", "Test entry - updated"),
					CheckACLEntriesRemoteState("fastly_service_cdn.test", aclName, 1),
				),
			},
		},
	})
}

// lookupServiceAndACLID resolves the service ID (from the given service
// resource in Terraform state) and the ACL ID (by name, via the API) that
// the CDN ACL entries test helpers below operate against.
func lookupServiceAndACLID(s *terraform.State, serviceResource, aclName string) (client *fastly.Client, serviceID, aclID string, err error) {
	rs, ok := s.RootModule().Resources[serviceResource]
	if !ok {
		return nil, "", "", fmt.Errorf("not found: %s", serviceResource)
	}

	serviceID = rs.Primary.ID
	client, err = NewFastlyClient()
	if err != nil {
		return nil, "", "", fmt.Errorf("error creating Fastly client: %w", err)
	}

	acls, err := client.ListACLs(context.Background(), &fastly.ListACLsInput{
		ServiceID:      serviceID,
		ServiceVersion: 1,
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("error listing ACLs: %w", err)
	}

	for _, acl := range acls {
		if acl.Name != nil && *acl.Name == aclName {
			return client, serviceID, *acl.ACLID, nil
		}
	}

	return nil, "", "", fmt.Errorf("ACL %s not found", aclName)
}

func listRemoteACLEntries(client *fastly.Client, serviceID, aclID string) ([]*fastly.ACLEntry, error) {
	paginator := client.GetACLEntries(context.Background(), &fastly.GetACLEntriesInput{
		ServiceID: serviceID,
		ACLID:     aclID,
	})

	var entries []*fastly.ACLEntry
	for paginator.HasNext() {
		results, err := paginator.GetNext()
		if err != nil {
			return nil, fmt.Errorf("error getting ACL entries: %w", err)
		}
		entries = append(entries, results...)
	}
	return entries, nil
}

func CheckACLEntriesRemoteState(serviceResource, aclName string, expectedCount int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, serviceID, aclID, err := lookupServiceAndACLID(s, serviceResource, aclName)
		if err != nil {
			return err
		}

		entries, err := listRemoteACLEntries(client, serviceID, aclID)
		if err != nil {
			return err
		}

		if len(entries) != expectedCount {
			return fmt.Errorf("expected %d entries, got %d", expectedCount, len(entries))
		}

		return nil
	}
}

// findRemoteACLEntry returns the remote entry matching ip/subnet, or nil if absent.
func findRemoteACLEntry(client *fastly.Client, serviceID, aclID, ip string, subnet int) (*fastly.ACLEntry, error) {
	entries, err := listRemoteACLEntries(client, serviceID, aclID)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.IP != nil && *e.IP == ip && e.Subnet != nil && *e.Subnet == subnet {
			return e, nil
		}
	}
	return nil, nil
}

// InsertACLEntry creates an ACL entry directly via the API, bypassing
// Terraform, to simulate an entry managed outside this resource.
func InsertACLEntry(serviceResource, aclName, ip string, subnet int, comment string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, serviceID, aclID, err := lookupServiceAndACLID(s, serviceResource, aclName)
		if err != nil {
			return err
		}

		_, err = client.CreateACLEntry(context.Background(), &fastly.CreateACLEntryInput{
			ServiceID: serviceID,
			ACLID:     aclID,
			IP:        new(ip),
			Subnet:    new(subnet),
			Comment:   new(comment),
		})
		if err != nil {
			return fmt.Errorf("error creating external ACL entry: %w", err)
		}
		return nil
	}
}

// DriftACLEntryComment updates an existing ACL entry's comment directly via
// the API, bypassing Terraform, to simulate drift on a Terraform-managed entry.
func DriftACLEntryComment(serviceResource, aclName, ip string, subnet int, newComment string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, serviceID, aclID, err := lookupServiceAndACLID(s, serviceResource, aclName)
		if err != nil {
			return err
		}

		entry, err := findRemoteACLEntry(client, serviceID, aclID, ip, subnet)
		if err != nil {
			return err
		}
		if entry == nil || entry.EntryID == nil {
			return fmt.Errorf("entry %s/%d not found for ACL %s", ip, subnet, aclName)
		}

		_, err = client.UpdateACLEntry(context.Background(), &fastly.UpdateACLEntryInput{
			ServiceID: serviceID,
			ACLID:     aclID,
			EntryID:   *entry.EntryID,
			Comment:   new(newComment),
		})
		if err != nil {
			return fmt.Errorf("error drifting ACL entry: %w", err)
		}
		return nil
	}
}

// CheckACLEntryPresent asserts that an ACL entry with the given ip/subnet
// exists remotely, regardless of whether Terraform manages it.
func CheckACLEntryPresent(serviceResource, aclName, ip string, subnet int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, serviceID, aclID, err := lookupServiceAndACLID(s, serviceResource, aclName)
		if err != nil {
			return err
		}

		entry, err := findRemoteACLEntry(client, serviceID, aclID, ip, subnet)
		if err != nil {
			return err
		}
		if entry == nil {
			return fmt.Errorf("expected entry %s/%d to exist for ACL %s, but it was not found", ip, subnet, aclName)
		}
		return nil
	}
}

// CheckACLEntryComment asserts that the ACL entry with the given ip/subnet
// exists remotely with the expected comment.
func CheckACLEntryComment(serviceResource, aclName, ip string, subnet int, wantComment string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, serviceID, aclID, err := lookupServiceAndACLID(s, serviceResource, aclName)
		if err != nil {
			return err
		}

		entry, err := findRemoteACLEntry(client, serviceID, aclID, ip, subnet)
		if err != nil {
			return err
		}
		if entry == nil {
			return fmt.Errorf("expected entry %s/%d to exist for ACL %s, but it was not found", ip, subnet, aclName)
		}
		if entry.Comment == nil || *entry.Comment != wantComment {
			got := ""
			if entry.Comment != nil {
				got = *entry.Comment
			}
			return fmt.Errorf("expected entry %s/%d comment %q, got %q", ip, subnet, wantComment, got)
		}
		return nil
	}
}
