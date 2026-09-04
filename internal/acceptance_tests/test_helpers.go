package acceptancetests

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/provider"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/imageoptimizerdefaultsettings"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/settings"
)

// ProtoV6ProviderFactories returns the provider factories for acceptance tests.
func ProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"fastly": providerserver.NewProtocol6WithError(provider.New()),
	}
}

// PreCheck ensures the required environment variables are set for acceptance tests.
func PreCheck(t *testing.T) {
	if v := os.Getenv("FASTLY_API_TOKEN"); v == "" {
		t.Fatal("FASTLY_API_TOKEN must be set for acceptance tests")
	}
}

// NewFastlyClient creates a new Fastly API client for testing.
func NewFastlyClient() (*fastly.Client, error) {
	apiToken := os.Getenv("FASTLY_API_TOKEN")
	if apiToken == "" {
		return nil, fmt.Errorf("FASTLY_API_TOKEN environment variable must be set")
	}
	return fastly.NewClient(apiToken)
}

// serviceDestroyCheckAttempts and serviceDestroyCheckInterval bound the retry loop in
// CheckServiceDestroy, which tolerates the Fastly API's soft-delete taking a moment to become
// visible on a subsequent read - most noticeable when many acceptance tests run in parallel.
const (
	serviceDestroyCheckAttempts = 5
	serviceDestroyCheckInterval = 2 * time.Second
)

// CheckServiceDestroy returns a TestCheckFunc that verifies a service resource was destroyed.
func CheckServiceDestroy(resourceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != resourceType {
				continue
			}

			var lastErr error
			for attempt := 1; attempt <= serviceDestroyCheckAttempts; attempt++ {
				lastErr = checkServiceDestroyed(client, rs.Primary.ID)
				if lastErr == nil {
					break
				}
				if attempt < serviceDestroyCheckAttempts {
					time.Sleep(serviceDestroyCheckInterval)
				}
			}
			if lastErr != nil {
				return lastErr
			}
		}

		return nil
	}
}

func checkServiceDestroyed(client *fastly.Client, serviceID string) error {
	svc, err := client.GetService(context.Background(), &fastly.GetServiceInput{
		ServiceID: serviceID,
	})

	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("error checking if service was destroyed: %w", err)
	}

	// Service exists - check if it's soft-deleted
	if svc != nil && svc.DeletedAt != nil {
		return nil
	}

	return fmt.Errorf("service %s still exists", serviceID)
}

// CheckServiceExists returns a TestCheckFunc that verifies a service resource exists.
func CheckServiceExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no service ID is set")
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		_, err = client.GetService(context.Background(), &fastly.GetServiceInput{
			ServiceID: rs.Primary.ID,
		})
		if err != nil {
			return fmt.Errorf("error fetching service: %w", err)
		}

		return nil
	}
}

// CheckGzipFieldClearedRemotely returns a TestCheckFunc that fetches the named gzip
// configuration directly from the Fastly API (bypassing Terraform state) and fails if
// content_types or extensions still holds staleValue. This exists because the provider's
// custom Read (gzip.ReadForVersionWithPlan) normalizes an unset field against the plan
// regardless of what's actually stored remotely, so a state-only check can pass even if
// the remote value was never cleared - see the servicecdnauto gzip.ReconcileWithPrevious fix.
func CheckGzipFieldClearedRemotely(resourceName, gzipName, field, staleValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		version, err := strconv.Atoi(rs.Primary.Attributes["active_version"])
		if err != nil {
			return fmt.Errorf("error parsing active_version: %w", err)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		g, err := client.GetGzip(context.Background(), &fastly.GetGzipInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           gzipName,
		})
		if err != nil {
			return fmt.Errorf("error fetching gzip %q: %w", gzipName, err)
		}

		var raw string
		switch field {
		case "content_types":
			raw = fastly.ToValue(g.ContentTypes)
		case "extensions":
			raw = fastly.ToValue(g.Extensions)
		default:
			return fmt.Errorf("unknown gzip field %q", field)
		}

		if raw == staleValue {
			return fmt.Errorf("gzip %q %s still equals the stale value %q in Fastly - the remote value was not actually cleared", gzipName, field, staleValue)
		}

		return nil
	}
}

// CheckConditionTypeInFastly returns a TestCheckFunc that fetches a condition directly from the
// Fastly API (bypassing Terraform state) and verifies its type matches expectedType. This exists
// because the Fastly API doesn't support updating a condition's type via PUT (see condition's
// ops.Update) - the provider works around this with a delete+create, and Terraform state alone
// wouldn't catch a bug where the recreate silently failed to apply the new type remotely.
func CheckConditionTypeInFastly(resourceName, conditionName, expectedType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		version, err := strconv.Atoi(rs.Primary.Attributes["active_version"])
		if err != nil {
			return fmt.Errorf("error parsing active_version: %w", err)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		c, err := client.GetCondition(context.Background(), &fastly.GetConditionInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           conditionName,
		})
		if err != nil {
			return fmt.Errorf("error fetching condition %q: %w", conditionName, err)
		}

		if fastly.ToValue(c.Type) != expectedType {
			return fmt.Errorf("condition %q has type %q in Fastly, expected %q", conditionName, fastly.ToValue(c.Type), expectedType)
		}

		return nil
	}
}

// CheckImageOptimizerDefaultSettingsMatchAPIDefaults returns a TestCheckFunc that fetches Image
// Optimizer default settings directly from the Fastly API (bypassing Terraform state) and fails
// if any field still holds a previously-configured, non-default value. This exists because the
// read path only surfaces image_optimizer_default_settings when the block is present in config,
// so a state-only check of the block being absent can pass even if the remote settings were
// never actually reset.
func CheckImageOptimizerDefaultSettingsMatchAPIDefaults(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		version, err := strconv.Atoi(rs.Primary.Attributes["active_version"])
		if err != nil {
			return fmt.Errorf("error parsing active_version: %w", err)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		settings, err := client.GetImageOptimizerDefaultSettings(context.Background(), &fastly.GetImageOptimizerDefaultSettingsInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
		})
		if err != nil {
			return fmt.Errorf("error fetching Image Optimizer default settings: %w", err)
		}
		if settings == nil {
			return fmt.Errorf("expected Image Optimizer default settings to be populated since Image Optimizer remains enabled, got nil")
		}

		var mismatches []string
		if settings.ResizeFilter != imageoptimizerdefaultsettings.DefaultResizeFilter {
			mismatches = append(mismatches, fmt.Sprintf("resize_filter=%q, want %q", settings.ResizeFilter, imageoptimizerdefaultsettings.DefaultResizeFilter))
		}
		if settings.Webp != imageoptimizerdefaultsettings.DefaultWebp {
			mismatches = append(mismatches, fmt.Sprintf("webp=%v, want %v", settings.Webp, imageoptimizerdefaultsettings.DefaultWebp))
		}
		if settings.WebpQuality != imageoptimizerdefaultsettings.DefaultWebpQuality {
			mismatches = append(mismatches, fmt.Sprintf("webp_quality=%d, want %d", settings.WebpQuality, imageoptimizerdefaultsettings.DefaultWebpQuality))
		}
		if settings.JpegType != imageoptimizerdefaultsettings.DefaultJpegType {
			mismatches = append(mismatches, fmt.Sprintf("jpeg_type=%q, want %q", settings.JpegType, imageoptimizerdefaultsettings.DefaultJpegType))
		}
		if settings.JpegQuality != imageoptimizerdefaultsettings.DefaultJpegQuality {
			mismatches = append(mismatches, fmt.Sprintf("jpeg_quality=%d, want %d", settings.JpegQuality, imageoptimizerdefaultsettings.DefaultJpegQuality))
		}
		if settings.Upscale != imageoptimizerdefaultsettings.DefaultUpscale {
			mismatches = append(mismatches, fmt.Sprintf("upscale=%v, want %v", settings.Upscale, imageoptimizerdefaultsettings.DefaultUpscale))
		}
		if settings.AllowVideo != imageoptimizerdefaultsettings.DefaultAllowVideo {
			mismatches = append(mismatches, fmt.Sprintf("allow_video=%v, want %v", settings.AllowVideo, imageoptimizerdefaultsettings.DefaultAllowVideo))
		}

		if len(mismatches) > 0 {
			return fmt.Errorf("image optimizer default settings were not reset to API defaults in Fastly: %s", strings.Join(mismatches, ", "))
		}

		return nil
	}
}

// CheckSettingsMatchAPIDefaults returns a TestCheckFunc that fetches the general settings (and
// HTTP/3 status) directly from the Fastly API (bypassing Terraform state) and fails if any
// field still holds a previously-configured, non-default value. This exists because the read
// path only surfaces the settings block when it's present in config, so a state-only check of
// the block being absent can pass even if the remote settings were never actually reset.
func CheckSettingsMatchAPIDefaults(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		version, err := strconv.Atoi(rs.Primary.Attributes["active_version"])
		if err != nil {
			return fmt.Errorf("error parsing active_version: %w", err)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		remote, err := client.GetSettings(context.Background(), &fastly.GetSettingsInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
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
			mismatches = append(mismatches, fmt.Sprintf("stale_if_error=%v, want %v", fastly.ToValue(remote.StaleIfError), settings.DefaultStaleIfError))
		}
		if int(fastly.ToValue(remote.StaleIfErrorTTL)) != settings.DefaultStaleIfErrorTTL {
			mismatches = append(mismatches, fmt.Sprintf("stale_if_error_ttl=%d, want %d", fastly.ToValue(remote.StaleIfErrorTTL), settings.DefaultStaleIfErrorTTL))
		}

		_, http3Err := client.GetHTTP3(context.Background(), &fastly.GetHTTP3Input{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
		})
		var http3Enabled bool
		switch {
		case http3Err == nil:
			http3Enabled = true
		case errors.IsNotFound(http3Err):
			http3Enabled = false
		default:
			return fmt.Errorf("error fetching HTTP/3 status: %w", http3Err)
		}
		if http3Enabled != settings.DefaultHTTP3 {
			mismatches = append(mismatches, fmt.Sprintf("http3=%v, want %v", http3Enabled, settings.DefaultHTTP3))
		}

		if len(mismatches) > 0 {
			return fmt.Errorf("settings were not reset to API defaults in Fastly: %s", strings.Join(mismatches, ", "))
		}

		return nil
	}
}

// GetPackagePath returns the path to the valid.tar.gz test package
// Assumes tests are always run from the acceptance_tests package directory.
func GetPackagePath() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("failed to get working directory: %v", err))
	}
	return filepath.Join(wd, "fixtures", "packages", "valid.tar.gz")
}

// AddACLEntry adds an ACL entry to the specified ACL via the raw API client, deliberately
// bypassing fastly_service_cdn_acl_entries so the entry is unmanaged by Terraform. This is used
// as a test side-effect to populate ACLs for testing force_destroy behavior, mirroring
// AddDictionaryItem below. Returns a TestCheckFunc.
func AddACLEntry(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		serviceID := rs.Primary.ID
		aclName := rs.Primary.Attributes["acl.0.name"]
		activeVersion := rs.Primary.Attributes["active_version"]

		if serviceID == "" || aclName == "" || activeVersion == "" {
			return fmt.Errorf("service_id, acl name, or active_version not set in state")
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		version := new(int)
		if _, err := fmt.Sscanf(activeVersion, "%d", version); err != nil {
			return fmt.Errorf("error parsing active_version: %w", err)
		}

		acl, err := client.GetACL(context.Background(), &fastly.GetACLInput{
			ServiceID:      serviceID,
			ServiceVersion: *version,
			Name:           aclName,
		})
		if err != nil {
			return fmt.Errorf("error fetching ACL %s: %w", aclName, err)
		}

		ip := "192.168.0.1"
		_, err = client.CreateACLEntry(context.Background(), &fastly.CreateACLEntryInput{
			ServiceID: serviceID,
			ACLID:     *acl.ACLID,
			IP:        &ip,
		})
		if err != nil {
			return fmt.Errorf("error adding entry to ACL %s on service %s: %w", *acl.ACLID, serviceID, err)
		}

		return nil
	}
}

// AddDictionaryItem adds an item to the dictionary at the given state attribute prefix
// (e.g. "dictionary.0"). This is used as a test side-effect to populate a dictionary for
// testing force_destroy behavior. Returns a TestCheckFunc.
func AddDictionaryItem(resourceName, dictionaryAttrPrefix string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		serviceID := rs.Primary.ID
		dictionaryID := rs.Primary.Attributes[dictionaryAttrPrefix+".dictionary_id"]

		if serviceID == "" || dictionaryID == "" {
			return fmt.Errorf("service_id or dictionary_id not set in state")
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		_, err = client.CreateDictionaryItem(context.Background(), &fastly.CreateDictionaryItemInput{
			ServiceID:    serviceID,
			DictionaryID: dictionaryID,
			ItemKey:      new("test-key"),
			ItemValue:    new("test-value"),
		})
		if err != nil {
			return fmt.Errorf("error adding item to dictionary %s on service %s: %w", dictionaryID, serviceID, err)
		}

		return nil
	}
}

// Configuration helpers for CDN Auto service

// ConfigCDNAutoBasic returns a basic CDN auto service config with a single domain.
func ConfigCDNAutoBasic(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
	)
}

// ConfigCDNAutoWithBackend returns a CDN auto service config with a domain and backend.
func ConfigCDNAutoWithBackend(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"BACKEND_NAME": backendName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
	)
}

// ConfigCDNAutoMultipleBackends returns a CDN auto service config with multiple backends.
func ConfigCDNAutoMultipleBackends(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_multiple.tf",
	)
}

// ConfigCDNAutoUnsortedBackendAndDomainBlocks returns a CDN auto service config
// with backend and domain blocks declared in non-sorted order.
func ConfigCDNAutoUnsortedBackendAndDomainBlocks(serviceName, domainBName, domainAName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":  serviceName,
			"DOMAIN_B_NAME": domainBName,
			"DOMAIN_A_NAME": domainAName,
		},
		"internal/acceptance_tests/blocks/domain_multiple_unsorted.tf",
		"internal/acceptance_tests/blocks/backend_multiple_unsorted.tf",
	)
}

// ConfigCDNAutoReversedBackendAndDomainBlocks returns a CDN auto service config
// with the same backend and domain blocks as ConfigCDNAutoUnsortedBackendAndDomainBlocks,
// but declared in the reverse order.
func ConfigCDNAutoReversedBackendAndDomainBlocks(serviceName, domainBName, domainAName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":  serviceName,
			"DOMAIN_B_NAME": domainBName,
			"DOMAIN_A_NAME": domainAName,
		},
		"internal/acceptance_tests/blocks/domain_multiple_reversed.tf",
		"internal/acceptance_tests/blocks/backend_multiple_reversed.tf",
	)
}

// imageOptimizerProductEnablement returns a fastly_service_product_image_optimizer resource
// block that enables Image Optimizer on the given service resource reference (e.g.
// "fastly_service_cdn_auto.test"). Image Optimizer default settings can only be persisted once
// the product has been enabled on the service.
func imageOptimizerProductEnablement(serviceRef string) string {
	return fmt.Sprintf(`
resource "fastly_service_product_image_optimizer" "image_optimizer" {
  service_id = %s.id
}
`, serviceRef)
}

// ConfigCDNAutoImageOptimizerEnabled returns a CDN auto service config with a domain and Image
// Optimizer enabled via fastly_service_product_image_optimizer, but no
// image_optimizer_default_settings block.
func ConfigCDNAutoImageOptimizerEnabled(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
	) + imageOptimizerProductEnablement("fastly_service_cdn_auto.test")
}

// ConfigCDNAutoWithImageOptimizerDefaultSettings returns a CDN auto service config with a
// domain, Image Optimizer enabled, and default (all-default-value) Image Optimizer default
// settings.
func ConfigCDNAutoWithImageOptimizerDefaultSettings(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/image_optimizer_default_settings_single.tf",
	) + imageOptimizerProductEnablement("fastly_service_cdn_auto.test")
}

// ConfigCDNAutoWithImageOptimizerDefaultSettingsUpdated returns a CDN auto service config with
// a domain, Image Optimizer enabled, and non-default Image Optimizer default settings.
func ConfigCDNAutoWithImageOptimizerDefaultSettingsUpdated(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/image_optimizer_default_settings_updated.tf",
	) + imageOptimizerProductEnablement("fastly_service_cdn_auto.test")
}

// ConfigCDNAutoWithSettings returns a CDN auto service config with a domain and a settings
// block with every optional attribute set to a non-default value.
func ConfigCDNAutoWithSettings(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/settings_single.tf",
	)
}

// ConfigCDNAutoWithSettingsMinimal returns a CDN auto service config with an empty settings
// block, exercising the computed defaults for every attribute.
func ConfigCDNAutoWithSettingsMinimal(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/settings_minimal.tf",
	)
}

// ConfigCDNAutoWithSettingsUpdated returns a CDN auto service config with a settings block
// whose attribute values differ from ConfigCDNAutoWithSettings.
func ConfigCDNAutoWithSettingsUpdated(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/settings_updated.tf",
	)
}

// ConfigCDNAutoWithTooManySettings returns a CDN auto service config with two settings blocks,
// exercising the schema-level listvalidator.SizeAtMost(1) plan-time check.
func ConfigCDNAutoWithTooManySettings(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/settings_too_many.tf",
	)
}

// ConfigCDNAutoWithACL returns a CDN auto service config with a domain and ACL.
func ConfigCDNAutoWithACL(serviceName, domainName, aclName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"ACL_NAME":     aclName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/acl_single.tf",
	)
}

// ConfigCDNAutoWithBackendAndACL returns a CDN auto service config with domain, backend, and ACL.
func ConfigCDNAutoWithBackendAndACL(serviceName, domainName, backendName, aclName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"BACKEND_NAME": backendName,
			"ACL_NAME":     aclName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/acl_single.tf",
	)
}

// ConfigCDNAutoWithMultipleACLs returns a CDN auto service config with multiple ACLs.
func ConfigCDNAutoWithMultipleACLs(serviceName, domainName, aclName1, aclName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"ACL_NAME_1":   aclName1,
			"ACL_NAME_2":   aclName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/acl_multi.tf",
	)
}

// ConfigCDNAutoWithACLForceDestroy returns a CDN auto service config with an ACL that has force_destroy enabled.
func ConfigCDNAutoWithACLForceDestroy(serviceName, domainName, aclName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"ACL_NAME":     aclName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/acl_with_force_destroy.tf",
	)
}

// ConfigCDNAutoWithDictionary returns a CDN auto service config with a domain and a dictionary.
func ConfigCDNAutoWithDictionary(serviceName, domainName, dictionaryName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"DICTIONARY_NAME": dictionaryName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/dictionary_single.tf",
	)
}

// ConfigCDNAutoWithDictionaryWriteOnly returns a CDN auto service config with a domain and a
// write_only dictionary.
func ConfigCDNAutoWithDictionaryWriteOnly(serviceName, domainName, dictionaryName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"DICTIONARY_NAME": dictionaryName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/dictionary_write_only.tf",
	)
}

// ConfigCDNAutoWithDictionaryWriteOnlyForceDestroy returns a CDN auto service config with a
// domain and a write_only dictionary that has force_destroy enabled.
func ConfigCDNAutoWithDictionaryWriteOnlyForceDestroy(serviceName, domainName, dictionaryName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"DICTIONARY_NAME": dictionaryName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/dictionary_write_only_force_destroy.tf",
	)
}

// ConfigCDNAutoWithMultipleDictionaries returns a CDN auto service config with multiple dictionaries.
func ConfigCDNAutoWithMultipleDictionaries(serviceName, domainName, dictionaryName1, dictionaryName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"DICTIONARY_NAME_1": dictionaryName1,
			"DICTIONARY_NAME_2": dictionaryName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/dictionary_multi.tf",
	)
}

// ConfigCDNAutoWithDictionaryForceDestroy returns a CDN auto service config with a dictionary
// that has force_destroy enabled.
func ConfigCDNAutoWithDictionaryForceDestroy(serviceName, domainName, dictionaryName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"DICTIONARY_NAME": dictionaryName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/dictionary_with_force_destroy.tf",
	)
}

// ConfigCDNAutoWithHealthCheck returns a CDN auto service config with a domain and a health check.
func ConfigCDNAutoWithHealthCheck(serviceName, domainName, healthCheckName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"HEALTHCHECK_NAME": healthCheckName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/healthcheck_single.tf",
	)
}

// ConfigCDNAutoWithMultipleHealthChecks returns a CDN auto service config with multiple health
// checks, the second of which sets every optional attribute to a non-default value.
func ConfigCDNAutoWithMultipleHealthChecks(serviceName, domainName, healthCheckName1, healthCheckName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"HEALTHCHECK_NAME_1": healthCheckName1,
			"HEALTHCHECK_NAME_2": healthCheckName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/healthcheck_multi.tf",
	)
}

// ConfigCDNAutoWithHealthCheckUpdated returns a CDN auto service config with a health check whose
// optional attributes are all set to non-default values, for use as the second step of an update test.
func ConfigCDNAutoWithHealthCheckUpdated(serviceName, domainName, healthCheckName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"HEALTHCHECK_NAME": healthCheckName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/healthcheck_updated.tf",
	)
}

// ConfigCDNAutoWithHealthCheckAndBackend returns a CDN auto service config with a health check
// and a backend that references it by name, confirming health checks are reconciled before
// backend so the reference resolves within the same service version.
func ConfigCDNAutoWithHealthCheckAndBackend(serviceName, domainName, healthCheckName, backendName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"HEALTHCHECK_NAME": healthCheckName,
			"BACKEND_NAME":     backendName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/healthcheck_single.tf",
		"internal/acceptance_tests/blocks/backend_with_healthcheck.tf",
	)
}

// ConfigCDNAutoWithHeader returns a CDN auto service config with a domain and a header.
func ConfigCDNAutoWithHeader(serviceName, domainName, headerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"HEADER_NAME":  headerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/header_single.tf",
	)
}

// ConfigCDNAutoWithHeaderUpdated returns a CDN auto service config with the same header name but
// a different action, type, destination, source, priority, and ignore_if_set, for use as the
// second step of an update test.
func ConfigCDNAutoWithHeaderUpdated(serviceName, domainName, headerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"HEADER_NAME":  headerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/header_updated.tf",
	)
}

// ConfigCDNAutoWithHeaderRequestCondition returns a CDN auto service config with a header whose
// request_condition references a real nested REQUEST-type condition block.
func ConfigCDNAutoWithHeaderRequestCondition(serviceName, domainName, headerName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":   serviceName,
			"DOMAIN_NAME":    domainName,
			"HEADER_NAME":    headerName,
			"CONDITION_NAME": conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_single.tf",
		"internal/acceptance_tests/blocks/header_with_request_condition.tf",
	)
}

// ConfigCDNAutoWithHeaderInvalidAction returns a CDN auto service config with a header whose
// action is not one of the accepted values, exercising the schema-level stringvalidator.OneOf
// plan-time check.
func ConfigCDNAutoWithHeaderInvalidAction(serviceName, domainName, headerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"HEADER_NAME":  headerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/header_invalid_action.tf",
	)
}

// ConfigCDNAutoWithRateLimiter returns a CDN auto service config with a domain and a rate
// limiter.
func ConfigCDNAutoWithRateLimiter(serviceName, domainName, rateLimiterName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"RATE_LIMITER_NAME": rateLimiterName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_single.tf",
	)
}

// ConfigCDNAutoWithRateLimiterResponseCleared returns a CDN auto service config with the same
// rate limiter name as ConfigCDNAutoWithRateLimiter, but with action changed to log_only and
// response removed.
func ConfigCDNAutoWithRateLimiterResponseCleared(serviceName, domainName, rateLimiterName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"RATE_LIMITER_NAME": rateLimiterName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_response_cleared.tf",
	)
}

// ConfigCDNAutoWithRateLimiterMinimal returns a CDN auto service config with a rate limiter
// that leaves feature_revision, logger_type, response, response_object_name, and
// uri_dictionary_name unset.
func ConfigCDNAutoWithRateLimiterMinimal(serviceName, domainName, rateLimiterName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"RATE_LIMITER_NAME": rateLimiterName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_minimal.tf",
	)
}

// ConfigCDNAutoWithRateLimiterUpdated returns a CDN auto service config with the same rate
// limiter name but different action/client_key/http_methods/penalty_box_duration/rps_limit/
// window_size values.
func ConfigCDNAutoWithRateLimiterUpdated(serviceName, domainName, rateLimiterName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"RATE_LIMITER_NAME": rateLimiterName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_updated.tf",
	)
}

// ConfigCDNAutoWithMultipleRateLimiters returns a CDN auto service config with two rate limiters.
func ConfigCDNAutoWithMultipleRateLimiters(serviceName, domainName, rateLimiterName1, rateLimiterName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"RATE_LIMITER_NAME_1": rateLimiterName1,
			"RATE_LIMITER_NAME_2": rateLimiterName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_multi.tf",
	)
}

// ConfigCDNAutoWithRateLimiterDictionary returns a CDN auto service config with a rate limiter
// whose uri_dictionary_name references a real nested dictionary block.
func ConfigCDNAutoWithRateLimiterDictionary(serviceName, domainName, rateLimiterName, dictionaryName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"RATE_LIMITER_NAME": rateLimiterName,
			"DICTIONARY_NAME":   dictionaryName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_with_dictionary.tf",
	)
}

// ConfigCDNAutoWithRateLimiterDictionaryCleared returns a CDN auto service config with the same
// rate limiter and dictionary as ConfigCDNAutoWithRateLimiterDictionary, but with the rate
// limiter's uri_dictionary_name unset - the dictionary block itself is left in place, only the
// reference to it is cleared.
func ConfigCDNAutoWithRateLimiterDictionaryCleared(serviceName, domainName, rateLimiterName, dictionaryName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"RATE_LIMITER_NAME": rateLimiterName,
			"DICTIONARY_NAME":   dictionaryName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_with_dictionary_cleared.tf",
	)
}

// ConfigCDNAutoWithRateLimiterDictionaryRemoved returns a CDN auto service config with the same
// rate limiter as ConfigCDNAutoWithRateLimiterDictionary, but with the dictionary block removed
// entirely - the rate limiter's uri_dictionary_name is left unchanged, still naming the
// now-unmanaged dictionary.
func ConfigCDNAutoWithRateLimiterDictionaryRemoved(serviceName, domainName, rateLimiterName, dictionaryName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"RATE_LIMITER_NAME": rateLimiterName,
			"DICTIONARY_NAME":   dictionaryName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_dictionary_removed.tf",
	)
}

// ConfigCDNAutoWithRateLimiterResponseObject returns a CDN auto service config with a rate
// limiter whose response_object_name references a real nested response_object block.
func ConfigCDNAutoWithRateLimiterResponseObject(serviceName, domainName, rateLimiterName, responseObjectName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"RATE_LIMITER_NAME":    rateLimiterName,
			"RESPONSE_OBJECT_NAME": responseObjectName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/rate_limiter_with_response_object.tf",
	)
}

// ConfigCDNAutoWithDirector returns a CDN auto service config with a domain, a backend, and a
// director mapped to that backend.
func ConfigCDNAutoWithDirector(serviceName, domainName, backendName, directorName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":  serviceName,
			"DOMAIN_NAME":   domainName,
			"BACKEND_NAME":  backendName,
			"DIRECTOR_NAME": directorName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/director_single.tf",
	)
}

// ConfigCDNAutoWithDirectorUpdated returns a CDN auto service config with the same director and
// backend names as ConfigCDNAutoWithDirector, but with comment/quorum/retries/shield/type changed.
func ConfigCDNAutoWithDirectorUpdated(serviceName, domainName, backendName, directorName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":  serviceName,
			"DOMAIN_NAME":   domainName,
			"BACKEND_NAME":  backendName,
			"DIRECTOR_NAME": directorName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/director_updated.tf",
	)
}

// ConfigCDNAutoWithDirectorBackendSwapped returns a CDN auto service config with the same
// director name as ConfigCDNAutoWithDirector, but the original backend removed entirely and a
// new backend added and referenced instead - exercising the ordering between backend
// create/delete and director reconciliation (see servicecdnauto's Update).
func ConfigCDNAutoWithDirectorBackendSwapped(serviceName, domainName, backendName2, directorName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":   serviceName,
			"DOMAIN_NAME":    domainName,
			"BACKEND_NAME_2": backendName2,
			"DIRECTOR_NAME":  directorName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/director_backend_swapped.tf",
	)
}

// ConfigCDNAutoWithTwoOrderedDirectors returns a CDN auto service config with two directors,
// directorA (type "hash") followed by directorB (type omitted, defaults to "random").
func ConfigCDNAutoWithTwoOrderedDirectors(serviceName, domainName, backendNameA, backendNameB, directorNameA, directorNameB string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"BACKEND_NAME_A":  backendNameA,
			"BACKEND_NAME_B":  backendNameB,
			"DIRECTOR_NAME_A": directorNameA,
			"DIRECTOR_NAME_B": directorNameB,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/director_two_ordered.tf",
	)
}

// ConfigCDNAutoWithDirectorInsertedAhead returns a CDN auto service config with the same
// directorA/directorB names and backends as ConfigCDNAutoWithTwoOrderedDirectors, but with a new
// directorC (and its backend) inserted ahead of directorA in the config, and the explicit
// type = "hash" removed from directorA. directorA's type should reset to the default ("random")
// on omit, not stick to "hash" - see the typeStickyDefault doc comment in
// internal/resources/director/schema.go.
func ConfigCDNAutoWithDirectorInsertedAhead(serviceName, domainName, backendNameA, backendNameB, backendNameC, directorNameA, directorNameB, directorNameC string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"BACKEND_NAME_A":  backendNameA,
			"BACKEND_NAME_B":  backendNameB,
			"BACKEND_NAME_C":  backendNameC,
			"DIRECTOR_NAME_A": directorNameA,
			"DIRECTOR_NAME_B": directorNameB,
			"DIRECTOR_NAME_C": directorNameC,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/director_reordered_insert.tf",
	)
}

// ConfigCDNAutoWithDirectorNegativeRetries returns a CDN auto service config with a director
// whose retries is negative, exercising the retries int64validator.AtLeast(0) plan-time check.
func ConfigCDNAutoWithDirectorNegativeRetries(serviceName, domainName, backendName, directorName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":  serviceName,
			"DOMAIN_NAME":   domainName,
			"BACKEND_NAME":  backendName,
			"DIRECTOR_NAME": directorName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/director_negative_retries.tf",
	)
}

// ConfigCDNAutoWithGzip returns a CDN auto service config with a domain and a gzip configuration.
func ConfigCDNAutoWithGzip(serviceName, domainName, gzipName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"GZIP_NAME":    gzipName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/gzip_single.tf",
	)
}

// ConfigCDNAutoWithGzipEmptyLists returns a CDN auto service config with a gzip configuration
// that explicitly sets content_types and extensions to empty lists.
func ConfigCDNAutoWithGzipEmptyLists(serviceName, domainName, gzipName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"GZIP_NAME":    gzipName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/gzip_empty_lists.tf",
	)
}

// ConfigCDNAutoWithGzipContentTypesRemoved returns a CDN auto service config with a gzip
// configuration whose content_types attribute has been removed, leaving extensions set.
func ConfigCDNAutoWithGzipContentTypesRemoved(serviceName, domainName, gzipName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"GZIP_NAME":    gzipName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/gzip_content_types_removed.tf",
	)
}

// ConfigCDNAutoWithGzipAllRemoved returns a CDN auto service config with a gzip configuration
// whose content_types and extensions attributes have both been removed.
func ConfigCDNAutoWithGzipAllRemoved(serviceName, domainName, gzipName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"GZIP_NAME":    gzipName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/gzip_all_removed.tf",
	)
}

// ConfigCDNAutoWithMultipleGzips returns a CDN auto service config with multiple gzip configurations.
func ConfigCDNAutoWithMultipleGzips(serviceName, domainName, gzipName1, gzipName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"GZIP_NAME_1":  gzipName1,
			"GZIP_NAME_2":  gzipName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/gzip_multi.tf",
	)
}

// ConfigCDNAutoWithCondition returns a CDN auto service config with a domain and a condition.
func ConfigCDNAutoWithCondition(serviceName, domainName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":   serviceName,
			"DOMAIN_NAME":    domainName,
			"CONDITION_NAME": conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_single.tf",
	)
}

// ConfigCDNAutoWithConditionUpdated returns a CDN auto service config with the same condition
// name but an updated statement and priority.
func ConfigCDNAutoWithConditionUpdated(serviceName, domainName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":   serviceName,
			"DOMAIN_NAME":    domainName,
			"CONDITION_NAME": conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_updated.tf",
	)
}

// ConfigCDNAutoWithConditionTypeChanged returns a CDN auto service config with the same
// condition name but a different type, exercising the delete+recreate path the Fastly API
// requires since it doesn't support updating a condition's type in place.
func ConfigCDNAutoWithConditionTypeChanged(serviceName, domainName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":   serviceName,
			"DOMAIN_NAME":    domainName,
			"CONDITION_NAME": conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_type_changed.tf",
	)
}

// ConfigCDNAutoWithMultipleConditions returns a CDN auto service config with two conditions.
func ConfigCDNAutoWithMultipleConditions(serviceName, domainName, conditionName1, conditionName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"CONDITION_NAME_1": conditionName1,
			"CONDITION_NAME_2": conditionName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_multi.tf",
	)
}

// ConfigCDNAutoWithBackendRequestCondition returns a CDN auto service config with a backend
// whose request_condition references a real nested condition block.
func ConfigCDNAutoWithBackendRequestCondition(serviceName, domainName, backendName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":   serviceName,
			"DOMAIN_NAME":    domainName,
			"BACKEND_NAME":   backendName,
			"CONDITION_NAME": conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_single.tf",
		"internal/acceptance_tests/blocks/backend_with_request_condition.tf",
	)
}

// ConfigCDNAutoWithGzipCacheCondition returns a CDN auto service config with a gzip
// configuration whose cache_condition references a real nested CACHE-type condition block.
func ConfigCDNAutoWithGzipCacheCondition(serviceName, domainName, gzipName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":   serviceName,
			"DOMAIN_NAME":    domainName,
			"GZIP_NAME":      gzipName,
			"CONDITION_NAME": conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_cache.tf",
		"internal/acceptance_tests/blocks/gzip_with_cache_condition.tf",
	)
}

// ConfigCDNAutoWithCacheSetting returns a CDN auto service config with a domain and a cache
// setting.
func ConfigCDNAutoWithCacheSetting(serviceName, domainName, cacheSettingName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"CACHE_SETTING_NAME": cacheSettingName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/cache_setting_single.tf",
	)
}

// ConfigCDNAutoWithCacheSettingMinimal returns a CDN auto service config with a cache setting
// that leaves action, ttl, and stale_ttl unset.
func ConfigCDNAutoWithCacheSettingMinimal(serviceName, domainName, cacheSettingName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"CACHE_SETTING_NAME": cacheSettingName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/cache_setting_minimal.tf",
	)
}

// ConfigCDNAutoWithCacheSettingUpdated returns a CDN auto service config with the same cache
// setting name but different action, ttl, and stale_ttl values.
func ConfigCDNAutoWithCacheSettingUpdated(serviceName, domainName, cacheSettingName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"CACHE_SETTING_NAME": cacheSettingName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/cache_setting_updated.tf",
	)
}

// ConfigCDNAutoWithMultipleCacheSettings returns a CDN auto service config with two cache settings.
func ConfigCDNAutoWithMultipleCacheSettings(serviceName, domainName, cacheSettingName1, cacheSettingName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"CACHE_SETTING_NAME_1": cacheSettingName1,
			"CACHE_SETTING_NAME_2": cacheSettingName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/cache_setting_multi.tf",
	)
}

// ConfigCDNAutoWithCacheSettingCacheCondition returns a CDN auto service config with a cache
// setting whose cache_condition references a real nested CACHE-type condition block.
func ConfigCDNAutoWithCacheSettingCacheCondition(serviceName, domainName, cacheSettingName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"CACHE_SETTING_NAME": cacheSettingName,
			"CONDITION_NAME":     conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_cache.tf",
		"internal/acceptance_tests/blocks/cache_setting_with_cache_condition.tf",
	)
}

func ConfigCDNAutoWithRequestSetting(serviceName, domainName, requestSettingName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"REQUEST_SETTING_NAME": requestSettingName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/request_setting_single.tf",
	)
}

// ConfigCDNAutoWithRequestSettingMinimal returns a CDN auto service config with a request
// setting that leaves every optional attribute unset.
func ConfigCDNAutoWithRequestSettingMinimal(serviceName, domainName, requestSettingName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"REQUEST_SETTING_NAME": requestSettingName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/request_setting_minimal.tf",
	)
}

// ConfigCDNAutoWithRequestSettingUpdated returns a CDN auto service config with the same
// request setting name but different attribute values.
func ConfigCDNAutoWithRequestSettingUpdated(serviceName, domainName, requestSettingName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"REQUEST_SETTING_NAME": requestSettingName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/request_setting_updated.tf",
	)
}

// ConfigCDNAutoWithMultipleRequestSettings returns a CDN auto service config with two request
// settings, each scoped to its own request_condition - Fastly rejects more than one request
// setting sharing the same request_condition value (including the unset/blank default).
func ConfigCDNAutoWithMultipleRequestSettings(serviceName, domainName, requestSettingName1, requestSettingName2, conditionName1, conditionName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"REQUEST_SETTING_NAME_1": requestSettingName1,
			"REQUEST_SETTING_NAME_2": requestSettingName2,
			"CONDITION_NAME_1":       conditionName1,
			"CONDITION_NAME_2":       conditionName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/request_setting_multi.tf",
	)
}

// ConfigCDNAutoWithRequestSettingInvalidAction returns a CDN auto service config with a request
// setting whose action is not one of the accepted values, exercising the schema-level
// stringvalidator.OneOfCaseInsensitive plan-time check.
func ConfigCDNAutoWithRequestSettingInvalidAction(serviceName, domainName, requestSettingName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"REQUEST_SETTING_NAME": requestSettingName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/request_setting_invalid_action.tf",
	)
}

// ConfigCDNAutoWithRequestSettingRequestCondition returns a CDN auto service config with a
// request setting whose request_condition references a real nested REQUEST-type condition block.
func ConfigCDNAutoWithRequestSettingRequestCondition(serviceName, domainName, requestSettingName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"REQUEST_SETTING_NAME": requestSettingName,
			"CONDITION_NAME":       conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/condition_single.tf",
		"internal/acceptance_tests/blocks/request_setting_with_request_condition.tf",
	)
}

// ConfigCDNAutoWithResponseObject returns a CDN auto service config with a single response
// object with every optional attribute set to a non-default value.
func ConfigCDNAutoWithResponseObject(serviceName, domainName, responseObjectName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"RESPONSE_OBJECT_NAME": responseObjectName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/response_object_single.tf",
	)
}

// ConfigCDNAutoWithResponseObjectMinimal returns a CDN auto service config with a response
// object that leaves every optional attribute unset.
func ConfigCDNAutoWithResponseObjectMinimal(serviceName, domainName, responseObjectName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"RESPONSE_OBJECT_NAME": responseObjectName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/response_object_minimal.tf",
	)
}

// ConfigCDNAutoWithResponseObjectUpdated returns a CDN auto service config with the same
// response object name but different attribute values.
func ConfigCDNAutoWithResponseObjectUpdated(serviceName, domainName, responseObjectName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"RESPONSE_OBJECT_NAME": responseObjectName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/response_object_updated.tf",
	)
}

// ConfigCDNAutoWithMultipleResponseObjects returns a CDN auto service config with two response
// objects.
func ConfigCDNAutoWithMultipleResponseObjects(serviceName, domainName, responseObjectName1, responseObjectName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"RESPONSE_OBJECT_NAME_1": responseObjectName1,
			"RESPONSE_OBJECT_NAME_2": responseObjectName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/response_object_multi.tf",
	)
}

// ConfigCDNAutoWithResponseObjectConditions returns a CDN auto service config with a response
// object whose request_condition and cache_condition reference real nested condition blocks.
func ConfigCDNAutoWithResponseObjectConditions(serviceName, domainName, responseObjectName, requestConditionName, cacheConditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"RESPONSE_OBJECT_NAME":   responseObjectName,
			"REQUEST_CONDITION_NAME": requestConditionName,
			"CACHE_CONDITION_NAME":   cacheConditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/response_object_with_conditions.tf",
	)
}

// ConfigACLForImport returns a test configuration for importing an ACL.
func ConfigACLForImport(serviceName, domainName, aclName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"ACL_NAME":        aclName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
	)
}

// ConfigACLAtVersion returns a service/domain/ACL config pinned to the given version,
// for exercising in-place version changes on the explicit fastly_service_cdn_acl resource.
func ConfigACLAtVersion(serviceName, domainName, aclName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": fmt.Sprintf("%d", version),
			"ACL_NAME":        aclName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
	)
}

// Configuration helpers for Compute Auto service

// ConfigComputeAutoBasic returns a basic Compute auto service config with a domain and package.
func ConfigComputeAutoBasic(serviceName, domainName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"PACKAGE_PATH": GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithBackend returns a Compute auto service config with a domain, backend, and package.
func ConfigComputeAutoWithBackend(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"BACKEND_NAME": backendName,
			"PACKAGE_PATH": GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithHealthCheck returns a Compute auto service config with a domain, package, and a health check.
func ConfigComputeAutoWithHealthCheck(serviceName, domainName, healthCheckName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"HEALTHCHECK_NAME": healthCheckName,
			"PACKAGE_PATH":     GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/healthcheck_single.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithDictionary returns a Compute auto service config with a domain, package, and a dictionary.
func ConfigComputeAutoWithDictionary(serviceName, domainName, dictionaryName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"DICTIONARY_NAME": dictionaryName,
			"PACKAGE_PATH":    GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/dictionary_single.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithDictionaryForceDestroy returns a Compute auto service config with a
// domain, package, and a dictionary that has force_destroy enabled.
func ConfigComputeAutoWithDictionaryForceDestroy(serviceName, domainName, dictionaryName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"DICTIONARY_NAME": dictionaryName,
			"PACKAGE_PATH":    GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/dictionary_with_force_destroy.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithKVStoreResourceLink returns a Compute auto service config with a
// domain, package, and a resource_link block pointing at a Terraform-managed fastly_kvstore
// (declared as a sibling resource, referenced by ID rather than a literal string).
func ConfigComputeAutoWithKVStoreResourceLink(serviceName, domainName, storeName, linkName string) string {
	kvStoreConfig := RenderBlock("internal/acceptance_tests/blocks/kvstore_single.tf", map[string]string{
		"KVSTORE_NAME": storeName,
	})

	return kvStoreConfig + "\n" + BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"PACKAGE_PATH":            GetPackagePath(),
			"RESOURCE_LINK_NAME":      linkName,
			"RESOURCE_LINK_TARGET_ID": "fastly_kvstore.store.id",
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/resource_link_ref.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithStandaloneKVStore returns a Compute auto service config (domain and
// package, no resource_link) alongside a separately declared, unlinked fastly_kvstore.
//
// The Fastly API doesn't allow deleting a KV Store in the same request that unlinks it from a
// service, so tests that remove a resource_link and then delete its target KV Store need this as
// an intermediate step: unlink first and let that settle, then delete the KV Store in a later step.
func ConfigComputeAutoWithStandaloneKVStore(serviceName, domainName, storeName string) string {
	kvStoreConfig := RenderBlock("internal/acceptance_tests/blocks/kvstore_single.tf", map[string]string{
		"KVSTORE_NAME": storeName,
	})

	return kvStoreConfig + "\n" + ConfigComputeAutoBasic(serviceName, domainName)
}

// ConfigComputeAutoWithKVStoreResourceLinkTarget returns a Compute auto service config
// declaring two fastly_kvstore resources (kv1 and kv2), with the resource_link pointing at
// whichever is named by targetLabel. Both KV Stores stay declared regardless of which is
// targeted, so retargeting exercises the reconcile delete-old/create-new pass without deleting
// either KV Store.
func ConfigComputeAutoWithKVStoreResourceLinkTarget(serviceName, domainName, storeName1, storeName2, linkName, targetLabel string) string {
	kvStoreConfig := RenderBlock("internal/acceptance_tests/blocks/kvstore_two.tf", map[string]string{
		"KVSTORE_NAME_1": storeName1,
		"KVSTORE_NAME_2": storeName2,
	})

	return kvStoreConfig + "\n" + BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"PACKAGE_PATH":            GetPackagePath(),
			"RESOURCE_LINK_NAME":      linkName,
			"RESOURCE_LINK_TARGET_ID": fmt.Sprintf("fastly_kvstore.%s.id", targetLabel),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/resource_link_ref.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigKVStore returns a minimal standalone fastly_kvstore config.
func ConfigKVStore(name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/kvstore_single.tf", map[string]string{
		"KVSTORE_NAME": name,
	})
}

// ConfigKVStoreWithLocation returns a standalone fastly_kvstore config with an explicit
// location, for exercising the location attribute's plan-time validation and its
// replace-on-change behavior.
func ConfigKVStoreWithLocation(name, location string) string {
	return RenderBlock("internal/acceptance_tests/blocks/kvstore_with_location.tf", map[string]string{
		"KVSTORE_NAME":     name,
		"KVSTORE_LOCATION": location,
	})
}

// ConfigKVStoreForceDestroy returns a standalone fastly_kvstore config with force_destroy set,
// for exercising deletion of a KV Store that still contains entries.
func ConfigKVStoreForceDestroy(name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/kvstore_force_destroy.tf", map[string]string{
		"KVSTORE_NAME": name,
	})
}

// ConfigKVStoresDataSource returns a config declaring three fastly_kvstore resources alongside
// a fastly_kvstores data source that depends on all three.
func ConfigKVStoresDataSource(h string) string {
	return RenderBlock("internal/acceptance_tests/blocks/kvstore_three_with_datasource.tf", map[string]string{
		"KVSTORE_NAME_1": fmt.Sprintf("tf_%s_1", h),
		"KVSTORE_NAME_2": fmt.Sprintf("tf_%s_2", h),
		"KVSTORE_NAME_3": fmt.Sprintf("tf_%s_3", h),
	})
}

// ConfigNGWAFWorkspacesDataSource returns a config declaring three fastly_ngwaf_workspace
// resources alongside a fastly_ngwaf_workspaces data source that depends on all three.
func ConfigNGWAFWorkspacesDataSource(h string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_workspace_three_with_datasource.tf", map[string]string{
		"WORKSPACE_NAME_1": fmt.Sprintf("tf_%s_1", h),
		"WORKSPACE_NAME_2": fmt.Sprintf("tf_%s_2", h),
		"WORKSPACE_NAME_3": fmt.Sprintf("tf_%s_3", h),
	})
}

// ConfigNGWAFWorkspaceSignal returns a config declaring a workspace-scoped NGWAF signal.
func ConfigNGWAFWorkspaceSignal(workspaceName, signalName, signalDescription string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_workspace_signal_basic.tf", map[string]string{
		"WORKSPACE_NAME":     workspaceName,
		"SIGNAL_NAME":        signalName,
		"SIGNAL_DESCRIPTION": signalDescription,
	})
}

// ConfigNGWAFWorkspaceSignalUpdated returns a config updating the description of a
// workspace-scoped NGWAF signal. Signal names are immutable and require replacement.
func ConfigNGWAFWorkspaceSignalUpdated(workspaceName, signalName, updatedDescription string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_workspace_signal_updated.tf", map[string]string{
		"WORKSPACE_NAME":             workspaceName,
		"SIGNAL_NAME":                signalName,
		"SIGNAL_DESCRIPTION_UPDATED": updatedDescription,
	})
}

// ConfigNGWAFWorkspaceSignalsDataSource returns a config declaring two workspace-scoped
// NGWAF signals alongside a fastly_ngwaf_workspace_signals data source.
func ConfigNGWAFWorkspaceSignalsDataSource(workspaceName, signalName1, signalName2 string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_workspace_signals_with_datasource.tf", map[string]string{
		"WORKSPACE_NAME": workspaceName,
		"SIGNAL_NAME_1":  signalName1,
		"SIGNAL_NAME_2":  signalName2,
	})
}

// ConfigNGWAFWorkspaceRedaction returns a config declaring a workspace-scoped NGWAF
// field redaction.
func ConfigNGWAFWorkspaceRedaction(workspaceName, redactionField string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_workspace_redaction_basic.tf", map[string]string{
		"WORKSPACE_NAME":  workspaceName,
		"REDACTION_FIELD": redactionField,
	})
}

// ConfigNGWAFWorkspaceRedactionUpdated returns a config updating the field of a
// workspace-scoped NGWAF field redaction.
func ConfigNGWAFWorkspaceRedactionUpdated(workspaceName, updatedRedactionField string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_workspace_redaction_updated.tf", map[string]string{
		"WORKSPACE_NAME":          workspaceName,
		"REDACTION_FIELD_UPDATED": updatedRedactionField,
	})
}

// ConfigNGWAFWorkspaceRedactionsDataSource returns a config declaring two workspace-scoped
// NGWAF field redactions alongside a fastly_ngwaf_workspace_redactions data source.
func ConfigNGWAFWorkspaceRedactionsDataSource(workspaceName, redactionField1, redactionField2 string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_workspace_redactions_with_datasource.tf", map[string]string{
		"WORKSPACE_NAME":    workspaceName,
		"REDACTION_FIELD_1": redactionField1,
		"REDACTION_FIELD_2": redactionField2,
	})
}

// ConfigComputeAutoWithACLResourceLink returns a Compute auto service config with a
// domain, package, and a resource_link block pointing at a Terraform-managed fastly_acl
// (declared as a sibling resource, referenced by ID rather than a literal string).
func ConfigComputeAutoWithACLResourceLink(serviceName, domainName, aclName, linkName string) string {
	aclConfig := RenderBlock("internal/acceptance_tests/blocks/acl_resource_single.tf", map[string]string{
		"ACL_NAME": aclName,
	})

	return aclConfig + "\n" + BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"PACKAGE_PATH":            GetPackagePath(),
			"RESOURCE_LINK_NAME":      linkName,
			"RESOURCE_LINK_TARGET_ID": "fastly_acl.acl.id",
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/resource_link_ref.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithStandaloneACL returns a Compute auto service config (domain and
// package, no resource_link) alongside a separately declared, unlinked fastly_acl.
//
// The Fastly API doesn't allow deleting an ACL in the same request that unlinks it from a
// service, so tests that remove a resource_link and then delete its target ACL need this as an
// intermediate step: unlink first and let that settle, then delete the ACL in a later step.
func ConfigComputeAutoWithStandaloneACL(serviceName, domainName, aclName string) string {
	aclConfig := RenderBlock("internal/acceptance_tests/blocks/acl_resource_single.tf", map[string]string{
		"ACL_NAME": aclName,
	})

	return aclConfig + "\n" + ConfigComputeAutoBasic(serviceName, domainName)
}

// ConfigComputeAutoWithACLResourceLinkTarget returns a Compute auto service config
// declaring two fastly_acl resources (acl1 and acl2), with the resource_link pointing at
// whichever is named by targetLabel. Both ACLs stay declared regardless of which is targeted, so
// retargeting exercises the reconcile delete-old/create-new pass without deleting either ACL.
func ConfigComputeAutoWithACLResourceLinkTarget(serviceName, domainName, aclName1, aclName2, linkName, targetLabel string) string {
	aclConfig := RenderBlock("internal/acceptance_tests/blocks/acl_resource_two.tf", map[string]string{
		"ACL_NAME_1": aclName1,
		"ACL_NAME_2": aclName2,
	})

	return aclConfig + "\n" + BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"PACKAGE_PATH":            GetPackagePath(),
			"RESOURCE_LINK_NAME":      linkName,
			"RESOURCE_LINK_TARGET_ID": fmt.Sprintf("fastly_acl.%s.id", targetLabel),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/resource_link_ref.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoMultipleBackends returns a Compute auto service config with multiple backends and a package.
func ConfigComputeAutoMultipleBackends(serviceName, domainName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"PACKAGE_PATH": GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_multiple.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoUnsortedBackendAndDomainBlocks returns a Compute auto service config
// with backend and domain blocks declared in non-sorted order.
func ConfigComputeAutoUnsortedBackendAndDomainBlocks(serviceName, domainBName, domainAName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":  serviceName,
			"DOMAIN_B_NAME": domainBName,
			"DOMAIN_A_NAME": domainAName,
			"PACKAGE_PATH":  GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_multiple_unsorted.tf",
		"internal/acceptance_tests/blocks/backend_multiple_unsorted.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoReversedBackendAndDomainBlocks returns a Compute auto service config
// with the same backend and domain blocks as ConfigComputeAutoUnsortedBackendAndDomainBlocks,
// but declared in the reverse order.
func ConfigComputeAutoReversedBackendAndDomainBlocks(serviceName, domainBName, domainAName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":  serviceName,
			"DOMAIN_B_NAME": domainBName,
			"DOMAIN_A_NAME": domainAName,
			"PACKAGE_PATH":  GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_multiple_reversed.tf",
		"internal/acceptance_tests/blocks/backend_multiple_reversed.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// Configuration helpers for CDN service (explicit version management)

// ConfigServiceCDNBasic returns a basic CDN service config without any nested resources.
func ConfigServiceCDNBasic(serviceName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
		},
	)
}

// ConfigServiceCDNWithComment returns a CDN service config with a comment.
func ConfigServiceCDNWithComment(serviceName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "Managed by Terraform",
		},
	)
}

// ConfigServiceCDNWithDomain returns a CDN service config with a domain resource.
func ConfigServiceCDNWithDomain(serviceName, domainName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": fmt.Sprintf("%d", version),
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
	)
}

// ConfigServiceCDNWithBackend returns a CDN service config with a domain and backend resource.
func ConfigServiceCDNWithBackend(serviceName, domainName, backendName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"BACKEND_NAME":    backendName,
			"SERVICE_VERSION": fmt.Sprintf("%d", version),
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
	)
}

// ConfigServiceCDNWithVersionClone returns a CDN service config with a version clone action.
func ConfigServiceCDNWithVersionClone(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/action_version_clone.tf",
	)
}

// ConfigServiceCDNWithVersionActivate returns a CDN service config with a version activate action.
func ConfigServiceCDNWithVersionActivate(serviceName, domainName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": fmt.Sprintf("%d", version),
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/action_version_activate.tf",
	)
}

// ConfigServiceCDNWithCloneAndActivate returns a CDN service config with both clone and activate actions.
func ConfigServiceCDNWithCloneAndActivate(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"BACKEND_NAME":    backendName,
			"SERVICE_VERSION": "1",
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/action_version_clone.tf",
		"internal/acceptance_tests/blocks/action_version_activate.tf",
	)
}

// Configuration helpers for Compute service (explicit version management)

// ConfigServiceComputeBasic returns a basic Compute service config without any nested resources.
func ConfigServiceComputeBasic(serviceName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
		},
	)
}

// ConfigServiceComputeWithComment returns a Compute service config with a comment.
func ConfigServiceComputeWithComment(serviceName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "Managed by Terraform",
		},
	)
}

// ConfigServiceComputeWithACLResourceLink returns an explicit Compute service config
// with a fastly_acl linked into it via fastly_service_resource_link.
func ConfigServiceComputeWithACLResourceLink(serviceName, aclName, linkName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "",
			"ACL_NAME":           aclName,
			"RESOURCE_LINK_NAME": linkName,
			"SERVICE_VERSION":    "1",
		},
		"internal/acceptance_tests/blocks/resource_link_acl.tf",
	)
}

// Configuration helpers for backend resources (explicit version management)

// ConfigBackendBasic returns a basic backend resource config.
func ConfigBackendBasic(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"BACKEND_NAME":    backendName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/backend_basic.tf",
	)
}

// ConfigBackendUpdated returns a backend resource config with updated values.
func ConfigBackendUpdated(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"BACKEND_NAME":    backendName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/backend_updated.tf",
	)
}

// ConfigBackendFull returns a backend resource config with all optional fields.
func ConfigBackendFull(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"BACKEND_NAME":    backendName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/backend_full.tf",
	)
}

func ConfigBackendFullUpdated(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"BACKEND_NAME":    backendName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/backend_full_updated.tf",
	)
}

// ConfigBackendMultiple returns a config with multiple backend resources.
func ConfigBackendMultiple(serviceName, domainName, backend1Name, backend2Name string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"BACKEND_1_NAME":  backend1Name,
			"BACKEND_2_NAME":  backend2Name,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/backend_multi.tf",
	)
}

// ConfigBackendForImport returns a test configuration for importing a backend.
func ConfigBackendForImport(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"BACKEND_NAME":    backendName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/backend_basic.tf",
	)
}

// ConfigBackendWithRequestCondition returns an explicit backend resource config whose
// request_condition references a real explicit fastly_service_condition resource.
func ConfigBackendWithRequestCondition(serviceName, domainName, backendName, conditionName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"BACKEND_NAME":    backendName,
			"CONDITION_NAME":  conditionName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/backend_explicit_with_request_condition.tf",
	)
}

// Configuration helpers for condition resources (explicit version management)

// ConfigConditionBasic returns a basic condition resource config.
func ConfigConditionBasic(serviceName, domainName, conditionName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"CONDITION_NAME":  conditionName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/condition_explicit.tf",
	)
}

// ConfigConditionUpdated returns a condition resource config with an updated statement and priority.
func ConfigConditionUpdated(serviceName, domainName, conditionName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"CONDITION_NAME":  conditionName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/condition_explicit_updated.tf",
	)
}

// ConfigConditionTypeChanged returns a condition resource config with the same name but a
// different type, exercising the delete+recreate path the Fastly API requires since it
// doesn't support updating a condition's type in place.
func ConfigConditionTypeChanged(serviceName, domainName, conditionName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"CONDITION_NAME":  conditionName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/condition_explicit_type_changed.tf",
	)
}

// ConfigConditionMultiple returns a config with two condition resources.
func ConfigConditionMultiple(serviceName, domainName, conditionName1, conditionName2 string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"DOMAIN_NAME":      domainName,
			"SERVICE_VERSION":  "1",
			"CONDITION_NAME_1": conditionName1,
			"CONDITION_NAME_2": conditionName2,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/condition_explicit_multi.tf",
	)
}

// ConfigConditionHeredoc returns a condition resource config whose statement is defined via a
// HEREDOC, which typically leaves a trailing newline in the configured value.
func ConfigConditionHeredoc(serviceName, domainName, conditionName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"CONDITION_NAME":  conditionName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/condition_explicit_heredoc.tf",
	)
}

// ConfigConditionForImport returns a test configuration for importing a condition.
func ConfigConditionForImport(serviceName, domainName, conditionName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"CONDITION_NAME":  conditionName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/condition_explicit.tf",
	)
}

// Configuration helpers for fastly_service_domain resources (explicit version management)

// ConfigServiceDomainBasic returns a basic domain resource config.
func ConfigServiceDomainBasic(serviceName, domainName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"SERVICE_VERSION": "1",
			"DOMAIN_NAME":     domainName,
		},
		"internal/acceptance_tests/blocks/service_domain_basic.tf",
	)
}

// ConfigServiceDomainWithComment returns a domain resource config with a comment.
func ConfigServiceDomainWithComment(serviceName, domainName, comment string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"SERVICE_VERSION": "1",
			"DOMAIN_NAME":     domainName,
			"DOMAIN_COMMENT":  comment,
		},
		"internal/acceptance_tests/blocks/service_domain_with_comment.tf",
	)
}

// ConfigServiceDomainMultiple returns a config with multiple domain resources.
func ConfigServiceDomainMultiple(serviceName, domain1Name, domain2Name string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"SERVICE_VERSION": "1",
			"DOMAIN_1_NAME":   domain1Name,
			"DOMAIN_2_NAME":   domain2Name,
		},
		"internal/acceptance_tests/blocks/service_domain_multi.tf",
	)
}

// ConfigServiceDomainForImport returns a test configuration for importing a domain.
func ConfigServiceDomainForImport(serviceName, domainName, additionalDomainName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"SERVICE_VERSION": "1",
			"DOMAIN_1_NAME":   domainName,
			"DOMAIN_2_NAME":   additionalDomainName,
		},
		"internal/acceptance_tests/blocks/service_domain_multi.tf",
	)
}

// Configuration helpers for CDN service ACL entries resources

func aclEntriesBase(serviceName, domainName, aclName string) (ServiceType, map[string]string) {
	return ServiceCDN, map[string]string{
		"SERVICE_NAME":    serviceName,
		"SERVICE_COMMENT": "",
		"SERVICE_VERSION": "1",
		"DOMAIN_NAME":     domainName,
		"BACKEND_NAME":    "backend",
		"ACL_NAME":        aclName,
	}
}

func ConfigACLEntriesCreate(serviceName, domainName, aclName string) string {
	svc, replacements := aclEntriesBase(serviceName, domainName, aclName)
	return BuildConfig(svc, replacements,
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
		"internal/acceptance_tests/blocks/acl_entries_single.tf",
	)
}

// ConfigACLEntriesACLOnly declares the ACL container without an
// fastly_service_cdn_acl_entries resource, so tests can seed ACL entries
// outside of Terraform before the entries resource exists.
func ConfigACLEntriesACLOnly(serviceName, domainName, aclName string) string {
	svc, replacements := aclEntriesBase(serviceName, domainName, aclName)
	return BuildConfig(svc, replacements,
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
	)
}

func ConfigACLEntriesUpdate(serviceName, domainName, aclName string) string {
	svc, replacements := aclEntriesBase(serviceName, domainName, aclName)
	return BuildConfig(svc, replacements,
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
		"internal/acceptance_tests/blocks/acl_entries_two.tf",
	)
}

func ConfigACLEntriesDelete(serviceName, domainName, aclName string) string {
	svc, replacements := aclEntriesBase(serviceName, domainName, aclName)
	return BuildConfig(svc, replacements,
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
		"internal/acceptance_tests/blocks/acl_entries_empty.tf",
	)
}

func ConfigACLEntriesMinimalEntry(serviceName, domainName, aclName string) string {
	svc, replacements := aclEntriesBase(serviceName, domainName, aclName)
	return BuildConfig(svc, replacements,
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
		"internal/acceptance_tests/blocks/acl_entries_minimal.tf",
	)
}

func ConfigACLEntriesSameIPDifferentSubnet(serviceName, domainName, aclName string) string {
	svc, replacements := aclEntriesBase(serviceName, domainName, aclName)
	return BuildConfig(svc, replacements,
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
		"internal/acceptance_tests/blocks/acl_entries_same_ip_different_subnet.tf",
	)
}

// ConfigACLEntriesCommentChanged mirrors ConfigACLEntriesCreate's single entry but with
// its comment changed and ip/subnet left untouched, exercising an in-place update of an
// existing entry rather than a replace or a create of an additional entry.
func ConfigACLEntriesCommentChanged(serviceName, domainName, aclName string) string {
	svc, replacements := aclEntriesBase(serviceName, domainName, aclName)
	return BuildConfig(svc, replacements,
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
		"internal/acceptance_tests/blocks/acl_entries_single_comment_changed.tf",
	)
}

func ConfigACLEntriesManyEntries(serviceName, domainName, aclName string, count int) string {
	var entries strings.Builder
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&entries, "\n  entry {\n    ip      = \"%d.0.0.1\"\n    subnet  = 32\n    negated = false\n    comment = \"Entry %d\"\n  }", i, i)
	}
	entries.WriteString("\n")

	svc, replacements := aclEntriesBase(serviceName, domainName, aclName)
	replacements["ENTRIES"] = entries.String()
	return BuildConfig(svc, replacements,
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/service_cdn_backend.tf",
		"internal/acceptance_tests/blocks/acl_explicit.tf",
		"internal/acceptance_tests/blocks/acl_entries_many.tf",
	)
}

// Configuration helpers for the standalone Compute ACL entries resource (fastly_acl_entries)

// ConfigACLEntries returns a config declaring a fastly_acl resource alongside a
// fastly_acl_entries resource (with manage_entries = true) that targets it.
func ConfigACLEntries(aclName string, entries map[string]string) string {
	return RenderBlock("internal/acceptance_tests/blocks/acl_entries_resource.tf", map[string]string{
		"ACL_NAME": aclName,
		"ENTRIES":  entriesHCL(entries),
	})
}

// ConfigACLEntriesUnmanaged mirrors ConfigACLEntries but omits manage_entries,
// leaving it at its default (false).
func ConfigACLEntriesUnmanaged(aclName string, entries map[string]string) string {
	return RenderBlock("internal/acceptance_tests/blocks/acl_entries_resource_unmanaged.tf", map[string]string{
		"ACL_NAME": aclName,
		"ENTRIES":  entriesHCL(entries),
	})
}

func entriesHCL(entries map[string]string) string {
	var hcl strings.Builder
	hcl.WriteString("{\n")
	for prefix, action := range entries {
		fmt.Fprintf(&hcl, "    %q = %q\n", prefix, action)
	}
	hcl.WriteString("  }")
	return hcl.String()
}

// Configuration helpers for S3 logging resources

// ConfigCDNAutoWithLoggingS3 returns a CDN auto service config with a domain and a nested S3 logging block.
func ConfigCDNAutoWithLoggingS3(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_nested.tf",
	)
}

// ConfigCDNAutoWithLoggingS3All returns a CDN auto service config with a nested S3 logging
// block that sets the full set of optional attributes.
func ConfigCDNAutoWithLoggingS3All(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_nested_all.tf",
	)
}

// ConfigCDNAutoWithLoggingS3GzipCodec returns a CDN auto service config with a nested S3 logging
// block that sets compression_codec = "gzip" and leaves gzip_level unset, exercising the auto
// read-back sentinel handling (MatchOrder/preserveGzipSentinelList) that must avoid a perpetual diff.
func ConfigCDNAutoWithLoggingS3GzipCodec(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_nested_gzip_codec.tf",
	)
}

// ConfigCDNAutoWithLoggingS3Updated returns a CDN auto service config with a nested S3 logging block
// whose optional attributes have been changed, exercising the reconcile update path.
func ConfigCDNAutoWithLoggingS3Updated(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_nested_updated.tf",
	)
}

// ConfigCDNAutoWithLoggingS3Condition returns a CDN auto service config with a nested S3
// logging block whose response_condition references a real nested condition block.
func ConfigCDNAutoWithLoggingS3Condition(serviceName, domainName, loggerName, bucketName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
			"CONDITION_NAME":  conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_with_condition.tf",
	)
}

// ConfigCDNAutoWithLoggingS3ConditionRemoved returns a CDN auto service config with the same
// logging_s3 block as ConfigCDNAutoWithLoggingS3Condition, but with the condition block removed
// entirely - response_condition is left unchanged, still naming the now-unmanaged condition.
func ConfigCDNAutoWithLoggingS3ConditionRemoved(serviceName, domainName, loggerName, bucketName, conditionName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
			"CONDITION_NAME":  conditionName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_condition_removed.tf",
	)
}

// ConfigCDNAutoWithMultipleLoggingS3 returns a CDN auto service config with two nested S3 logging blocks.
func ConfigCDNAutoWithMultipleLoggingS3(serviceName, domainName, loggerName1, loggerName2, bucketName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"LOGGING_S3_NAME_1": loggerName1,
			"LOGGING_S3_NAME_2": loggerName2,
			"BUCKET_NAME":       bucketName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_nested_multi.tf",
	)
}

// ConfigCDNAutoWithBackendAndLoggingS3 returns a CDN auto service config with a domain, backend, and nested S3 logging block.
func ConfigCDNAutoWithBackendAndLoggingS3(serviceName, domainName, backendName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"BACKEND_NAME":    backendName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_nested.tf",
	)
}

// ConfigComputeAutoWithLoggingS3 returns a Compute auto service config with a domain, package, and nested S3 logging block.
func ConfigComputeAutoWithLoggingS3(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
			"PACKAGE_PATH":    GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingS3Format returns a Compute auto service config whose
// nested S3 logging block sets format, a VCL-only attribute. service_compute_auto's
// logging_s3 schema (ComputeNestedBlockSchema) omits format/format_version/placement/
// response_condition entirely, so this is expected to fail Terraform's own schema
// validation ("Unsupported argument") rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingS3Format(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"DOMAIN_NAME":     domainName,
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
			"PACKAGE_PATH":    GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_s3_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigCDNAutoWithLoggingBlobStorage returns a CDN auto service config with a nested Blob
// Storage logging block, exercising the reconcile path (clone + activate a new version).
func ConfigCDNAutoWithLoggingBlobStorage(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_nested.tf",
	)
}

// ConfigCDNAutoWithLoggingBlobStorageAll returns a CDN auto service config with a nested
// Blob Storage logging block that sets the full set of optional attributes.
func ConfigCDNAutoWithLoggingBlobStorageAll(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_nested_all.tf",
	)
}

// ConfigCDNAutoWithLoggingBlobStorageGzipCodec returns a CDN auto service config with a
// nested Blob Storage logging block that sets compression_codec = "gzip" and leaves
// gzip_level unset, exercising the auto read-back sentinel handling that must avoid a
// perpetual diff.
func ConfigCDNAutoWithLoggingBlobStorageGzipCodec(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_nested_gzip_codec.tf",
	)
}

// ConfigCDNAutoWithLoggingBlobStorageUpdated returns a CDN auto service config with a nested
// Blob Storage logging block whose optional attributes have been changed, exercising the
// reconcile update path.
func ConfigCDNAutoWithLoggingBlobStorageUpdated(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_nested_updated.tf",
	)
}

// ConfigCDNAutoWithMultipleLoggingBlobStorage returns a CDN auto service config with two
// nested Blob Storage logging blocks.
func ConfigCDNAutoWithMultipleLoggingBlobStorage(serviceName, domainName, loggerName1, loggerName2, containerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":               serviceName,
			"DOMAIN_NAME":                domainName,
			"LOGGING_BLOBSTORAGE_NAME_1": loggerName1,
			"LOGGING_BLOBSTORAGE_NAME_2": loggerName2,
			"CONTAINER_NAME":             containerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_nested_multi.tf",
	)
}

// ConfigCDNAutoWithBackendAndLoggingBlobStorage returns a CDN auto service config with a
// domain, backend, and nested Blob Storage logging block.
func ConfigCDNAutoWithBackendAndLoggingBlobStorage(serviceName, domainName, backendName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"BACKEND_NAME":             backendName,
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_nested.tf",
	)
}

// ConfigComputeAutoWithLoggingBlobStorage returns a Compute auto service config with a
// domain, package, and nested Blob Storage logging block.
func ConfigComputeAutoWithLoggingBlobStorage(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
			"PACKAGE_PATH":             GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingBlobStorageFormat returns a Compute auto service config whose
// nested Blob Storage logging block sets format, a VCL-only attribute.
// service_compute_auto's logging_blobstorage schema (ComputeNestedBlockSchema) omits
// format/format_version/placement/response_condition entirely, so this is expected to fail
// Terraform's own schema validation ("Unsupported argument") rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingBlobStorageFormat(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
			"PACKAGE_PATH":             GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingS3Basic(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_basic.tf",
	)
}

func ConfigLoggingS3AtVersion(serviceName, domainName, loggerName, bucketName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": fmt.Sprintf("%d", version),
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_basic.tf",
	)
}

func ConfigLoggingS3NoAuth(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_no_auth.tf",
	)
}

func ConfigLoggingS3Updated(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_updated.tf",
	)
}

func ConfigLoggingS3IAM(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_iam.tf",
	)
}

func ConfigLoggingS3All(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_all.tf",
	)
}

func ConfigLoggingS3Defaults(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_defaults.tf",
	)
}

func ConfigLoggingS3PlacementNone(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_placement_none.tf",
	)
}

func ConfigLoggingS3CompressionCodec(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_compression_codec.tf",
	)
}

func ConfigLoggingS3GzipCodec(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_gzip_codec.tf",
	)
}

func ConfigLoggingS3CodecConflict(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_codec_conflict.tf",
	)
}

func ConfigLoggingS3GzipLevelInvalid(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_gzip_level_invalid.tf",
	)
}

// ConfigLoggingS3GzipLevelSentinel returns a config that explicitly sets
// gzip_level = -1, the internal "unset" sentinel. This should fail plan-time
// validation via int64validator.Between(0, 9) rather than being silently accepted
// and reinterpreted as "unset" - a user should omit the attribute for that.
func ConfigLoggingS3GzipLevelSentinel(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_gzip_level_sentinel.tf",
	)
}

func ConfigLoggingS3ForImport(serviceName, domainName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"DOMAIN_NAME":     domainName,
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_s3_basic.tf",
	)
}

// ConfigLoggingS3Compute returns a config attaching fastly_service_logging_s3 to
// an explicit Compute service with none of the VCL-only attributes set, which is
// the only shape a Compute service can be configured in.
func ConfigLoggingS3Compute(serviceName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/logging_s3_compute.tf",
	)
}

// ConfigLoggingS3ComputeFormat returns a config attaching fastly_service_logging_s3
// to an explicit Compute service with format set, a VCL-only attribute. Unlike the
// nested blocks, the standalone resource's schema is shared by both service types, so
// this is expected to fail at apply time via ValidateNoVCLOnlyAttributesForCompute
// rather than at Terraform's own schema-validation stage.
func ConfigLoggingS3ComputeFormat(serviceName, loggerName, bucketName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"SERVICE_VERSION": "1",
			"LOGGING_S3_NAME": loggerName,
			"BUCKET_NAME":     bucketName,
		},
		"internal/acceptance_tests/blocks/logging_s3_compute_format.tf",
	)
}

func ConfigLoggingBlobStorageBasic(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_basic.tf",
	)
}

func ConfigLoggingBlobStorageAtVersion(serviceName, domainName, loggerName, containerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          fmt.Sprintf("%d", version),
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_basic.tf",
	)
}

func ConfigLoggingBlobStorageNoAuth(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_no_auth.tf",
	)
}

func ConfigLoggingBlobStorageMissingAuth(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_missing_auth.tf",
	)
}

func ConfigLoggingBlobStorageFileMaxBytesInvalid(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_file_max_bytes_invalid.tf",
	)
}

func ConfigLoggingBlobStorageCompressionCodec(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_compression_codec.tf",
	)
}

func ConfigLoggingBlobStorageGzipCodec(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_gzip_codec.tf",
	)
}

func ConfigLoggingBlobStorageGzipLevelInvalid(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_gzip_level_invalid.tf",
	)
}

// ConfigLoggingBlobStorageGzipLevelSentinel returns a config that explicitly sets
// gzip_level = -1, the internal "unset" sentinel. This should fail plan-time
// validation via int64validator.Between(0, 9) rather than being silently accepted
// and reinterpreted as "unset" - a user should omit the attribute for that.
func ConfigLoggingBlobStorageGzipLevelSentinel(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_gzip_level_sentinel.tf",
	)
}

// pgpPublicKeyFixturePath returns the absolute path to a shared PGP public
// key fixture, used to exercise the public_key attribute across logging
// endpoint acceptance tests without embedding the key block inline in HCL.
func pgpPublicKeyFixturePath() string {
	return filepath.ToSlash(filepath.Join(getRepoRoot(), "internal/acceptance_tests/fixtures/pgp/test_public_key.asc"))
}

func ConfigLoggingBlobStorageAll(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
			"PUBLIC_KEY_PATH":          pgpPublicKeyFixturePath(),
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_all.tf",
	)
}

func ConfigLoggingBlobStorageDefaults(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_defaults.tf",
	)
}

func ConfigLoggingBlobStoragePlacementNone(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_placement_none.tf",
	)
}

func ConfigLoggingBlobStorageCodecConflict(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_codec_conflict.tf",
	)
}

func ConfigLoggingBlobStorageForImport(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_basic.tf",
	)
}

// ConfigLoggingBlobStorageCompute returns a config attaching
// fastly_service_logging_blobstorage to an explicit Compute service with none of the
// VCL-only attributes set, which is the only shape a Compute service can be configured in.
func ConfigLoggingBlobStorageCompute(serviceName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/logging_blobstorage_compute.tf",
	)
}

// ConfigLoggingBlobStorageComputeFormat returns a config attaching
// fastly_service_logging_blobstorage to an explicit Compute service with format set, a
// VCL-only attribute. Unlike the nested blocks, the standalone resource's schema is
// shared by both service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own schema-validation
// stage.
func ConfigLoggingBlobStorageComputeFormat(serviceName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
		},
		"internal/acceptance_tests/blocks/logging_blobstorage_compute_format.tf",
	)
}

func ConfigLoggingBlobStorageUpdated(serviceName, domainName, loggerName, containerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "",
			"DOMAIN_NAME":              domainName,
			"SERVICE_VERSION":          "1",
			"LOGGING_BLOBSTORAGE_NAME": loggerName,
			"CONTAINER_NAME":           containerName,
			"PUBLIC_KEY_PATH":          pgpPublicKeyFixturePath(),
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_blobstorage_updated.tf",
	)
}

func ConfigLoggingNewRelicOTLPBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_basic.tf",
	)
}

func ConfigLoggingNewRelicOTLPUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_updated.tf",
	)
}

func ConfigLoggingNewRelicOTLPAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       fmt.Sprintf("%d", version),
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_basic.tf",
	)
}

func ConfigLoggingNewRelicOTLPForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_basic.tf",
	)
}

// ConfigLoggingNewRelicOTLPCompute returns a config attaching
// fastly_service_logging_newrelicotlp to an explicit Compute service with none of
// the VCL-only attributes set, which is the only shape a Compute service can be
// configured in.
func ConfigLoggingNewRelicOTLPCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_newrelicotlp_compute.tf",
	)
}

// ConfigLoggingNewRelicOTLPComputeFormat returns a config attaching
// fastly_service_logging_newrelicotlp to an explicit Compute service with
// format set, a VCL-only attribute. The standalone resource's schema is shared
// by both service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingNewRelicOTLPComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_newrelicotlp_compute_format.tf",
	)
}

func ConfigCDNAutoWithLoggingNewRelicOTLP(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_nested.tf",
	)
}

func ConfigCDNAutoWithLoggingNewRelicOTLPPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingNewRelicOTLPUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingNewRelicOTLP(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"LOGGING_NEWRELIC_NAME_1": loggerName1,
			"LOGGING_NEWRELIC_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingNewRelicOTLP(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"BACKEND_NAME":          backendName,
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingNewRelicOTLP(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
			"PACKAGE_PATH":          GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingDatadogBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"SERVICE_COMMENT":      "",
			"DOMAIN_NAME":          domainName,
			"SERVICE_VERSION":      "1",
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_datadog_basic.tf",
	)
}

func ConfigLoggingDatadogUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"SERVICE_COMMENT":      "",
			"DOMAIN_NAME":          domainName,
			"SERVICE_VERSION":      "1",
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_datadog_updated.tf",
	)
}

func ConfigLoggingDatadogAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"SERVICE_COMMENT":      "",
			"DOMAIN_NAME":          domainName,
			"SERVICE_VERSION":      fmt.Sprintf("%d", version),
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_datadog_basic.tf",
	)
}

func ConfigLoggingDatadogForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"SERVICE_COMMENT":      "",
			"DOMAIN_NAME":          domainName,
			"SERVICE_VERSION":      "1",
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_datadog_basic.tf",
	)
}

// ConfigLoggingDatadogComputeFormat returns a config attaching
// fastly_service_logging_datadog to an explicit Compute service with format set,
// a VCL-only attribute. The standalone resource's schema is shared by both
// service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingDatadogComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"SERVICE_COMMENT":      "",
			"SERVICE_VERSION":      "1",
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_datadog_compute_format.tf",
	)
}

// ConfigLoggingDatadogCompute returns a config attaching
// fastly_service_logging_datadog to an explicit Compute service with no VCL-only
// attributes set. ClearVCLOnlyCreateFields strips format from the create
// request, so the endpoint ends up with whatever format the Fastly API defaults
// to - see TestAccFastlyServiceLoggingDatadog_formatDefault.
func ConfigLoggingDatadogCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"SERVICE_COMMENT":      "",
			"SERVICE_VERSION":      "1",
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_datadog_compute.tf",
	)
}

func ConfigCDNAutoWithLoggingDatadog(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_datadog_nested.tf",
	)
}

func ConfigCDNAutoWithLoggingDatadogPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_datadog_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingDatadogUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_datadog_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingDatadog(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"LOGGING_DATADOG_NAME_1": loggerName1,
			"LOGGING_DATADOG_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_datadog_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingDatadog(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"BACKEND_NAME":         backendName,
			"LOGGING_DATADOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_datadog_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingDatadog(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"LOGGING_DATADOG_NAME": loggerName,
			"PACKAGE_PATH":         GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_datadog_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingDatadogFormat returns a Compute auto service
// config whose nested logging_datadog block sets format, a VCL-only attribute.
// service_compute_auto's logging_datadog schema (ComputeNestedBlockSchema) omits
// format/format_version/placement/response_condition entirely, so this is
// expected to fail Terraform's own schema validation ("Unsupported argument")
// rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingDatadogFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"LOGGING_DATADOG_NAME": loggerName,
			"PACKAGE_PATH":         GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_datadog_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingNewRelicBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_basic.tf",
	)
}

func ConfigLoggingNewRelicUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_updated.tf",
	)
}

func ConfigLoggingNewRelicAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       fmt.Sprintf("%d", version),
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_basic.tf",
	)
}

func ConfigLoggingNewRelicForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_basic.tf",
	)
}

// ConfigLoggingNewRelicComputeFormat returns a config attaching
// fastly_service_logging_newrelic to an explicit Compute service with format
// set, a VCL-only attribute. The standalone resource's schema is shared by both
// service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingNewRelicComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_newrelic_compute_format.tf",
	)
}

// ConfigLoggingNewRelicCompute returns a config attaching
// fastly_service_logging_newrelic to an explicit Compute service with no
// VCL-only attributes set. ClearVCLOnlyCreateFields strips format from the
// create request, so the endpoint ends up with whatever format the Fastly API
// defaults to - see TestAccFastlyServiceLoggingNewRelic_formatDefault.
func ConfigLoggingNewRelicCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"SERVICE_VERSION":       "1",
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_newrelic_compute.tf",
	)
}

func ConfigCDNAutoWithLoggingNewRelic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_nested.tf",
	)
}

func ConfigCDNAutoWithLoggingNewRelicPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingNewRelicUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingNewRelic(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"LOGGING_NEWRELIC_NAME_1": loggerName1,
			"LOGGING_NEWRELIC_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingNewRelic(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"BACKEND_NAME":          backendName,
			"LOGGING_NEWRELIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingNewRelic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
			"PACKAGE_PATH":          GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingNewRelicFormat returns a Compute auto service
// config whose nested logging_newrelic block sets format, a VCL-only
// attribute. service_compute_auto's logging_newrelic schema
// (ComputeNestedBlockSchema) omits format/format_version/placement/
// response_condition entirely, so this is expected to fail Terraform's own
// schema validation ("Unsupported argument") rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingNewRelicFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
			"PACKAGE_PATH":          GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelic_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingBigQueryBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_basic.tf",
	)
}

func ConfigLoggingBigQueryUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_updated.tf",
	)
}

func ConfigLoggingBigQueryAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       fmt.Sprintf("%d", version),
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_basic.tf",
	)
}

func ConfigLoggingBigQueryForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_basic.tf",
	)
}

// ConfigLoggingBigQueryComputeFormat returns a config attaching
// fastly_service_logging_bigquery to an explicit Compute service with format
// set, a VCL-only attribute. The standalone resource's schema is shared by
// both service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingBigQueryComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"SERVICE_VERSION":       "1",
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_bigquery_compute_format.tf",
	)
}

// ConfigLoggingBigQueryCompute returns a config attaching
// fastly_service_logging_bigquery to an explicit Compute service with no
// VCL-only attributes set. ClearVCLOnlyCreateFields strips format from the
// create request, so the endpoint ends up with whatever format the Fastly API
// defaults to - see TestAccFastlyServiceLoggingBigQuery_formatDefault.
func ConfigLoggingBigQueryCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"SERVICE_VERSION":       "1",
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_bigquery_compute.tf",
	)
}

func ConfigLoggingBigQueryNoAuth(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_no_auth.tf",
	)
}

// ConfigLoggingBigQueryAccountName returns a config authenticating with
// authentication.account_name rather than email/secret_key. Paired with
// ConfigLoggingBigQueryBasic in
// TestAccFastlyServiceLoggingBigQuery_accountNameToEmailSecretKey to exercise
// clearing account_name on update.
func ConfigLoggingBigQueryAccountName(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"SERVICE_COMMENT":       "",
			"DOMAIN_NAME":           domainName,
			"SERVICE_VERSION":       "1",
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_account_name.tf",
	)
}

func ConfigCDNAutoWithLoggingBigQuery(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_nested.tf",
	)
}

// ConfigCDNAutoWithLoggingBigQueryAccountName is ConfigCDNAutoWithLoggingBigQuery
// authenticating with authentication.account_name rather than
// email/secret_key. Paired with ConfigCDNAutoWithLoggingBigQuery in
// TestAccFastlyServiceCDNAuto_loggingBigQueryAccountNameToEmailSecretKey to
// exercise clearing account_name through the nested-block reconcile path.
func ConfigCDNAutoWithLoggingBigQueryAccountName(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_nested_account_name.tf",
	)
}

func ConfigCDNAutoWithLoggingBigQueryPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingBigQueryUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingBigQuery(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"LOGGING_BIGQUERY_NAME_1": loggerName1,
			"LOGGING_BIGQUERY_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingBigQuery(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"BACKEND_NAME":          backendName,
			"LOGGING_BIGQUERY_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingBigQuery(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_BIGQUERY_NAME": loggerName,
			"PACKAGE_PATH":          GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingBigQueryFormat returns a Compute auto service
// config whose nested logging_bigquery block sets format, a VCL-only
// attribute. service_compute_auto's logging_bigquery schema
// (ComputeNestedBlockSchema) omits format/format_version/placement/
// response_condition entirely, so this is expected to fail Terraform's own
// schema validation ("Unsupported argument") rather than reach the Fastly
// API.
func ConfigComputeAutoWithLoggingBigQueryFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_BIGQUERY_NAME": loggerName,
			"PACKAGE_PATH":          GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_bigquery_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingGCSBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"DOMAIN_NAME":      domainName,
			"SERVICE_VERSION":  "1",
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_gcs_basic.tf",
	)
}

func ConfigLoggingGCSUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"DOMAIN_NAME":      domainName,
			"SERVICE_VERSION":  "1",
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_gcs_updated.tf",
	)
}

func ConfigLoggingGCSAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"DOMAIN_NAME":      domainName,
			"SERVICE_VERSION":  fmt.Sprintf("%d", version),
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_gcs_basic.tf",
	)
}

func ConfigLoggingGCSForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"DOMAIN_NAME":      domainName,
			"SERVICE_VERSION":  "1",
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_gcs_basic.tf",
	)
}

// ConfigLoggingGCSComputeFormat returns a config attaching
// fastly_service_logging_gcs to an explicit Compute service with format set,
// a VCL-only attribute. The standalone resource's schema is shared by both
// service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingGCSComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"SERVICE_VERSION":  "1",
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_gcs_compute_format.tf",
	)
}

// ConfigLoggingGCSCompute returns a config attaching fastly_service_logging_gcs
// to an explicit Compute service with no VCL-only attributes set.
// ClearVCLOnlyCreateFields strips format from the create request, so the
// endpoint ends up with whatever format the Fastly API defaults to - see
// TestAccFastlyServiceLoggingGCS_formatDefault.
func ConfigLoggingGCSCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"SERVICE_VERSION":  "1",
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_gcs_compute.tf",
	)
}

func ConfigLoggingGCSNoAuth(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"DOMAIN_NAME":      domainName,
			"SERVICE_VERSION":  "1",
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_gcs_no_auth.tf",
	)
}

// ConfigLoggingGCSAccountName returns a config authenticating with
// authentication.account_name rather than email/secret_key. Paired with
// ConfigLoggingGCSBasic in
// TestAccFastlyServiceLoggingGCS_accountNameToEmailSecretKey to exercise
// clearing account_name on update.
func ConfigLoggingGCSAccountName(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"SERVICE_COMMENT":  "",
			"DOMAIN_NAME":      domainName,
			"SERVICE_VERSION":  "1",
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_gcs_account_name.tf",
	)
}

func ConfigCDNAutoWithLoggingGCS(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_gcs_nested.tf",
	)
}

// ConfigCDNAutoWithLoggingGCSAccountName is ConfigCDNAutoWithLoggingGCS
// authenticating with authentication.account_name rather than
// email/secret_key. Paired with ConfigCDNAutoWithLoggingGCS in
// TestAccFastlyServiceCDNAuto_loggingGCSAccountNameToEmailSecretKey to
// exercise clearing account_name through the nested-block reconcile path.
func ConfigCDNAutoWithLoggingGCSAccountName(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_gcs_nested_account_name.tf",
	)
}

func ConfigCDNAutoWithLoggingGCSPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_gcs_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingGCSUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_gcs_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingGCS(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"LOGGING_GCS_NAME_1": loggerName1,
			"LOGGING_GCS_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_gcs_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingGCS(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"BACKEND_NAME":     backendName,
			"LOGGING_GCS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_gcs_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingGCS(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"LOGGING_GCS_NAME": loggerName,
			"PACKAGE_PATH":     GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_gcs_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingGCSFormat returns a Compute auto service config
// whose nested logging_gcs block sets format, a VCL-only attribute.
// service_compute_auto's logging_gcs schema (ComputeNestedBlockSchema) omits
// format/format_version/placement/response_condition entirely, so this is
// expected to fail Terraform's own schema validation ("Unsupported argument")
// rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingGCSFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":     serviceName,
			"DOMAIN_NAME":      domainName,
			"LOGGING_GCS_NAME": loggerName,
			"PACKAGE_PATH":     GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_gcs_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingSplunkBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"DOMAIN_NAME":         domainName,
			"SERVICE_VERSION":     "1",
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_splunk_basic.tf",
	)
}

func ConfigLoggingSplunkUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"DOMAIN_NAME":         domainName,
			"SERVICE_VERSION":     "1",
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_splunk_updated.tf",
	)
}

func ConfigLoggingSplunkAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"DOMAIN_NAME":         domainName,
			"SERVICE_VERSION":     fmt.Sprintf("%d", version),
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_splunk_basic.tf",
	)
}

func ConfigLoggingSplunkForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"DOMAIN_NAME":         domainName,
			"SERVICE_VERSION":     "1",
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_splunk_basic.tf",
	)
}

// ConfigLoggingSplunkComputeFormat returns a config attaching
// fastly_service_logging_splunk to an explicit Compute service with format set,
// a VCL-only attribute. The standalone resource's schema is shared by both
// service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingSplunkComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"SERVICE_VERSION":     "1",
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_splunk_compute_format.tf",
	)
}

// ConfigLoggingSplunkCompute returns a config attaching
// fastly_service_logging_splunk to an explicit Compute service with no VCL-only
// attributes set. ClearVCLOnlyCreateFields strips format from the create
// request, so the endpoint ends up with whatever format the Fastly API defaults
// to - see TestAccFastlyServiceLoggingSplunk_formatDefault.
func ConfigLoggingSplunkCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"SERVICE_VERSION":     "1",
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_splunk_compute.tf",
	)
}

func ConfigCDNAutoWithLoggingSplunk(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_splunk_nested.tf",
	)
}

func ConfigCDNAutoWithLoggingSplunkPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_splunk_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingSplunkUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_splunk_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingSplunk(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_SPLUNK_NAME_1": loggerName1,
			"LOGGING_SPLUNK_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_splunk_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingSplunk(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"BACKEND_NAME":        backendName,
			"LOGGING_SPLUNK_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_splunk_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingSplunk(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SPLUNK_NAME": loggerName,
			"PACKAGE_PATH":        GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_splunk_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingSplunkFormat returns a Compute auto service
// config whose nested logging_splunk block sets format, a VCL-only attribute.
// service_compute_auto's logging_splunk schema (ComputeNestedBlockSchema) omits
// format/format_version/placement/response_condition entirely, so this is
// expected to fail Terraform's own schema validation ("Unsupported argument")
// rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingSplunkFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SPLUNK_NAME": loggerName,
			"PACKAGE_PATH":        GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_splunk_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingHTTPSBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "",
			"DOMAIN_NAME":        domainName,
			"SERVICE_VERSION":    "1",
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_https_basic.tf",
	)
}

func ConfigLoggingHTTPSUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "",
			"DOMAIN_NAME":        domainName,
			"SERVICE_VERSION":    "1",
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_https_updated.tf",
	)
}

func ConfigLoggingHTTPSAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "",
			"DOMAIN_NAME":        domainName,
			"SERVICE_VERSION":    fmt.Sprintf("%d", version),
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_https_basic.tf",
	)
}

func ConfigLoggingHTTPSForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "",
			"DOMAIN_NAME":        domainName,
			"SERVICE_VERSION":    "1",
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_https_basic.tf",
	)
}

// ConfigLoggingHTTPSGzipCodecConflict returns a config setting both gzip_level
// and compression_codec, which the schema's gzipLevelCodecConflict validator
// rejects at plan time.
func ConfigLoggingHTTPSGzipCodecConflict(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "",
			"DOMAIN_NAME":        domainName,
			"SERVICE_VERSION":    "1",
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_https_gzip_codec.tf",
	)
}

// ConfigLoggingHTTPSComputeFormat returns a config attaching
// fastly_service_logging_https to an explicit Compute service with format set,
// a VCL-only attribute. The standalone resource's schema is shared by both
// service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingHTTPSComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "",
			"SERVICE_VERSION":    "1",
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_https_compute_format.tf",
	)
}

// ConfigLoggingHTTPSCompute returns a config attaching
// fastly_service_logging_https to an explicit Compute service with no VCL-only
// attributes set. ClearVCLOnlyCreateFields strips format from the create
// request, so the endpoint ends up with whatever format the Fastly API defaults
// to - see TestAccFastlyServiceLoggingHTTPS_formatDefault.
func ConfigLoggingHTTPSCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "",
			"SERVICE_VERSION":    "1",
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_https_compute.tf",
	)
}

func ConfigCDNAutoWithLoggingHTTPS(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_https_nested.tf",
	)
}

func ConfigCDNAutoWithLoggingHTTPSPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_https_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingHTTPSUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_https_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingHTTPS(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":         serviceName,
			"DOMAIN_NAME":          domainName,
			"LOGGING_HTTPS_NAME_1": loggerName1,
			"LOGGING_HTTPS_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_https_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingHTTPS(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"BACKEND_NAME":       backendName,
			"LOGGING_HTTPS_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_https_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingHTTPS(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"LOGGING_HTTPS_NAME": loggerName,
			"PACKAGE_PATH":       GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_https_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingSumologicBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"SERVICE_COMMENT":        "",
			"DOMAIN_NAME":            domainName,
			"SERVICE_VERSION":        "1",
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_basic.tf",
	)
}

func ConfigLoggingSumologicUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"SERVICE_COMMENT":        "",
			"DOMAIN_NAME":            domainName,
			"SERVICE_VERSION":        "1",
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_updated.tf",
	)
}

func ConfigLoggingSumologicAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"SERVICE_COMMENT":        "",
			"DOMAIN_NAME":            domainName,
			"SERVICE_VERSION":        fmt.Sprintf("%d", version),
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_basic.tf",
	)
}

func ConfigLoggingSumologicForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"SERVICE_COMMENT":        "",
			"DOMAIN_NAME":            domainName,
			"SERVICE_VERSION":        "1",
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_basic.tf",
	)
}

// ConfigLoggingSumologicComputeFormat returns a config attaching
// fastly_service_logging_sumologic to an explicit Compute service with format
// set, a VCL-only attribute. The standalone resource's schema is shared by both
// service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingSumologicComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"SERVICE_COMMENT":        "",
			"SERVICE_VERSION":        "1",
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_sumologic_compute_format.tf",
	)
}

// ConfigLoggingSumologicCompute returns a config attaching
// fastly_service_logging_sumologic to an explicit Compute service with no
// VCL-only attributes set. ClearVCLOnlyCreateFields strips format from the
// create request, so the endpoint ends up with whatever format the Fastly API
// defaults to - see TestAccFastlyServiceLoggingSumologic_formatDefault.
func ConfigLoggingSumologicCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"SERVICE_COMMENT":        "",
			"SERVICE_VERSION":        "1",
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_sumologic_compute.tf",
	)
}

func ConfigCDNAutoWithLoggingSumologic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_nested.tf",
	)
}

func ConfigCDNAutoWithLoggingSumologicPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingSumologicUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingSumologic(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"LOGGING_SUMOLOGIC_NAME_1": loggerName1,
			"LOGGING_SUMOLOGIC_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingSumologic(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"BACKEND_NAME":           backendName,
			"LOGGING_SUMOLOGIC_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingSumologic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"LOGGING_SUMOLOGIC_NAME": loggerName,
			"PACKAGE_PATH":           GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingHTTPSFormat returns a Compute auto service
// config whose nested logging_https block sets format, a VCL-only attribute.
// service_compute_auto's logging_https schema (ComputeNestedBlockSchema) omits
// format/format_version/placement/response_condition entirely, so this is
// expected to fail Terraform's own schema validation ("Unsupported argument")
// rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingHTTPSFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"LOGGING_HTTPS_NAME": loggerName,
			"PACKAGE_PATH":       GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_https_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingSumologicFormat returns a Compute auto service
// config whose nested logging_sumologic block sets format, a VCL-only
// attribute. service_compute_auto's logging_sumologic schema
// (ComputeNestedBlockSchema) omits format/format_version/placement/response_condition
// entirely, so this is expected to fail Terraform's own schema validation
// ("Unsupported argument") rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingSumologicFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"LOGGING_SUMOLOGIC_NAME": loggerName,
			"PACKAGE_PATH":           GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_sumologic_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

func ConfigLoggingSyslogBasic(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"DOMAIN_NAME":         domainName,
			"SERVICE_VERSION":     "1",
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_syslog_basic.tf",
	)
}

func ConfigLoggingSyslogUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"DOMAIN_NAME":         domainName,
			"SERVICE_VERSION":     "1",
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_syslog_updated.tf",
	)
}

func ConfigLoggingSyslogAtVersion(serviceName, domainName, loggerName string, version int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"DOMAIN_NAME":         domainName,
			"SERVICE_VERSION":     fmt.Sprintf("%d", version),
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_syslog_basic.tf",
	)
}

func ConfigLoggingSyslogForImport(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"DOMAIN_NAME":         domainName,
			"SERVICE_VERSION":     "1",
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/service_cdn_domain.tf",
		"internal/acceptance_tests/blocks/logging_syslog_basic.tf",
	)
}

// ConfigLoggingSyslogComputeFormat returns a config attaching
// fastly_service_logging_syslog to an explicit Compute service with format
// set, a VCL-only attribute. The standalone resource's schema is shared by both
// service types, so this is expected to fail at apply time via
// ValidateNoVCLOnlyAttributesForCompute rather than at Terraform's own
// schema-validation stage.
func ConfigLoggingSyslogComputeFormat(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"SERVICE_VERSION":     "1",
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_syslog_compute_format.tf",
	)
}

// ConfigLoggingSyslogCompute returns a config attaching
// fastly_service_logging_syslog to an explicit Compute service with no
// VCL-only attributes set. ClearVCLOnlyCreateFields strips format from the
// create request, so the endpoint ends up with whatever format the Fastly API
// defaults to - see TestAccFastlyServiceLoggingSyslog_formatDefault.
func ConfigLoggingSyslogCompute(serviceName, loggerName string) string {
	return BuildConfig(
		ServiceCompute,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"SERVICE_COMMENT":     "",
			"SERVICE_VERSION":     "1",
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/logging_syslog_compute.tf",
	)
}

func ConfigCDNAutoWithLoggingSyslog(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_syslog_nested.tf",
	)
}

func ConfigCDNAutoWithLoggingSyslogPlacementNone(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_syslog_nested_placement_none.tf",
	)
}

func ConfigCDNAutoWithLoggingSyslogUpdated(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_syslog_nested_updated.tf",
	)
}

func ConfigCDNAutoWithMultipleLoggingSyslog(serviceName, domainName, loggerName1, loggerName2 string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_SYSLOG_NAME_1": loggerName1,
			"LOGGING_SYSLOG_NAME_2": loggerName2,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_syslog_nested_multi.tf",
	)
}

func ConfigCDNAutoWithBackendAndLoggingSyslog(serviceName, domainName, backendName, loggerName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"BACKEND_NAME":        backendName,
			"LOGGING_SYSLOG_NAME": loggerName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/logging_syslog_nested.tf",
	)
}

func ConfigComputeAutoWithLoggingSyslog(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SYSLOG_NAME": loggerName,
			"PACKAGE_PATH":        GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_syslog_nested.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingSyslogFormat returns a Compute auto service
// config whose nested logging_syslog block sets format, a VCL-only
// attribute. service_compute_auto's logging_syslog schema
// (ComputeNestedBlockSchema) omits format/format_version/placement/response_condition
// entirely, so this is expected to fail Terraform's own schema validation
// ("Unsupported argument") rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingSyslogFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"LOGGING_SYSLOG_NAME": loggerName,
			"PACKAGE_PATH":        GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_syslog_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// productEnablementBlock renders a single "internal/acceptance_tests/blocks/service_product_<product>.tf"
// template, merging SERVICE_ID_REF (the Terraform expression for the owning
// service's id, e.g. "fastly_service_cdn_auto.test.id") with any
// product-specific values (e.g. CONTENT_GUARD, DDOS_MODE).
func productEnablementBlock(product, serviceIDRef string, extra map[string]string) string {
	data := map[string]string{"SERVICE_ID_REF": serviceIDRef}
	maps.Copy(data, extra)
	return RenderBlock(fmt.Sprintf("internal/acceptance_tests/blocks/service_product_%s.tf", product), data)
}

// ConfigServiceVCLWithFile returns a CDN service with one explicit custom VCL resource
// whose content is loaded through Terraform's file() function.
func ConfigServiceVCLWithFile(serviceName, vclName, vclFilePath string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "VCL acceptance test",
			"VCL_NAME":        vclName,
			"VCL_FILE_PATH":   filepath.ToSlash(vclFilePath),
		},
		"internal/acceptance_tests/blocks/vcl_explicit.tf",
	)
}

// ConfigServiceVCLInline returns an explicit custom VCL resource whose content
// is defined inline in HCL rather than loaded through Terraform's file() function.
func ConfigServiceVCLInline(serviceName, vclName, content string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"SERVICE_COMMENT":    "VCL acceptance test",
			"VCL_NAME":           vclName,
			"VCL_INLINE_CONTENT": strconv.Quote(content),
		},
		"internal/acceptance_tests/blocks/vcl_explicit_inline.tf",
	)
}

// ConfigCDNAutoWithVCLFile returns a CDN auto service with domain, backend, and one
// nested custom VCL block whose content is loaded through Terraform's file() function.
func ConfigCDNAutoWithVCLFile(serviceName, domainName, backendName, vclName, vclFilePath string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":  serviceName,
			"DOMAIN_NAME":   domainName,
			"BACKEND_NAME":  backendName,
			"VCL_NAME":      vclName,
			"VCL_FILE_PATH": filepath.ToSlash(vclFilePath),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/vcl_nested_single.tf",
	)
}

// ConfigCDNAutoWithVCLInline returns a CDN auto service with domain, backend,
// and one nested custom VCL block whose content is defined inline in HCL.
func ConfigCDNAutoWithVCLInline(serviceName, domainName, backendName, vclName, content string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":       serviceName,
			"DOMAIN_NAME":        domainName,
			"BACKEND_NAME":       backendName,
			"VCL_NAME":           vclName,
			"VCL_INLINE_CONTENT": strconv.Quote(content),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/vcl_nested_inline.tf",
	)
}

// ConfigCDNAutoWithVCLHeredoc returns a CDN auto service with domain, backend,
// and one nested custom VCL block whose content is defined with a Terraform HEREDOC.
func ConfigCDNAutoWithVCLHeredoc(serviceName, domainName, backendName, vclName, content string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":        serviceName,
			"DOMAIN_NAME":         domainName,
			"BACKEND_NAME":        backendName,
			"VCL_NAME":            vclName,
			"VCL_HEREDOC_CONTENT": strings.TrimSuffix(content, "\n"),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/vcl_nested_heredoc.tf",
	)
}

// ConfigCDNAutoWithMultipleVCLFiles returns a CDN auto service with a main VCL file
// and an included library VCL file.
func ConfigCDNAutoWithMultipleVCLFiles(serviceName, domainName, backendName, mainName, includeName, mainFilePath, includeFilePath string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"BACKEND_NAME":          backendName,
			"VCL_MAIN_NAME":         mainName,
			"VCL_INCLUDE_NAME":      includeName,
			"VCL_MAIN_FILE_PATH":    filepath.ToSlash(mainFilePath),
			"VCL_INCLUDE_FILE_PATH": filepath.ToSlash(includeFilePath),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/vcl_nested_multiple.tf",
	)
}

// ConfigCDNAutoWithInvalidMultipleMainVCLFiles returns a CDN auto service with two
// nested custom VCL files marked as main, exercising provider-side validation.
func ConfigCDNAutoWithInvalidMultipleMainVCLFiles(serviceName, domainName, backendName, mainFilePath, secondMainFilePath string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":              serviceName,
			"DOMAIN_NAME":               domainName,
			"BACKEND_NAME":              backendName,
			"VCL_MAIN_FILE_PATH":        filepath.ToSlash(mainFilePath),
			"VCL_SECOND_MAIN_FILE_PATH": filepath.ToSlash(secondMainFilePath),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/vcl_nested_two_main.tf",
	)
}

// ConfigCDNAutoWithInvalidNoMainVCLFile returns a CDN auto service with one
// nested custom VCL file but no main VCL, exercising provider-side validation.
func ConfigCDNAutoWithInvalidNoMainVCLFile(serviceName, domainName, backendName, includeFilePath string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"BACKEND_NAME":          backendName,
			"VCL_INCLUDE_FILE_PATH": filepath.ToSlash(includeFilePath),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/vcl_nested_no_main.tf",
	)
}

// joinBlocks concatenates rendered blocks, each already ending in its own
// newline, separated by a blank line.
func joinBlocks(blocks ...string) string {
	return strings.Join(blocks, "\n")
}

// ConfigProductEnablementCDNEmpty returns a CDN auto service (with a
// shield-equipped backend, required by image_optimizer) and no
// product-enablement resources at all, for use as the "nothing enabled"
// starting point of a lifecycle test.
func ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"BACKEND_NAME": backendName,
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_with_shield.tf",
	)
}

// ConfigProductEnablementCDNBasic returns the same CDN auto service as
// ConfigProductEnablementCDNEmpty, plus one resource per CDN-applicable
// product (every product except the Compute-only fanout).
func ConfigProductEnablementCDNBasic(serviceName, domainName, backendName, ngwafWorkspaceID string) string {
	const serviceIDRef = "fastly_service_cdn_auto.test.id"
	products := joinBlocks(
		productEnablementBlock("brotli_compression", serviceIDRef, nil),
		productEnablementBlock("image_optimizer", serviceIDRef, nil),
		productEnablementBlock("origin_inspector", serviceIDRef, nil),
		productEnablementBlock("domain_inspector", serviceIDRef, nil),
		productEnablementBlock("websockets", serviceIDRef, nil),
		productEnablementBlock("log_explorer_insights", serviceIDRef, nil),
		productEnablementBlock("api_discovery", serviceIDRef, nil),
		productEnablementBlock("bot_management", serviceIDRef, map[string]string{"CONTENT_GUARD": "on"}),
		productEnablementBlock("ddos_protection", serviceIDRef, map[string]string{"DDOS_MODE": "block"}),
		productEnablementBlock("ngwaf", serviceIDRef, map[string]string{
			"NGWAF_WORKSPACE_ID": ngwafWorkspaceID,
			"NGWAF_TRAFFIC_RAMP": "50",
		}),
	)
	return ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName) + "\n" + products
}

// ConfigProductEnablementComputeEmpty returns a Compute auto service and no
// product-enablement resources at all.
func ConfigProductEnablementComputeEmpty(serviceName, domainName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME": serviceName,
			"DOMAIN_NAME":  domainName,
			"PACKAGE_PATH": GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigComputeAutoWithLoggingNewRelicOTLPFormat returns a Compute auto service
// config whose nested logging_newrelicotlp block sets format, a VCL-only
// attribute. service_compute_auto's logging_newrelicotlp schema
// (ComputeNestedBlockSchema) omits format/format_version/placement/
// response_condition entirely, so this is expected to fail Terraform's own
// schema validation ("Unsupported argument") rather than reach the Fastly API.
func ConfigComputeAutoWithLoggingNewRelicOTLPFormat(serviceName, domainName, loggerName string) string {
	return BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"LOGGING_NEWRELIC_NAME": loggerName,
			"PACKAGE_PATH":          GetPackagePath(),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/logging_newrelicotlp_nested_compute_format.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigProductEnablementComputeBasic returns the same Compute auto service
// as ConfigProductEnablementComputeEmpty, plus one resource per
// Compute-applicable product (every product except the CDN-only
// brotli_compression and image_optimizer).
func ConfigProductEnablementComputeBasic(serviceName, domainName, ngwafWorkspaceID string) string {
	const serviceIDRef = "fastly_service_compute_auto.test.id"
	products := joinBlocks(
		productEnablementBlock("fanout", serviceIDRef, nil),
		productEnablementBlock("origin_inspector", serviceIDRef, nil),
		productEnablementBlock("domain_inspector", serviceIDRef, nil),
		productEnablementBlock("websockets", serviceIDRef, nil),
		productEnablementBlock("log_explorer_insights", serviceIDRef, nil),
		productEnablementBlock("api_discovery", serviceIDRef, nil),
		productEnablementBlock("bot_management", serviceIDRef, map[string]string{"CONTENT_GUARD": "on"}),
		productEnablementBlock("ddos_protection", serviceIDRef, map[string]string{"DDOS_MODE": "block"}),
		productEnablementBlock("ngwaf", serviceIDRef, map[string]string{"NGWAF_WORKSPACE_ID": ngwafWorkspaceID}),
	)
	return ConfigProductEnablementComputeEmpty(serviceName, domainName) + "\n" + products
}

// ConfigProductEnablementInvalidFanoutOnCDN returns a CDN auto service
// paired with fastly_service_product_fanout, a Compute-only resource, to
// exercise runtime service-type validation.
func ConfigProductEnablementInvalidFanoutOnCDN(serviceName, domainName string) string {
	return ConfigCDNAutoBasic(serviceName, domainName) + "\n" +
		productEnablementBlock("fanout", "fastly_service_cdn_auto.test.id", nil)
}

// ConfigProductEnablementInvalidFanoutOnCDNExistingService returns the same
// CDN auto service as ConfigCDNAutoBasic, plus
// fastly_service_product_fanout on it. Meant to be used as the second
// step after ConfigCDNAutoBasic has already applied, so service_id is
// already known when this config is planned - exercising ModifyPlan's
// plan-time rejection path, rather than the Create-time fallback exercised
// when the service and product-enablement resource are created together in
// a single apply.
func ConfigProductEnablementInvalidFanoutOnCDNExistingService(serviceName, domainName string) string {
	return ConfigCDNAutoBasic(serviceName, domainName) + "\n" +
		productEnablementBlock("fanout", "fastly_service_cdn_auto.test.id", nil)
}

// ConfigProductEnablementInvalidBrotliCompressionOnCompute returns a
// Compute auto service paired with fastly_service_product_brotli_compression,
// a CDN-only resource, to exercise runtime service-type validation - the
// mirror image of ConfigProductEnablementInvalidFanoutOnCDN.
func ConfigProductEnablementInvalidBrotliCompressionOnCompute(serviceName, domainName string) string {
	return ConfigProductEnablementComputeEmpty(serviceName, domainName) + "\n" +
		productEnablementBlock("brotli_compression", "fastly_service_compute_auto.test.id", nil)
}

// ConfigProductEnablementInvalidImageOptimizerOnCompute returns a Compute
// auto service paired with fastly_service_product_image_optimizer, a
// CDN-only resource, to exercise runtime service-type validation.
// Confirmed against the live Fastly API: the product_enablement API
// rejects image_optimizer for wasm services outright ("image_optimizer not
// available for wasm services"), independent of any account entitlement to
// the Image Optimizer-on-Compute Beta described elsewhere in Fastly's docs
// - that Beta is accessed through the Compute SDK's own request API, not
// this enablement endpoint.
func ConfigProductEnablementInvalidImageOptimizerOnCompute(serviceName, domainName string) string {
	return ConfigProductEnablementComputeEmpty(serviceName, domainName) + "\n" +
		productEnablementBlock("image_optimizer", "fastly_service_compute_auto.test.id", nil)
}

// ConfigProductEnablementInvalidNGWAFTrafficRampOnCompute returns a Compute
// auto service paired with fastly_service_product_ngwaf whose
// traffic_ramp is set to a non-default value, a CDN-only setting, to
// exercise runtime service-type validation.
func ConfigProductEnablementInvalidNGWAFTrafficRampOnCompute(serviceName, domainName, ngwafWorkspaceID string) string {
	return ConfigProductEnablementComputeEmpty(serviceName, domainName) + "\n" +
		productEnablementBlock("ngwaf", "fastly_service_compute_auto.test.id", map[string]string{
			"NGWAF_WORKSPACE_ID": ngwafWorkspaceID,
			"NGWAF_TRAFFIC_RAMP": "50",
		})
}

// ConfigProductEnablementInvalidContentGuard returns a CDN auto service
// paired with fastly_service_product_bot_management whose contentguard
// is set to a value outside "off"/"on", to exercise the attribute's
// stringvalidator.OneOf schema validation.
func ConfigProductEnablementInvalidContentGuard(serviceName, domainName string) string {
	return ConfigCDNAutoBasic(serviceName, domainName) + "\n" +
		productEnablementBlock("bot_management", "fastly_service_cdn_auto.test.id", map[string]string{"CONTENT_GUARD": "nonsense"})
}

// ConfigProductEnablementInvalidDDoSMode returns a CDN auto service paired
// with fastly_service_product_ddos_protection whose mode is set to a
// value outside "off"/"log"/"block", to exercise the attribute's
// stringvalidator.OneOf schema validation.
func ConfigProductEnablementInvalidDDoSMode(serviceName, domainName string) string {
	return ConfigCDNAutoBasic(serviceName, domainName) + "\n" +
		productEnablementBlock("ddos_protection", "fastly_service_cdn_auto.test.id", map[string]string{"DDOS_MODE": "nonsense"})
}

// ConfigProductEnablementInvalidNGWAFTrafficRampRange returns a CDN auto
// service paired with fastly_service_product_ngwaf whose traffic_ramp is
// set outside the 0-100 range, to exercise the attribute's
// int64validator.Between schema validation.
func ConfigProductEnablementInvalidNGWAFTrafficRampRange(serviceName, domainName, ngwafWorkspaceID string) string {
	return ConfigCDNAutoBasic(serviceName, domainName) + "\n" +
		productEnablementBlock("ngwaf", "fastly_service_cdn_auto.test.id", map[string]string{
			"NGWAF_WORKSPACE_ID": ngwafWorkspaceID,
			"NGWAF_TRAFFIC_RAMP": "150",
		})
}

// ConfigProductEnablementDDoSModeOnly returns a CDN auto service paired
// with just fastly_service_product_ddos_protection at the given mode,
// used to verify that changing mode updates in place.
func ConfigProductEnablementDDoSModeOnly(serviceName, domainName, backendName, mode string) string {
	return ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName) + "\n" +
		productEnablementBlock("ddos_protection", "fastly_service_cdn_auto.test.id", map[string]string{"DDOS_MODE": mode})
}

// ConfigProductEnablementBotManagementOnly returns a CDN auto service
// paired with just fastly_service_product_bot_management at the given
// contentguard value, used to verify that changing contentguard updates
// in place.
func ConfigProductEnablementBotManagementOnly(serviceName, domainName, backendName, contentGuard string) string {
	return ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName) + "\n" +
		productEnablementBlock("bot_management", "fastly_service_cdn_auto.test.id", map[string]string{"CONTENT_GUARD": contentGuard})
}

// ConfigProductEnablementNGWAFOnly returns a CDN auto service paired with
// just fastly_service_product_ngwaf at the given workspace_id and
// traffic_ramp, used to verify that changing either updates in place.
func ConfigProductEnablementNGWAFOnly(serviceName, domainName, backendName, workspaceID string, trafficRamp int) string {
	return ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName) + "\n" +
		productEnablementBlock("ngwaf", "fastly_service_cdn_auto.test.id", map[string]string{
			"NGWAF_WORKSPACE_ID": workspaceID,
			"NGWAF_TRAFFIC_RAMP": strconv.Itoa(trafficRamp),
		})
}

// ConfigProductEnablementDDoSEnabledToggle returns a CDN auto service paired
// with fastly_service_product_ddos_protection at the given mode, explicitly
// setting the enabled attribute, used to verify that toggling enabled
// between true and false re-enables the product (via the Enable API) rather
// than leaving it disabled after a false -> true transition.
func ConfigProductEnablementDDoSEnabledToggle(serviceName, domainName, backendName, mode string, enabled bool) string {
	return ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName) + "\n" +
		productEnablementBlock("ddos_protection", "fastly_service_cdn_auto.test.id", map[string]string{
			"DDOS_MODE": mode,
			"ENABLED":   strconv.FormatBool(enabled),
		})
}

// ConfigProductEnablementBotManagementEnabledToggle returns a CDN auto
// service paired with fastly_service_product_bot_management at the given
// contentguard value, explicitly setting the enabled attribute, used to
// verify that toggling enabled between true and false re-enables the
// product (via the Enable API) rather than leaving it disabled after a
// false -> true transition.
func ConfigProductEnablementBotManagementEnabledToggle(serviceName, domainName, backendName, contentGuard string, enabled bool) string {
	return ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName) + "\n" +
		productEnablementBlock("bot_management", "fastly_service_cdn_auto.test.id", map[string]string{
			"CONTENT_GUARD": contentGuard,
			"ENABLED":       strconv.FormatBool(enabled),
		})
}

// ConfigProductEnablementNGWAFEnabledToggle returns a CDN auto service
// paired with fastly_service_product_ngwaf at the given workspace_id,
// explicitly setting the enabled attribute, used to verify that toggling
// enabled between true and false re-enables the product (via the Enable
// API) rather than leaving it disabled after a false -> true transition.
func ConfigProductEnablementNGWAFEnabledToggle(serviceName, domainName, backendName, workspaceID string, enabled bool) string {
	return ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName) + "\n" +
		productEnablementBlock("ngwaf", "fastly_service_cdn_auto.test.id", map[string]string{
			"NGWAF_WORKSPACE_ID": workspaceID,
			"ENABLED":            strconv.FormatBool(enabled),
		})
}

// ConfigProductEnablementSimpleEnabledToggle returns a CDN auto service paired
// with a simple product (origin_inspector) that explicitly sets the enabled
// attribute to the given value, used to verify that toggling enabled between
// true and false works correctly.
func ConfigProductEnablementSimpleEnabledToggle(serviceName, domainName, backendName string, enabled bool) string {
	enabledStr := "true"
	if !enabled {
		enabledStr = "false"
	}
	return ConfigProductEnablementCDNEmpty(serviceName, domainName, backendName) + "\n" +
		productEnablementBlock("origin_inspector", "fastly_service_cdn_auto.test.id", map[string]string{"ENABLED": enabledStr})
}

// ConfigProductEnablementServiceIDReplace returns two CDN auto services and
// a single fastly_service_product_domain_inspector resource whose
// service_id points at either "first" or "second" depending on useSecond,
// to exercise the service_id RequiresReplace plan modifier.
func ConfigProductEnablementServiceIDReplace(serviceName1, domainName1, serviceName2, domainName2 string, useSecond bool) string {
	target := "fastly_service_cdn_auto.first.id"
	if useSecond {
		target = "fastly_service_cdn_auto.second.id"
	}

	first := RenderBlock("internal/acceptance_tests/blocks/service_cdn_auto_named.tf", map[string]string{
		"LABEL":        "first",
		"SERVICE_NAME": serviceName1,
		"DOMAIN_NAME":  domainName1,
	})
	second := RenderBlock("internal/acceptance_tests/blocks/service_cdn_auto_named.tf", map[string]string{
		"LABEL":        "second",
		"SERVICE_NAME": serviceName2,
		"DOMAIN_NAME":  domainName2,
	})

	return joinBlocks(first, second, productEnablementBlock("domain_inspector", target, nil))
}

// ConfigServiceVCLSnippetWithFile returns a CDN service with one explicit regular VCL snippet
// whose content is loaded through Terraform's file() function.
func ConfigServiceVCLSnippetWithFile(serviceName, snippetName, snippetType string, priority int, snippetFilePath string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"SERVICE_COMMENT":   "VCL snippet acceptance test",
			"SNIPPET_NAME":      snippetName,
			"SNIPPET_TYPE":      snippetType,
			"SNIPPET_PRIORITY":  strconv.Itoa(priority),
			"SNIPPET_FILE_PATH": filepath.ToSlash(snippetFilePath),
		},
		"internal/acceptance_tests/blocks/snippet_explicit.tf",
	)
}

// ConfigServiceVCLSnippetInline returns an explicit regular VCL snippet whose content
// is defined inline in HCL.
func ConfigServiceVCLSnippetInline(serviceName, snippetName, snippetType string, priority int, content string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"SERVICE_COMMENT":        "VCL snippet acceptance test",
			"SNIPPET_NAME":           snippetName,
			"SNIPPET_TYPE":           snippetType,
			"SNIPPET_PRIORITY":       strconv.Itoa(priority),
			"SNIPPET_INLINE_CONTENT": strconv.Quote(content),
		},
		"internal/acceptance_tests/blocks/snippet_explicit_inline.tf",
	)
}

// ConfigCDNAutoWithSnippetFile returns a CDN auto service with domain, backend, and one
// regular VCL snippet whose content is loaded through Terraform's file() function.
func ConfigCDNAutoWithSnippetFile(serviceName, domainName, backendName, snippetName, snippetType string, priority int, snippetFilePath string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"BACKEND_NAME":      backendName,
			"SNIPPET_NAME":      snippetName,
			"SNIPPET_TYPE":      snippetType,
			"SNIPPET_PRIORITY":  strconv.Itoa(priority),
			"SNIPPET_FILE_PATH": filepath.ToSlash(snippetFilePath),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/snippet_nested_single.tf",
	)
}

// ConfigCDNAutoWithSnippetInline returns a CDN auto service with domain, backend, and one
// regular VCL snippet whose content is defined inline in HCL.
func ConfigCDNAutoWithSnippetInline(serviceName, domainName, backendName, snippetName, snippetType string, priority int, content string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":           serviceName,
			"DOMAIN_NAME":            domainName,
			"BACKEND_NAME":           backendName,
			"SNIPPET_NAME":           snippetName,
			"SNIPPET_TYPE":           snippetType,
			"SNIPPET_PRIORITY":       strconv.Itoa(priority),
			"SNIPPET_INLINE_CONTENT": strconv.Quote(content),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/snippet_nested_inline.tf",
	)
}

// ConfigCDNAutoWithMultipleSnippets returns a CDN auto service with two regular VCL snippets.
func ConfigCDNAutoWithMultipleSnippets(serviceName, domainName, backendName, snippetNameOne, snippetNameTwo, snippetFilePathOne, snippetFilePathTwo string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"BACKEND_NAME":          backendName,
			"SNIPPET_NAME_ONE":      snippetNameOne,
			"SNIPPET_NAME_TWO":      snippetNameTwo,
			"SNIPPET_FILE_PATH_ONE": filepath.ToSlash(snippetFilePathOne),
			"SNIPPET_FILE_PATH_TWO": filepath.ToSlash(snippetFilePathTwo),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/snippet_nested_multiple.tf",
	)
}

// ConfigCDNAutoWithDuplicateSnippets returns a CDN auto service with duplicate regular
// VCL snippet names, exercising provider-side validation.
func ConfigCDNAutoWithDuplicateSnippets(serviceName, domainName, backendName, snippetFilePathOne, snippetFilePathTwo string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":          serviceName,
			"DOMAIN_NAME":           domainName,
			"BACKEND_NAME":          backendName,
			"SNIPPET_FILE_PATH_ONE": filepath.ToSlash(snippetFilePathOne),
			"SNIPPET_FILE_PATH_TWO": filepath.ToSlash(snippetFilePathTwo),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/snippet_nested_duplicate_names.tf",
	)
}

// ConfigCDNAutoWithInvalidSnippetType returns a CDN auto service with an invalid regular
// VCL snippet type, exercising schema validation.
func ConfigCDNAutoWithInvalidSnippetType(serviceName, domainName, backendName, snippetFilePath string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"BACKEND_NAME":      backendName,
			"SNIPPET_FILE_PATH": filepath.ToSlash(snippetFilePath),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/snippet_nested_invalid_type.tf",
	)
}

// ConfigCDNAutoWithDynamicSnippet returns a CDN auto service with domain, backend,
// and one dynamic VCL snippet metadata block.
func ConfigCDNAutoWithDynamicSnippet(serviceName, domainName, backendName, snippetName, snippetType string, priority int) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"BACKEND_NAME":             backendName,
			"DYNAMIC_SNIPPET_NAME":     snippetName,
			"DYNAMIC_SNIPPET_TYPE":     snippetType,
			"DYNAMIC_SNIPPET_PRIORITY": strconv.Itoa(priority),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/dynamic_snippet_nested.tf",
	)
}

// ConfigCDNAutoWithDynamicSnippetContent returns a CDN auto service with one
// dynamic VCL snippet and a separate versionless dynamic snippet content resource.
func ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainName, backendName, snippetName, snippetType string, priority int, content string, manageSnippets bool) string {
	serviceConfig := BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"BACKEND_NAME":             backendName,
			"DYNAMIC_SNIPPET_NAME":     snippetName,
			"DYNAMIC_SNIPPET_TYPE":     snippetType,
			"DYNAMIC_SNIPPET_PRIORITY": strconv.Itoa(priority),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/dynamic_snippet_nested.tf",
	)

	contentResource := renderFixtureBlock("blocks/dynamic_snippet_content.tf", map[string]string{
		"DYNAMIC_SNIPPET_NAME":           snippetName,
		"DYNAMIC_SNIPPET_INLINE_CONTENT": strconv.Quote(content),
		"MANAGE_SNIPPETS":                strconv.FormatBool(manageSnippets),
	})

	return joinBlocks(serviceConfig, contentResource)
}

// ConfigCDNAutoWithRegularAndDynamicSnippetConflict returns a CDN auto service
// with regular and dynamic snippets using the same name.
func ConfigCDNAutoWithRegularAndDynamicSnippetConflict(serviceName, domainName, backendName, snippetName, snippetFilePath string) string {
	return BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":      serviceName,
			"DOMAIN_NAME":       domainName,
			"BACKEND_NAME":      backendName,
			"SNIPPET_NAME":      snippetName,
			"SNIPPET_FILE_PATH": filepath.ToSlash(snippetFilePath),
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/dynamic_snippet_nested_conflict.tf",
	)
}

// ConfigServiceDynamicVCLSnippet returns a CDN service with one explicit/default
// first-class dynamic VCL snippet metadata resource.
func ConfigServiceDynamicVCLSnippet(serviceName, snippetName, snippetType string, priority int) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "Dynamic VCL snippet acceptance test",
			"DYNAMIC_SNIPPET_NAME":     snippetName,
			"DYNAMIC_SNIPPET_TYPE":     snippetType,
			"DYNAMIC_SNIPPET_PRIORITY": strconv.Itoa(priority),
		},
		"internal/acceptance_tests/blocks/dynamic_snippet_explicit.tf",
	)
}

// ConfigServiceDynamicVCLSnippetContent returns a CDN service with one
// explicit/default first-class dynamic VCL snippet metadata resource and a
// separate versionless dynamic snippet content resource.
func ConfigServiceDynamicVCLSnippetContent(serviceName, snippetName, snippetType string, priority int, content string, manageSnippets bool) string {
	serviceConfig := BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"SERVICE_COMMENT":          "Dynamic VCL snippet acceptance test",
			"DYNAMIC_SNIPPET_NAME":     snippetName,
			"DYNAMIC_SNIPPET_TYPE":     snippetType,
			"DYNAMIC_SNIPPET_PRIORITY": strconv.Itoa(priority),
		},
		"internal/acceptance_tests/blocks/dynamic_snippet_explicit.tf",
	)

	contentResource := renderFixtureBlock("blocks/dynamic_snippet_explicit_content.tf", map[string]string{
		"DYNAMIC_SNIPPET_INLINE_CONTENT": strconv.Quote(content),
		"MANAGE_SNIPPETS":                strconv.FormatBool(manageSnippets),
	})

	return joinBlocks(serviceConfig, contentResource)
}

// ConfigDataSourceVCLSnippets returns a CDN auto service with regular and dynamic
// VCL snippets plus a fastly_vcl_snippets data source that reads the active version.
func ConfigDataSourceVCLSnippets(serviceName, domainName, backendName, regularNameOne, regularNameTwo, dynamicName, snippetFilePathOne, snippetFilePathTwo string) string {
	serviceConfig := BuildConfig(
		ServiceCDNAuto,
		map[string]string{
			"SERVICE_NAME":             serviceName,
			"DOMAIN_NAME":              domainName,
			"BACKEND_NAME":             backendName,
			"SNIPPET_NAME_ONE":         regularNameOne,
			"SNIPPET_NAME_TWO":         regularNameTwo,
			"SNIPPET_FILE_PATH_ONE":    filepath.ToSlash(snippetFilePathOne),
			"SNIPPET_FILE_PATH_TWO":    filepath.ToSlash(snippetFilePathTwo),
			"DYNAMIC_SNIPPET_NAME":     dynamicName,
			"DYNAMIC_SNIPPET_TYPE":     "recv",
			"DYNAMIC_SNIPPET_PRIORITY": "25",
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/backend_single.tf",
		"internal/acceptance_tests/blocks/snippet_nested_multiple.tf",
		"internal/acceptance_tests/blocks/dynamic_snippet_nested.tf",
	)

	dataSource := `
data "fastly_vcl_snippets" "example" {
  depends_on      = [fastly_service_cdn_auto.test]
  service_id      = fastly_service_cdn_auto.test.id
  service_version = fastly_service_cdn_auto.test.active_version
}
`

	return joinBlocks(serviceConfig, dataSource)
}

func renderFixtureBlock(path string, values map[string]string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("error reading fixture %s: %s", path, err))
	}

	replacements := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacements = append(replacements, "{{."+key+"}}", value)
	}

	return strings.NewReplacer(replacements...).Replace(string(data))
}

// ConfigConfigStore returns a standalone fastly_configstore configuration.
func ConfigConfigStore(name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/configstore_single.tf", map[string]string{
		"CONFIGSTORE_NAME": name,
	})
}

// ConfigConfigStoreWithComputeAutoResourceLink returns a Config Store plus a Compute auto
// service with a resource_link pointing at that Config Store. The Compute package is included
// so this is a complete runnable Compute service config, not an isolated resource test.
func ConfigConfigStoreWithComputeAutoResourceLink(storeName, serviceName, domainName, linkName string) string {
	return ConfigConfigStore(storeName) + "\n" + BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"PACKAGE_PATH":            GetPackagePath(),
			"RESOURCE_LINK_NAME":      linkName,
			"RESOURCE_LINK_TARGET_ID": "fastly_configstore.store.id",
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/resource_link_ref.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigConfigStoreWithComputeAutoUnlinked returns a Config Store plus the same Compute auto
// service without the resource_link. This is the required intermediate state before deleting a
// store that was linked to a service.
func ConfigConfigStoreWithComputeAutoUnlinked(storeName, serviceName, domainName string) string {
	return ConfigConfigStore(storeName) + "\n" + ConfigComputeAutoBasic(serviceName, domainName)
}

// ConfigConfigStoresDataSource returns one fastly_configstore resource and a
// fastly_configstores data source that depends on it. One known store is sufficient
// to verify enumeration without consuming the account's limited Config Store quota.
func ConfigConfigStoresDataSource(h string) string {
	return RenderBlock("internal/acceptance_tests/blocks/configstore_with_datasource.tf", map[string]string{
		"CONFIGSTORE_NAME": fmt.Sprintf("tf_%s", h),
	})
}

// ConfigSecretStore returns a standalone fastly_secretstore configuration.
func ConfigSecretStore(name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/secretstore_single.tf", map[string]string{
		"SECRETSTORE_NAME": name,
	})
}

// ConfigSecretStoreWithComputeAutoResourceLink returns a Secret Store plus a Compute auto
// service with a resource_link pointing at that Secret Store. The Compute package is included
// so this is a complete runnable Compute service config, not an isolated resource test.
func ConfigSecretStoreWithComputeAutoResourceLink(storeName, serviceName, domainName, linkName string) string {
	return ConfigSecretStore(storeName) + "\n" + BuildConfig(
		ServiceComputeAuto,
		map[string]string{
			"SERVICE_NAME":            serviceName,
			"DOMAIN_NAME":             domainName,
			"PACKAGE_PATH":            GetPackagePath(),
			"RESOURCE_LINK_NAME":      linkName,
			"RESOURCE_LINK_TARGET_ID": "fastly_secretstore.store.id",
		},
		"internal/acceptance_tests/blocks/domain_single.tf",
		"internal/acceptance_tests/blocks/resource_link_ref.tf",
		"internal/acceptance_tests/blocks/package.tf",
	)
}

// ConfigSecretStoreWithComputeAutoUnlinked returns a Secret Store plus the same Compute auto
// service without the resource_link. This is the required intermediate state before deleting a
// store that was linked to a service.
func ConfigSecretStoreWithComputeAutoUnlinked(storeName, serviceName, domainName string) string {
	return ConfigSecretStore(storeName) + "\n" + ConfigComputeAutoBasic(serviceName, domainName)
}

// ConfigSecretStoresDataSource returns one fastly_secretstore resource and a
// fastly_secretstores data source that depends on it. One known store is sufficient
// to verify enumeration without consuming the account's limited Secret Store quota.
func ConfigSecretStoresDataSource(h string) string {
	return RenderBlock("internal/acceptance_tests/blocks/secretstore_with_datasource.tf", map[string]string{
		"SECRETSTORE_NAME": fmt.Sprintf("tf_%s", h),
	})
}

// ConfigNGWAFWorkspaceListsByType returns a config declaring one workspace-scoped
// NGWAF list of every supported type alongside the workspace lists data source.
func ConfigNGWAFWorkspaceListsByType(workspaceName string, names map[string]string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_workspace_lists_by_type.tf", map[string]string{
		"WORKSPACE_NAME":     workspaceName,
		"IP_LIST_NAME":       names["ip"],
		"STRING_LIST_NAME":   names["string"],
		"WILDCARD_LIST_NAME": names["wildcard"],
		"COUNTRY_LIST_NAME":  names["country"],
		"SIGNAL_LIST_NAME":   names["signal"],
	})
}

// ConfigAPISecurityOperation returns a CDN service plus a fastly_api_security_operation
// resource. Passing an empty description omits the attribute from config entirely, to
// exercise the transition back to an unset (rather than empty-string) description.
func ConfigAPISecurityOperation(serviceName, method, domain, path, description string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"METHOD":          method,
			"DOMAIN":          domain,
			"PATH":            path,
			"DESCRIPTION":     description,
		},
		"internal/acceptance_tests/blocks/api_security_operation.tf",
	)
}

// ConfigAPISecurityOperationWithTag returns a CDN service plus a
// fastly_api_security_operation_tag and a fastly_api_security_operation that references
// it via tag_ids. Used to prove tag_ids survives an update that only touches description.
func ConfigAPISecurityOperationWithTag(serviceName, method, domain, path, tagName, description string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"METHOD":          method,
			"DOMAIN":          domain,
			"PATH":            path,
			"TAG_NAME":        tagName,
			"DESCRIPTION":     description,
		},
		"internal/acceptance_tests/blocks/api_security_operation_with_tag.tf",
	)
}

// ConfigAPISecurityOperationTag returns a CDN service plus a
// fastly_api_security_operation_tag resource. Passing an empty description omits the
// attribute from config entirely, to exercise the transition back to an unset (rather
// than empty-string) description.
func ConfigAPISecurityOperationTag(serviceName, tagName, description string) string {
	return BuildConfig(
		ServiceCDN,
		map[string]string{
			"SERVICE_NAME":    serviceName,
			"SERVICE_COMMENT": "",
			"TAG_NAME":        tagName,
			"DESCRIPTION":     description,
		},
		"internal/acceptance_tests/blocks/api_security_operation_tag.tf",
	)
}

// ConfigAlertStatsAccountWide returns a standalone account-wide fastly_alert (source "stats", no service_id).
func ConfigAlertStatsAccountWide(alertName, description, metric, evalType, evalPeriod string, threshold float64) string {
	return RenderBlock("internal/acceptance_tests/blocks/alert_stats_account_wide.tf", map[string]string{
		"ALERT_NAME":        alertName,
		"ALERT_DESCRIPTION": description,
		"METRIC":            metric,
		"EVAL_TYPE":         evalType,
		"EVAL_PERIOD":       evalPeriod,
		"EVAL_THRESHOLD":    strconv.FormatFloat(threshold, 'f', -1, 64),
	})
}

// ConfigAlertDomainsScoped returns a CDN auto service with Domain Inspector enabled plus a
// fastly_alert scoped to that service and restricted to domainName via a dimensions block.
func ConfigAlertDomainsScoped(serviceName, domainName, alertName, description, metric, evalType, evalPeriod string, threshold float64) string {
	service := ConfigCDNAutoBasic(serviceName, domainName)
	domainInspector := productEnablementBlock("domain_inspector", "fastly_service_cdn_auto.test.id", nil)
	alert := RenderBlock("internal/acceptance_tests/blocks/alert_domains_scoped.tf", map[string]string{
		"ALERT_NAME":        alertName,
		"ALERT_DESCRIPTION": description,
		"SERVICE_ID_REF":    "fastly_service_cdn_auto.test.id",
		"DOMAIN_NAME":       domainName,
		"METRIC":            metric,
		"EVAL_TYPE":         evalType,
		"EVAL_PERIOD":       evalPeriod,
		"EVAL_THRESHOLD":    strconv.FormatFloat(threshold, 'f', -1, 64),
	})

	return joinBlocks(service, domainInspector, alert)
}

// ConfigAlertDomainsMissingServiceID returns a "domains" source fastly_alert with no service_id.
func ConfigAlertDomainsMissingServiceID(alertName, metric string) string {
	return RenderBlock("internal/acceptance_tests/blocks/alert_domains_missing_service_id.tf", map[string]string{
		"ALERT_NAME": alertName,
		"METRIC":     metric,
	})
}

// ConfigAlertPercentIncreaseWithIgnoreBelow returns a "stats" fastly_alert using the
// percent_increase evaluation strategy with ignore_below set.
func ConfigAlertPercentIncreaseWithIgnoreBelow(alertName string, threshold, ignoreBelow float64) string {
	return RenderBlock("internal/acceptance_tests/blocks/alert_percent_increase.tf", map[string]string{
		"ALERT_NAME":   alertName,
		"THRESHOLD":    strconv.FormatFloat(threshold, 'f', -1, 64),
		"IGNORE_BELOW": strconv.FormatFloat(ignoreBelow, 'f', -1, 64),
	})
}

// ConfigDNSZone returns a standalone fastly_dns_zone with a name and description.
func ConfigDNSZone(name, description string) string {
	return RenderBlock("internal/acceptance_tests/blocks/dns_zone_basic.tf", map[string]string{
		"ZONE_NAME":        name,
		"ZONE_DESCRIPTION": description,
	})
}

// ConfigDNSZoneMinimal returns a standalone fastly_dns_zone with only name set.
func ConfigDNSZoneMinimal(name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/dns_zone_minimal.tf", map[string]string{
		"ZONE_NAME": name,
	})
}

// ConfigDNSZoneWithXfrConfig returns a fastly_dns_zone with an xfr_config_inbound block
// containing one primary and no inbound_tsig_key_id.
func ConfigDNSZoneWithXfrConfig(name, description, primaryAddress, primaryDescription string) string {
	return RenderBlock("internal/acceptance_tests/blocks/dns_zone_with_xfr_config.tf", map[string]string{
		"ZONE_NAME":           name,
		"ZONE_DESCRIPTION":    description,
		"PRIMARY_ADDRESS":     primaryAddress,
		"PRIMARY_DESCRIPTION": primaryDescription,
	})
}

// ConfigDNSZoneWithTSIGKey returns a fastly_dns_zone with an xfr_config_inbound block
// that references an out-of-band-created TSIG key ID.
func ConfigDNSZoneWithTSIGKey(name, description, tsigKeyID, primaryAddress, primaryDescription string) string {
	return RenderBlock("internal/acceptance_tests/blocks/dns_zone_with_tsig.tf", map[string]string{
		"ZONE_NAME":           name,
		"ZONE_DESCRIPTION":    description,
		"TSIG_KEY_ID":         tsigKeyID,
		"PRIMARY_ADDRESS":     primaryAddress,
		"PRIMARY_DESCRIPTION": primaryDescription,
	})
}

// ConfigDNSZonesDataSource returns a config declaring three fastly_dns_zone resources
// alongside a fastly_dns_zones data source that depends on all three.
func ConfigDNSZonesDataSource(name1, name2, name3 string) string {
	return RenderBlock("internal/acceptance_tests/blocks/dns_zone_three_with_datasource.tf", map[string]string{
		"ZONE_NAME_1": name1,
		"ZONE_NAME_2": name2,
		"ZONE_NAME_3": name3,
	})
}

// ConfigFastlyDomain returns a standalone fastly_domain with an fqdn and description.
func ConfigFastlyDomain(fqdn, description string) string {
	return RenderBlock("internal/acceptance_tests/blocks/fastly_domain_basic.tf", map[string]string{
		"DOMAIN_FQDN":        fqdn,
		"DOMAIN_DESCRIPTION": description,
	})
}

// ConfigFastlyDomainMinimal returns a standalone fastly_domain with only fqdn set.
func ConfigFastlyDomainMinimal(fqdn string) string {
	return RenderBlock("internal/acceptance_tests/blocks/fastly_domain_minimal.tf", map[string]string{
		"DOMAIN_FQDN": fqdn,
	})
}

// ConfigFastlyDomainWithServiceLink returns a CDN service, a fastly_domain, and a link between them.
func ConfigFastlyDomainWithServiceLink(serviceName, fqdn string) string {
	service := ConfigServiceCDNBasic(serviceName)
	link := RenderBlock("internal/acceptance_tests/blocks/fastly_domain_service_link.tf", map[string]string{
		"DOMAIN_FQDN":    fqdn,
		"SERVICE_ID_REF": "fastly_service_cdn.test.id",
	})
	return joinBlocks(service, link)
}

// ConfigFastlyDomainsDataSource returns three fastly_domain resources plus a fastly_domains data source.
func ConfigFastlyDomainsDataSource(fqdn1, fqdn2, fqdn3 string) string {
	return RenderBlock("internal/acceptance_tests/blocks/fastly_domain_three_with_datasource.tf", map[string]string{
		"DOMAIN_FQDN_1": fqdn1,
		"DOMAIN_FQDN_2": fqdn2,
		"DOMAIN_FQDN_3": fqdn3,
	})
}

// integrationAuthKeys are the config keys that hold secret values (API keys, tokens,
// webhook URLs) and so belong under `authentication` rather than `config`.
var integrationAuthKeys = map[string]struct{}{
	"apikey":  {},
	"token":   {},
	"key":     {},
	"webhook": {},
	"url":     {},
}

// splitIntegrationConfig splits a flat integration config map into the non-sensitive
// fields (config) and sensitive fields (authentication), mirroring the resource schema.
func splitIntegrationConfig(config map[string]string) (nonSensitive, sensitive map[string]string) {
	nonSensitive = map[string]string{}
	sensitive = map[string]string{}
	for k, v := range config {
		if _, ok := integrationAuthKeys[k]; ok {
			sensitive[k] = v
			continue
		}
		nonSensitive[k] = v
	}
	return nonSensitive, sensitive
}

// ConfigIntegration returns a standalone fastly_integration with the given name, description,
// type, and config. Sensitive keys (see integrationAuthKeys) are rendered under `authentication`.
func ConfigIntegration(name, description, integrationType string, config map[string]string) string {
	nonSensitive, sensitive := splitIntegrationConfig(config)
	return RenderBlock("internal/acceptance_tests/blocks/integration_basic.tf", map[string]string{
		"NAME":           name,
		"DESCRIPTION":    description,
		"TYPE":           integrationType,
		"CONFIG":         entriesHCL(nonSensitive),
		"AUTHENTICATION": entriesHCL(sensitive),
	})
}

// ConfigIntegrationInvalidType returns a fastly_integration using an unsupported type, for validator-failure testing.
func ConfigIntegrationInvalidType(name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/integration_invalid_type.tf", map[string]string{
		"NAME": name,
	})
}

// ConfigTLSActivation returns a CDN auto service (with a domain and backend) plus a
// fastly_tls_activation enabling TLS on that domain. certificateIDExpr is the raw HCL expression
// assigned to certificate_id — either a quoted literal (e.g. `""` for validator-failure tests) or
// a reference to a fastly_tls_certificate resource declared via ConfigTLSCertificate.
// extraDependsOn, when given, names additional resources (e.g. "fastly_tls_certificate.test") the
// activation's depends_on should wait on, alongside the service.
func ConfigTLSActivation(serviceName, domainName, backendName, certificateIDExpr string, extraDependsOn ...string) string {
	service := ConfigCDNAutoWithBackend(serviceName, domainName, backendName)
	activation := RenderBlock("internal/acceptance_tests/blocks/tls_activation_single.tf", map[string]string{
		"CERTIFICATE_ID_EXPR": certificateIDExpr,
		"DOMAIN_NAME":         domainName,
		"EXTRA_DEPENDS_ON":    extraDependsOnHCL(extraDependsOn),
	})
	return joinBlocks(service, activation)
}

// extraDependsOnHCL renders extraDependsOn as a leading-comma-prefixed, comma-joined HCL fragment
// suitable for splicing after an existing depends_on entry, e.g. ", a, b" for ["a", "b"], or "" if empty.
func extraDependsOnHCL(extraDependsOn []string) string {
	if len(extraDependsOn) == 0 {
		return ""
	}
	return ", " + strings.Join(extraDependsOn, ", ")
}

// ConfigTLSCertificatePair returns a fastly_tls_private_key + fastly_tls_certificate pair (both
// named resourceName) uploading keyPEM/certPEM under name, for use as a real, Terraform-managed
// certificate_id reference from a fastly_tls_activation under test. Unlike ConfigTLSCertificate
// (which always labels its resource "test", for tests exercising a single certificate),
// resourceName lets a test declare more than one pair at once, e.g. for rotation scenarios.
func ConfigTLSCertificatePair(resourceName, name, keyPEM, certPEM string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_key_and_certificate.tf", map[string]string{
		"RESOURCE_NAME": resourceName,
		"NAME":          name,
		"KEY_PEM":       keyPEM,
		"CERT_PEM":      certPEM,
	})
}

// ConfigTLSActivationWithMutualAuthentication is ConfigTLSActivation plus a
// fastly_tls_mutual_authentication resource wired to the activation directly.
func ConfigTLSActivationWithMutualAuthentication(serviceName, domainName, backendName, certificateIDExpr, mtlsCertBundle string, extraDependsOn ...string) string {
	service := ConfigCDNAutoWithBackend(serviceName, domainName, backendName)
	mtls := RenderBlock("internal/acceptance_tests/blocks/tls_mutual_authentication_single.tf", map[string]string{
		"CERT_BUNDLE": mtlsCertBundle,
	})
	activation := fmt.Sprintf(`
resource "fastly_tls_activation" "test" {
  certificate_id            = %s
  domain                    = %q
  mutual_authentication_id  = fastly_tls_mutual_authentication.test.id
  depends_on                = [fastly_service_cdn_auto.test%s]
}
`, certificateIDExpr, domainName, extraDependsOnHCL(extraDependsOn))
	return joinBlocks(service, mtls, activation)
}

// ConfigTLSMutualAuthentication returns a standalone fastly_tls_mutual_authentication.
// enforced/name are omitted from the config when passed as "".
func ConfigTLSMutualAuthentication(certBundle, enforced, name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_mutual_authentication_single.tf", map[string]string{
		"CERT_BUNDLE": certBundle,
		"ENFORCED":    enforced,
		"NAME":        name,
	})
}

// ConfigTLSPrivateKey returns a standalone fastly_tls_private_key with the given name and PEM-encoded key material.
func ConfigTLSPrivateKey(name, keyPEM string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_private_key_single.tf", map[string]string{
		"NAME":    name,
		"KEY_PEM": keyPEM,
	})
}

// ConfigTLSCertificate returns a fastly_tls_certificate uploading certificateBody, with an
// explicit name.
func ConfigTLSCertificate(certificateBody, name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_certificate_single.tf", map[string]string{
		"CERTIFICATE_BODY": certificateBody,
		"NAME":             name,
	})
}

// ConfigTLSCertificateWithoutName is ConfigTLSCertificate but leaves name unset, so it is
// computed by the API from the certificate's Common Name/SAN.
func ConfigTLSCertificateWithoutName(certificateBody string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_certificate_single.tf", map[string]string{
		"CERTIFICATE_BODY": certificateBody,
	})
}

// ConfigTLSCertificateWithPrivateKey is ConfigTLSCertificate plus depends_on =
// [fastly_tls_private_key.test], for tests that declare that resource (via ConfigTLSPrivateKey)
// in the same config: the Fastly API rejects a certificate upload until its matching private key
// already exists, and nothing else ties the two resources together for Terraform to order
// correctly. name may be "" to leave it computed, same as ConfigTLSCertificateWithoutName.
func ConfigTLSCertificateWithPrivateKey(certificateBody, name string) string {
	return RenderBlock("internal/acceptance_tests/blocks/tls_certificate_single.tf", map[string]string{
		"CERTIFICATE_BODY":       certificateBody,
		"NAME":                   name,
		"DEPENDS_ON_PRIVATE_KEY": "true",
	})
}

// ConfigTLSSubscription returns a CDN auto service with two domains plus a fastly_tls_subscription
// requesting a lets-encrypt certificate for both, so that common_name can be switched between them
// to exercise an in-place update.
func ConfigTLSSubscription(serviceName, domain1, domain2, backendName, commonName string) string {
	service := fmt.Sprintf(`
resource "fastly_service_cdn_auto" "test" {
  name          = %q
  force_destroy = true

  domain {
    name = %q
  }

  domain {
    name = %q
  }

  backend {
    name              = %q
    address           = "api.example.com"
    port              = 443
    use_ssl           = true
    ssl_cert_hostname = "api.example.com"
    ssl_sni_hostname  = "api.example.com"
  }
}
`, serviceName, domain1, domain2, backendName)

	subscription := fmt.Sprintf(`
resource "fastly_tls_subscription" "test" {
  domains               = [%q, %q]
  common_name           = %q
  certificate_authority = "lets-encrypt"
  depends_on            = [fastly_service_cdn_auto.test]
}
`, domain1, domain2, commonName)

	return joinBlocks(service, subscription)
}

// ConfigTLSSubscriptionWithConfigurationID is ConfigTLSSubscription but also pins
// configuration_id to a specific, non-default TLS configuration, so a later step can change it
// alone to verify the update reaches the API.
func ConfigTLSSubscriptionWithConfigurationID(serviceName, domain1, domain2, backendName, commonName string) string {
	configuration := `
data "fastly_tls_configuration" "secondary" {
  name = "HTTP/3 & TLS v1.3 (s.sni)"
}
`

	service := fmt.Sprintf(`
resource "fastly_service_cdn_auto" "test" {
  name          = %q
  force_destroy = true

  domain {
    name = %q
  }

  domain {
    name = %q
  }

  backend {
    name              = %q
    address           = "api.example.com"
    port              = 443
    use_ssl           = true
    ssl_cert_hostname = "api.example.com"
    ssl_sni_hostname  = "api.example.com"
  }
}
`, serviceName, domain1, domain2, backendName)

	subscription := fmt.Sprintf(`
resource "fastly_tls_subscription" "test" {
  domains               = [%q, %q]
  common_name           = %q
  certificate_authority = "lets-encrypt"
  configuration_id      = data.fastly_tls_configuration.secondary.id
  depends_on            = [fastly_service_cdn_auto.test]
}
`, domain1, domain2, commonName)

	return joinBlocks(configuration, service, subscription)
}
