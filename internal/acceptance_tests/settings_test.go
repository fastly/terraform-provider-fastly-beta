package acceptancetests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccFastlyServiceCDNAuto_withSettings exercises the full lifecycle of the settings nested
// block on fastly_service_cdn_auto: create, update, and removal (reset to API defaults).
func TestAccFastlyServiceCDNAuto_withSettings(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.#", "0"),
					// Initial version should be 1
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithSettings(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_host", "override.example.com"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_ttl", "120"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.http3", "true"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.stale_if_error", "true"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.stale_if_error_ttl", "600"),
					// Adding a settings block should create and activate version 2
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
			{
				// Removing the block resets settings back to API defaults.
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "3"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "3"),
					CheckSettingsMatchAPIDefaults("fastly_service_cdn_auto.test"),
				),
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_importDoesNotSurfaceSettings documents a deliberate limitation:
// unlike name-keyed collection blocks, GetSettings succeeds unconditionally for every service
// (there's no product-enablement gate to distinguish "never configured" from "at platform
// defaults"), so - unlike image_optimizer_default_settings, which can safely force a refresh on
// import - the settings block is never force-populated on import. Otherwise every import of
// every service would spuriously surface a settings block. This means a service whose settings
// were configured before import will show settings.# = 0 immediately after import; re-declaring
// the block and applying again brings it back under management.
func TestAccFastlyServiceCDNAuto_importDoesNotSurfaceSettings(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithSettings(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.#", "1"),
				),
			},
			{
				ResourceName: "fastly_service_cdn_auto.test",
				ImportState:  true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if got := states[0].Attributes["settings.#"]; got != "0" {
						return fmt.Errorf("expected settings.# to be 0 immediately after import, got %q", got)
					}
					return nil
				},
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_settingsMinimal(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				// An empty settings block must populate every attribute with its documented
				// default rather than drifting on every plan.
				Config: ConfigCDNAutoWithSettingsMinimal(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_host", ""),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_ttl", "3600"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.http3", "false"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.stale_if_error", "false"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.stale_if_error_ttl", "43200"),
				),
			},
			{
				Config:   ConfigCDNAutoWithSettingsMinimal(serviceName, domainName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_settingsUpdated(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithSettings(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_ttl", "120"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.http3", "true"),
				),
			},
			{
				// This is an in-place update: settings has no name/identity field, so there is
				// no delete+recreate distinction to make.
				Config: ConfigCDNAutoWithSettingsUpdated(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_host", "other.example.com"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_ttl", "300"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.http3", "false"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.stale_if_error", "false"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.stale_if_error_ttl", "1200"),
				),
			},
			{
				// Removing every optional attribute from config must clear them back to their
				// documented defaults remotely, not just hide the drift in state.
				Config: ConfigCDNAutoWithSettingsMinimal(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_host", ""),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.default_ttl", "3600"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.http3", "false"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.stale_if_error", "false"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "settings.0.stale_if_error_ttl", "43200"),
				),
			},
			{
				Config:   ConfigCDNAutoWithSettingsMinimal(serviceName, domainName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_settingsTooMany(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigCDNAutoWithTooManySettings(serviceName, domainName),
				ExpectError: regexp.MustCompile(`(?s)Attribute settings.*must contain at most 1`),
			},
		},
	})
}
