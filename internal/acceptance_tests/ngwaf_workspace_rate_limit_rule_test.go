package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
)

// TestAccFastlyNGWAFWorkspaceRateLimitRule_lifecycle exercises the
// rate_limit block, including request_header and request_cookie
// client_identifiers (see
// ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersCookie for why cookie is
// paired with ip). signal_payload isn't covered; it needs a real signal
// reference. The workspace and signal used here are created out-of-band via
// the go-fastly SDK since this repo has no signal resource yet.
// TODO: cover signal_payload and use a real fastly_ngwaf_workspace_signal
// resource once it exists.
func TestAccFastlyNGWAFWorkspaceRateLimitRule_lifecycle(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-ws-ratelimit-rule-%s", suffix)

	workspace, err := ws.Create(context.Background(), client, &ws.CreateInput{
		Name:        &workspaceName,
		Description: new("created out-of-band for a rate_limit rule test"),
		Mode:        new("log"),
	})
	if err != nil {
		t.Fatalf("creating out-of-band NGWAF workspace: %s", err)
	}
	workspaceID := workspace.WorkspaceID
	t.Cleanup(func() {
		if err := ws.Delete(context.Background(), client, &ws.DeleteInput{WorkspaceID: &workspaceID}); err != nil {
			t.Logf("cleanup: deleting out-of-band NGWAF workspace %s: %s", workspaceID, err)
		}
	})

	signalName := fmt.Sprintf("tf-test-signal-%s", suffix)
	signal, err := signals.Create(context.Background(), client, &signals.CreateInput{
		Name:        &signalName,
		Description: new("created out-of-band for a rate_limit rule test"),
		Scope: &scope.Scope{
			Type:      scope.ScopeTypeWorkspace,
			AppliesTo: []string{workspaceID},
		},
	})
	if err != nil {
		t.Fatalf("creating out-of-band NGWAF signal: %s", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRateLimitRule(workspaceID, signal.ReferenceID),
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
				Config: ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiers(workspaceID, signal.ReferenceID),
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
				Config: ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersCookie(workspaceID, signal.ReferenceID),
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

func ConfigNGWAFWorkspaceRateLimitRule(workspaceID, signalID string) string {
	return configNGWAFWorkspaceRateLimitRule("blocks/ngwaf_workspace_rate_limit_rule.tf", workspaceID, signalID)
}

// ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiers exercises the
// request_header client_identifiers type against the live API. Reuses the
// same out-of-band workspace/signal as the base rate_limit test since
// rate_limit.signal is unrelated to which client_identifiers type is under
// test.
func ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiers(workspaceID, signalID string) string {
	return configNGWAFWorkspaceRateLimitRule("blocks/ngwaf_workspace_rate_limit_rule_client_identifiers.tf", workspaceID, signalID)
}

// ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersCookie pairs
// request_cookie with ip: only ip may be combined with a second client
// identifier; every other type allows exactly one.
func ConfigNGWAFWorkspaceRateLimitRuleClientIdentifiersCookie(workspaceID, signalID string) string {
	return configNGWAFWorkspaceRateLimitRule("blocks/ngwaf_workspace_rate_limit_rule_client_identifiers_cookie.tf", workspaceID, signalID)
}

func configNGWAFWorkspaceRateLimitRule(blockFile, workspaceID, signalID string) string {
	raw, err := os.ReadFile(blockFile)
	if err != nil {
		panic(err)
	}
	replaced := strings.ReplaceAll(string(raw), "{{.WORKSPACE_ID}}", workspaceID)
	return strings.ReplaceAll(replaced, "{{.SIGNAL_ID}}", signalID)
}
