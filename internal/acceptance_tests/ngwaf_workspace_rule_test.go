package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
)

func TestAccFastlyNGWAFWorkspaceRule_lifecycle(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_rule_basic.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "type", "request"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "description", "Block a specific IP"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "enabled", "true"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "condition.0.field", "ip"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "condition.0.operator", "equals"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "condition.0.value", "127.0.0.1"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.type", "block"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.redirect_url", "https://example.com/blocked"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.response_code", "302"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_rule.test", "id"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_rule.test", "workspace_id"),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_rule_updated.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "description", "Allow a specific path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "enabled", "false"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "group_operator", "any"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "request_logging", "sampled"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "condition.0.field", "path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.type", "allow"),
				),
			},
			{
				ResourceName:      "fastly_ngwaf_workspace_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDFunc("fastly_ngwaf_workspace_rule.test"),
			},
		},
	})
}

func TestAccFastlyNGWAFWorkspaceRule_groupAndMultivalConditions(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-rule-group-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_rule_group_condition.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "group_condition.0.group_operator", "any"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "group_condition.0.condition.0.field", "ip"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "group_condition.0.multival_condition.0.field", "request_header"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "group_condition.0.multival_condition.0.condition.0.value", "X-Forwarded-For"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "multival_condition.0.field", "query_parameter"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "multival_condition.0.condition.0.value", "debug"),
				),
			},
		},
	})
}

// TestAccFastlyNGWAFWorkspaceRule_rateLimit exercises the rate_limit
// block, including request_header and request_cookie client_identifiers
// (see ConfigNGWAFWorkspaceRuleRateLimitClientIdentifiersCookie for why
// cookie is paired with ip). signal_payload isn't covered; it needs a
// real signal reference. The workspace and signal used here are created
// out-of-band via the go-fastly SDK since this repo has no signal
// resource yet.
// TODO: cover signal_payload and use a real fastly_ngwaf_workspace_signal
// resource once it exists.
func TestAccFastlyNGWAFWorkspaceRule_rateLimit(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-ws-rule-ratelimit-%s", suffix)

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
				Config: ConfigNGWAFWorkspaceRuleRateLimit(workspaceID, signal.ReferenceID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "type", "rate_limit"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "rate_limit.0.duration", "300"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "rate_limit.0.interval", "60"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "rate_limit.0.threshold", "100"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "rate_limit.0.client_identifiers.0.type", "ip"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.type", "log_request"),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceRuleRateLimitClientIdentifiers(workspaceID, signal.ReferenceID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "rate_limit.0.client_identifiers.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_ngwaf_workspace_rule.test", "rate_limit.0.client_identifiers.*", map[string]string{
						"type": "request_header",
						"name": "X-Forwarded-For",
						"key":  "ip",
					}),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceRuleRateLimitClientIdentifiersCookie(workspaceID, signal.ReferenceID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "rate_limit.0.client_identifiers.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_ngwaf_workspace_rule.test", "rate_limit.0.client_identifiers.*", map[string]string{
						"type": "ip",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_ngwaf_workspace_rule.test", "rate_limit.0.client_identifiers.*", map[string]string{
						"type": "request_cookie",
						"name": "session_id",
					}),
				),
			},
		},
	})
}

func TestAccFastlyNGWAFWorkspaceRule_signal(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-rule-signal-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_rule_signal.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "type", "signal"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "condition.0.field", "path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "condition.0.operator", "like"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.type", "exclude_signal"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.signal", "XSS"),
				),
			},
		},
	})
}

func TestAccFastlyNGWAFWorkspaceRule_templatedSignal(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-rule-tmplsig-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_rule_templated_signal.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "type", "templated_signal"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "description", ""),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.type", "templated_signal"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_rule.test", "action.0.signal", "LOGINATTEMPT"),
				),
			},
		},
	})
}

func TestAccFastlyNGWAFWorkspaceRules_dataSource(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-rules-ds-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_rules_data_source.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_workspace_rules.test", "id"),
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_workspace_rules.test", "workspace_id"),
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_rules.test", "rules.#", "1"),
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_rules.test", "rules.0.type", "request"),
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_rules.test", "rules.0.description", "Block a specific IP"),
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_workspace_rules.test", "rules.0.id"),
				),
			},
		},
	})
}

func importStateIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}
		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["workspace_id"], rs.Primary.ID), nil
	}
}

func ConfigNGWAFWorkspaceRule(blockFile, name string) string {
	raw, err := os.ReadFile("blocks/" + blockFile)
	if err != nil {
		panic(err)
	}
	return strings.ReplaceAll(string(raw), "{{.WORKSPACE_NAME}}", name)
}

func ConfigNGWAFWorkspaceRuleRateLimit(workspaceID, signalID string) string {
	return configNGWAFWorkspaceRuleRateLimit("blocks/ngwaf_workspace_rule_rate_limit.tf", workspaceID, signalID)
}

// ConfigNGWAFWorkspaceRuleRateLimitClientIdentifiers exercises the
// request_header client_identifiers type against the live API. Reuses the
// same out-of-band workspace/signal as the base rate_limit test since
// rate_limit.signal is unrelated to which client_identifiers type is under
// test.
func ConfigNGWAFWorkspaceRuleRateLimitClientIdentifiers(workspaceID, signalID string) string {
	return configNGWAFWorkspaceRuleRateLimit("blocks/ngwaf_workspace_rule_rate_limit_client_identifiers.tf", workspaceID, signalID)
}

// ConfigNGWAFWorkspaceRuleRateLimitClientIdentifiersCookie pairs
// request_cookie with ip: only ip may be combined with a second client
// identifier; every other type allows exactly one.
func ConfigNGWAFWorkspaceRuleRateLimitClientIdentifiersCookie(workspaceID, signalID string) string {
	return configNGWAFWorkspaceRuleRateLimit("blocks/ngwaf_workspace_rule_rate_limit_client_identifiers_cookie.tf", workspaceID, signalID)
}

func configNGWAFWorkspaceRuleRateLimit(blockFile, workspaceID, signalID string) string {
	raw, err := os.ReadFile(blockFile)
	if err != nil {
		panic(err)
	}
	replaced := strings.ReplaceAll(string(raw), "{{.WORKSPACE_ID}}", workspaceID)
	return strings.ReplaceAll(replaced, "{{.SIGNAL_ID}}", signalID)
}

func CheckNGWAFWorkspaceRuleDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_ngwaf_workspace_rule" {
			continue
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		ruleID := rs.Primary.ID

		_, err := rules.Get(context.Background(), client, &rules.GetInput{
			RuleID: &ruleID,
			Scope: &scope.Scope{
				Type:      scope.ScopeTypeWorkspace,
				AppliesTo: []string{workspaceID},
			},
		})
		if err == nil {
			return fmt.Errorf("NGWAF workspace rule %s still exists after destroy", ruleID)
		}
	}
	return nil
}
