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

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
)

func TestAccFastlyNGWAFWorkspaceTemplatedSignalRule_lifecycle(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-tmplsig-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRule("ngwaf_workspace_templated_signal_rule.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_templated_signal_rule.test", "enabled", "true"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_templated_signal_rule.test", "condition.0.field", "path"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_templated_signal_rule.test", "action.0.signal", "LOGINATTEMPT"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_templated_signal_rule.test", "id"),
				),
			},
			{
				ResourceName:      "fastly_ngwaf_workspace_templated_signal_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDFunc("fastly_ngwaf_workspace_templated_signal_rule.test"),
			},
		},
	})
}

// TestAccFastlyNGWAFWorkspaceTemplatedSignalRule_apiRequiresConditions pins the
// API's requirement that a templated_signal rule carry at least one condition.
// The rule is created through the SDK because the provider rejects this shape
// at plan time (see _missingConditions).
func TestAccFastlyNGWAFWorkspaceTemplatedSignalRule_apiRequiresConditions(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	workspaceName := fmt.Sprintf("tf-test-ws-tmplsig-nocond-%s", acctest.RandString(10))
	workspace, err := ws.Create(context.Background(), client, &ws.CreateInput{
		Name:        &workspaceName,
		Description: new("created out-of-band for a templated_signal condition test"),
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

	workspaceScope := &scope.Scope{
		Type:      scope.ScopeTypeWorkspace,
		AppliesTo: []string{workspaceID},
	}

	rule, err := rules.Create(context.Background(), client, &rules.CreateInput{
		Type:    new("templated_signal"),
		Enabled: new(true),
		Scope:   workspaceScope,
		Actions: []*rules.CreateAction{{
			Type:   new("templated_signal"),
			Signal: new("LOGINATTEMPT"),
		}},
	})
	if err == nil {
		ruleID := rule.RuleID
		if delErr := rules.Delete(context.Background(), client, &rules.DeleteInput{RuleID: &ruleID, Scope: workspaceScope}); delErr != nil {
			t.Logf("cleanup: deleting NGWAF rule %s: %s", ruleID, delErr)
		}
		t.Fatal("the API accepted a templated_signal rule with no conditions; ngwafrule.ValidateConditions can now exempt this rule type")
	}
	if !strings.Contains(err.Error(), "condition") {
		t.Fatalf("expected the API to reject a templated_signal rule for having no conditions, got: %s", err)
	}
}

// TestAccFastlyNGWAFWorkspaceTemplatedSignalRule_missingConditions checks that
// the shape _apiRequiresConditions proves invalid is caught at plan time.
func TestAccFastlyNGWAFWorkspaceTemplatedSignalRule_missingConditions(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("tf-test-ws-tmplsig-rule-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigNGWAFWorkspaceRule("ngwaf_workspace_templated_signal_rule_no_conditions.tf", name),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Missing rule conditions`),
			},
		},
	})
}
