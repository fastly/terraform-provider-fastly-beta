package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/settings"
)

func TestAccFastlyServiceSettings_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigSettingsBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "default_host", "override.example.com"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "default_ttl", "120"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "http3", "true"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "stale_if_error", "true"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "stale_if_error_ttl", "600"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_settings.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_settings.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceSettings_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigSettingsBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_settings.test", "default_ttl", "120"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "http3", "true"),
				),
			},
			{
				// Settings has no name/identity field, so an update is always in-place - there is
				// no delete+recreate distinction to make.
				Config: ConfigSettingsUpdated(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_settings.test", "default_host", "other.example.com"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "default_ttl", "300"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "http3", "false"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "stale_if_error", "false"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "stale_if_error_ttl", "1200"),
				),
			},
		},
	})
}

func TestAccFastlyServiceSettings_defaults(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				// Every optional attribute omitted must populate its documented default rather
				// than drifting on every plan.
				Config: ConfigSettingsMinimal(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_settings.test", "default_host", ""),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "default_ttl", "3600"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "http3", "false"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "stale_if_error", "false"),
					resource.TestCheckResourceAttr("fastly_service_settings.test", "stale_if_error_ttl", "43200"),
				),
			},
			{
				Config:   ConfigSettingsMinimal(serviceName, domainName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceSettings_computeServiceRejected verifies that fastly_service_settings, a
// CDN-only resource (general settings were never registered for Compute services in the legacy
// provider either), is rejected when targeting a Compute service.
func TestAccFastlyServiceSettings_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigSettingsOnComputeService(serviceName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_settings does not support Fastly service.*of type "Compute"`),
			},
		},
	})
}

func TestAccFastlyServiceSettings_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigSettingsBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_settings.test"]
						if !ok {
							return fmt.Errorf("settings resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_settings.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s", serviceID, versionNumber), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceSettings_destroyResetsDefaults verifies that destroying the resource
// resets the service version's settings back to their API defaults rather than leaving the
// last-applied values in place - settings always exist server-side, so there is nothing to
// actually delete.
func TestAccFastlyServiceSettings_destroyResetsDefaults(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	var serviceID string
	var versionNumber int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigSettingsBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_settings.test", "default_ttl", "120"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_settings.test"]
						if !ok {
							return fmt.Errorf("settings resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						_, err := fmt.Sscanf(rs.Primary.Attributes["version"], "%d", &versionNumber)
						return err
					},
				),
			},
			{
				// Removing the resource from config destroys it, which must reset the version's
				// settings back to API defaults remotely, not just drop it from state.
				Config: ConfigServiceCDNWithDomain(serviceName, domainName, 1),
				Check: func(_ *terraform.State) error {
					client, err := NewFastlyClient()
					if err != nil {
						return fmt.Errorf("error creating Fastly client: %w", err)
					}

					remote, err := client.GetSettings(context.Background(), &fastly.GetSettingsInput{
						ServiceID:      serviceID,
						ServiceVersion: versionNumber,
					})
					if err != nil {
						return fmt.Errorf("error fetching settings: %w", err)
					}

					var mismatches []string
					if fastly.ToValue(remote.DefaultHost) != settings.DefaultDefaultHost {
						mismatches = append(mismatches, fmt.Sprintf("default_host=%q, want %q", fastly.ToValue(remote.DefaultHost), settings.DefaultDefaultHost))
					}
					if int(fastly.ToValue(remote.DefaultTTL)) != settings.DefaultDefaultTTL {
						mismatches = append(mismatches, fmt.Sprintf("default_ttl=%d, want %d", fastly.ToValue(remote.DefaultTTL), settings.DefaultDefaultTTL))
					}
					if fastly.ToValue(remote.StaleIfError) != settings.DefaultStaleIfError {
						mismatches = append(mismatches, fmt.Sprintf("stale_if_error=%t, want %t", fastly.ToValue(remote.StaleIfError), settings.DefaultStaleIfError))
					}
					if int(fastly.ToValue(remote.StaleIfErrorTTL)) != settings.DefaultStaleIfErrorTTL {
						mismatches = append(mismatches, fmt.Sprintf("stale_if_error_ttl=%d, want %d", fastly.ToValue(remote.StaleIfErrorTTL), settings.DefaultStaleIfErrorTTL))
					}
					if len(mismatches) > 0 {
						return fmt.Errorf("settings did not reset to API defaults after destroy: %v", mismatches)
					}
					return nil
				},
			},
		},
	})
}
