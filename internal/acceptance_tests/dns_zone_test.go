package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/fastly/go-fastly/v17/fastly/dns/v1/dnszones"
	"github.com/fastly/go-fastly/v17/fastly/dns/v1/tsigkeys"
)

func testZoneName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s.fastly-example.com.", acctest.RandString(10))
}

func TestAccFastlyDNSZone_basic(t *testing.T) {
	t.Parallel()
	name := testZoneName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigDNSZone(name, "initial description"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "name", name),
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "description", "initial description"),
					resource.TestCheckResourceAttrSet("fastly_dns_zone.test", "id"),
				),
			},
			{
				Config: ConfigDNSZone(name, "updated description"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "description", "updated description"),
				),
			},
			{
				ResourceName:      "fastly_dns_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyDNSZone_nameForcesReplace confirms changing name replaces the
// zone rather than updating in place — the API can't rename a zone.
func TestAccFastlyDNSZone_nameForcesReplace(t *testing.T) {
	t.Parallel()
	name := testZoneName(t)
	renamed := testZoneName(t)

	var firstID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigDNSZoneMinimal(name),
				Check: resource.ComposeTestCheckFunc(
					captureResourceID("fastly_dns_zone.test", &firstID),
				),
			},
			{
				Config: ConfigDNSZoneMinimal(renamed),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "name", renamed),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_dns_zone.test"]
						if !ok {
							return fmt.Errorf("not found: fastly_dns_zone.test")
						}
						if rs.Primary.ID == firstID {
							return fmt.Errorf("expected a new zone ID after renaming, got the same ID %s", firstID)
						}
						return nil
					},
				),
			},
		},
	})
}

func captureResourceID(resourceName string, out *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}
		*out = rs.Primary.ID
		return nil
	}
}

func TestAccFastlyDNSZone_invalidName(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigDNSZoneMinimal("example.com"),
				ExpectError: regexp.MustCompile("must be in FQDN format"),
			},
		},
	})
}

// TestAccFastlyDNSZone_invalidPrimaryAddress guards a documented API
// constraint: DNS zone transfers don't support IPv6 for primary servers.
func TestAccFastlyDNSZone_invalidPrimaryAddress(t *testing.T) {
	t.Parallel()
	name := testZoneName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigDNSZoneWithXfrConfig(name, "zone with invalid primary", "2001:db8::1", "primary server"),
				ExpectError: regexp.MustCompile("must be a valid IPv4 address"),
			},
		},
	})
}

func TestAccFastlyDNSZone_withXfrConfig(t *testing.T) {
	t.Parallel()
	name := testZoneName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigDNSZoneWithXfrConfig(name, "zone with xfr config", "1.2.3.4", "primary server"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "xfr_config_inbound.0.primaries.0.address", "1.2.3.4"),
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "xfr_config_inbound.0.primaries.0.description", "primary server"),
				),
			},
			{
				Config: ConfigDNSZoneWithXfrConfig(name, "zone with updated xfr config", "5.6.7.8", "updated primary server"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "xfr_config_inbound.0.primaries.0.address", "5.6.7.8"),
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "xfr_config_inbound.0.primaries.0.description", "updated primary server"),
				),
			},
			{
				ResourceName:      "fastly_dns_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyDNSZone_clearInboundTSIGKeyID mirrors the legacy provider's
// ClearFields test: creates a zone referencing a TSIG key made out-of-band
// (no fastly_tsig_key resource exists yet), then clears just
// inbound_tsig_key_id while keeping xfr_config_inbound configured.
func TestAccFastlyDNSZone_clearInboundTSIGKeyID(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	name := testZoneName(t)
	tsigKeyName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	tsigKey, err := tsigkeys.Create(context.Background(), client, &tsigkeys.CreateInput{
		Name:      &tsigKeyName,
		Algorithm: new("hmac-sha256"),
		Secret:    new("dGVzdHNlY3JldA=="),
	})
	if err != nil {
		t.Fatalf("creating out-of-band TSIG key: %s", err)
	}
	t.Cleanup(func() {
		if err := tsigkeys.Delete(context.Background(), client, &tsigkeys.DeleteInput{TSIGKeyID: tsigKey.ID}); err != nil {
			t.Logf("cleanup: deleting out-of-band TSIG key %s: %s", *tsigKey.ID, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigDNSZoneWithTSIGKey(name, "description to be cleared", *tsigKey.ID, "1.2.3.4", "primary server"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("fastly_dns_zone.test", "xfr_config_inbound.0.inbound_tsig_key_id"),
				),
			},
			{
				Config: ConfigDNSZoneWithXfrConfig(name, "", "1.2.3.4", "primary server"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("fastly_dns_zone.test", "xfr_config_inbound.0.inbound_tsig_key_id"),
					resource.TestCheckResourceAttr("fastly_dns_zone.test", "description", ""),
				),
			},
		},
	})
}

func TestAccFastlyDataSourceDNSZones(t *testing.T) {
	t.Parallel()
	h := acctest.RandString(10)
	name1 := fmt.Sprintf("tf-%s-1.fastly-example.com.", h)
	name2 := fmt.Sprintf("tf-%s-2.fastly-example.com.", h)
	name3 := fmt.Sprintf("tf-%s-3.fastly-example.com.", h)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigDNSZonesDataSource(name1, name2, name3),
				Check: resource.ComposeTestCheckFunc(
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["data.fastly_dns_zones.example"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_dns_zones.example")
						}

						want := []string{name1, name2, name3}

						var found int
						var got []string
						for k, v := range rs.Primary.Attributes {
							if strings.HasSuffix(k, ".name") {
								got = append(got, v)
								if slices.Contains(want, v) {
									found++
								}
							}
						}

						if found != len(want) {
							return fmt.Errorf("want: %v, got: %v", want, got)
						}

						return nil
					},
				),
			},
		},
	})
}

func CheckDNSZoneDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_dns_zone" {
			continue
		}

		id := rs.Primary.ID
		_, err := dnszones.Get(context.Background(), client, &dnszones.GetInput{ZoneID: &id})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if DNS Zone was destroyed: %w", err)
		}

		return fmt.Errorf("DNS Zone %s still exists", id)
	}

	return nil
}
