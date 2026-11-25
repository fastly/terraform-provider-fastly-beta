package compatibility

import (
	"strings"
	"testing"
)

// The VCL-07 credential routes were already registered in attributeRules.
// Reuse those rules rather than appending duplicate source routes.
func TestRemainingCDNLoggingRoutesUseExistingRegistryRules(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(allRules())
	if err != nil {
		t.Fatalf("built-in migration registry is invalid: %v", err)
	}

	checked := 0
	for block, fields := range RemainingCDNLoggingFields {
		for _, field := range fields {
			object, member, nested := RemainingCDNLoggingObjectRoute(block, field)
			if !nested {
				continue
			}
			legacy := "fastly_service_vcl." + block + "." + field
			want := "fastly_service_cdn_auto." + block + "." + object + "." + member
			var matches []Rule
			for _, rule := range registry.RulesForLegacy(legacy) {
				if rule.Kind == ConstructAttribute && rule.Family == FamilyCDN {
					matches = append(matches, rule)
				}
			}
			if len(matches) != 1 {
				t.Errorf("%s: expected exactly one CDN credential route, got %d", legacy, len(matches))
				continue
			}
			rule := matches[0]
			if rule.Target != want || rule.Support != SupportAutomatic ||
				rule.HCL != HCLReshape || rule.State != StateMove {
				t.Errorf("%s: unexpected credential mapping: %+v; want %s", legacy, rule, want)
			}
			checked++
		}
	}
	if checked != 14 {
		t.Errorf("expected 14 existing credential routes, checked %d", checked)
	}
}

func TestRemainingComputeLoggingBlocksRequireExplicitPrerequisites(t *testing.T) {
	t.Parallel()

	registry := Default()
	for _, block := range []string{
		"logging_ftp", "logging_sftp", "logging_datadog", "logging_loggly",
		"logging_grafanacloudlogs", "logging_scalyr", "logging_newrelic",
		"logging_newrelicotlp", "logging_heroku", "logging_honeycomb",
		"logging_logshuttle",
	} {
		legacy := "fastly_service_compute." + block
		var found bool
		for _, rule := range registry.RulesForLegacy(legacy) {
			if rule.Kind != ConstructServiceBlock || rule.Family != FamilyCompute {
				continue
			}
			found = true
			if rule.Support != SupportBlocked || rule.HCL != HCLReject ||
				rule.State != StateNone || strings.TrimSpace(rule.ProviderPrerequisite) == "" {
				t.Errorf("%s: blocked Compute rule missing fail-closed configuration or prerequisite: %+v", legacy, rule)
			}
		}
		if !found {
			t.Errorf("%s: missing Compute logging block rule", legacy)
		}
	}
}

// Defaults are HCL reshapes, even if the endpoint has no nested credentials.
// Marking these blocks HCLUnchanged prevents the analyzer from scheduling
// their format/message_type edits.
func TestRemainingLoggingDefaultPinningIsScheduled(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		legacy, target string
	}{
		{"fastly_service_vcl.logging_papertrail", "fastly_service_cdn_auto.logging_papertrail"},
		{"fastly_service_vcl.logging_sumologic", "fastly_service_cdn_auto.logging_sumologic"},
		{"fastly_service_compute.logging_sumologic", "fastly_service_compute_auto.logging_sumologic"},
	} {
		t.Run(tc.legacy, func(t *testing.T) {
			var matches []Rule
			for _, rule := range Default().RulesForLegacy(tc.legacy) {
				if rule.Kind == ConstructServiceBlock {
					matches = append(matches, rule)
				}
			}
			if len(matches) != 1 {
				t.Fatalf("%s: expected one block rule, found %d", tc.legacy, len(matches))
			}
			rule := matches[0]
			if rule.Target != tc.target || rule.Support != SupportAutomatic ||
				rule.HCL != HCLReshape || rule.State != StateMove {
				t.Fatalf("%s: default pinning cannot run with rule %+v", tc.legacy, rule)
			}
		})
	}
}
