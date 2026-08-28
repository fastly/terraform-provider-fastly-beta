package acceptancetests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccFastlyNGWAFWorkspaceRateLimitRule_lifecycle exercises the
// rate_limit block, including request_header, request_cookie, and
// signal_payload client_identifiers (see
// ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersCookie for why cookie is
// paired with ip). The workspace and signal are managed by Terraform via
// fastly_ngwaf_workspace and fastly_ngwaf_workspace_signal.
func TestAccFastlyNGWAFWorkspaceRateLimitRule_lifecycle(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-ws-ratelimit-rule-%s", suffix)
	signalName := fmt.Sprintf("tf-test-signal-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy: func(s *terraform.State) error {
			if err := CheckNGWAFWorkspaceRuleDestroy(s); err != nil {
				return err
			}
			return CheckNGWAFWorkspaceSignalAndWorkspaceDestroy(s)
		},
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRateLimitRule(workspaceName, signalName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.duration", "300"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.interval", "60"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.threshold", "100"),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.client_identifiers.*", map[string]string{
						"type": "ip",
					}),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rate_limit_rule.test", "action.0.type", "log_request"),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiers(workspaceName, signalName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.client_identifiers.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.client_identifiers.*", map[string]string{
						"type": "request_header",
						"name": "X-Forwarded-For",
						"key":  "ip",
					}),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersCookie(workspaceName, signalName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.client_identifiers.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.client_identifiers.*", map[string]string{
						"type": "ip",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.client_identifiers.*", map[string]string{
						"type": "request_cookie",
						"name": "session_id",
					}),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersSignalPayload(workspaceName, signalName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.client_identifiers.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.client_identifiers.*", map[string]string{
						"type": "signal_payload",
					}),
					resource.TestCheckResourceAttrPair(
						"fastly_ngwaf_workspace_rate_limit_rule.test", "rate_limit.0.signal",
						"fastly_ngwaf_workspace_signal.test", "reference_id",
					),
				),
			},
		},
	})
}

// TestAccFastlyNGWAFWorkspaceRateLimitRule_missingRateLimitBlock checks that a
// rate limit rule without its rate_limit block is rejected at plan time rather
// than reaching the API.
func TestAccFastlyNGWAFWorkspaceRateLimitRule_missingRateLimitBlock(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-ratelimit-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigNGWAFWorkspaceRule("ngwaf_workspace_rate_limit_rule_missing_block.tf", name),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Block[\s\S]*rate_limit must have a configuration value`),
			},
		},
	})
}

// TestAccFastlyNGWAFWorkspaceRateLimitRule_missingClientIdentifiers checks
// that a rate_limit block without its client_identifiers block is rejected
// at plan time rather than reaching the API.
func TestAccFastlyNGWAFWorkspaceRateLimitRule_missingClientIdentifiers(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-ratelimit-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigNGWAFWorkspaceRule("ngwaf_workspace_rate_limit_rule_missing_client_identifiers.tf", name),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Block[\s\S]*client_identifiers must have a configuration value`),
			},
		},
	})
}

// TestAccFastlyNGWAFWorkspaceRateLimitRule_twoNonIPClientIdentifiers checks
// that pairing two non-ip client_identifiers entries is rejected at plan
// time: only ip may be combined with a second entry.
func TestAccFastlyNGWAFWorkspaceRateLimitRule_twoNonIPClientIdentifiers(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-ratelimit-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigNGWAFWorkspaceRule("ngwaf_workspace_rate_limit_rule_client_identifiers_two_non_ip.tf", name),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid client identifier[\s\S]*only contain 2 entries when one of them is type "ip"`),
			},
		},
	})
}

func ConfigNGWAFWorkspaceRateLimitRule(workspaceName, signalName string) string {
	return configNGWAFWorkspaceRateLimitRule("internal/acceptance_tests/blocks/ngwaf_workspace_rate_limit_rule.tf", workspaceName, signalName)
}

// ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiers exercises the
// request_header client_identifiers type against the live API. Reuses the
// same workspace/signal fixture as the base rate_limit test since
// rate_limit.signal is unrelated to which client_identifiers type is under
// test.
func ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiers(workspaceName, signalName string) string {
	return configNGWAFWorkspaceRateLimitRule("internal/acceptance_tests/blocks/ngwaf_workspace_rate_limit_rule_client_identifiers.tf", workspaceName, signalName)
}

// ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersCookie pairs
// request_cookie with ip: only ip may be combined with a second client
// identifier; every other type allows exactly one.
func ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersCookie(workspaceName, signalName string) string {
	return configNGWAFWorkspaceRateLimitRule("internal/acceptance_tests/blocks/ngwaf_workspace_rate_limit_rule_client_identifiers_cookie.tf", workspaceName, signalName)
}

// ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersSignalPayload exercises
// the signal_payload client_identifiers type, which requires a real signal
// reference rather than a name.
func ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersSignalPayload(workspaceName, signalName string) string {
	return configNGWAFWorkspaceRateLimitRule("internal/acceptance_tests/blocks/ngwaf_workspace_rate_limit_rule_client_identifiers_signal_payload.tf", workspaceName, signalName)
}

func configNGWAFWorkspaceRateLimitRule(blockPath, workspaceName, signalName string) string {
	return RenderBlock(blockPath, map[string]string{
		"WORKSPACE_NAME": workspaceName,
		"SIGNAL_NAME":    signalName,
	})
}
