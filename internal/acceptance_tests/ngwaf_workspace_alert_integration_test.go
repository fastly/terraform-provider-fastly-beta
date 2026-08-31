package acceptancetests

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/datadog"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/jira"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/mailinglist"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/microsoftteams"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/opsgenie"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/pagerduty"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/slack"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/webhook"
)

type ngwafWorkspaceAlertIntegrationAcceptanceCase struct {
	name            string
	resourceType    string
	resourceName    string
	dataSourceName  string
	basicBlock      string
	updatedBlock    string
	dataSourceBlock string
	updateAttr      string
	updatedValue    string
}

func TestAccFastlyNGWAFWorkspaceAlertIntegration_lifecycle(t *testing.T) {
	t.Parallel()

	for _, tc := range ngwafWorkspaceAlertIntegrationAcceptanceCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testAccFastlyNGWAFWorkspaceAlertIntegrationLifecycle(t, tc)
		})
	}
}

func TestAccFastlyDataSourceNGWAFWorkspaceAlertIntegrations(t *testing.T) {
	t.Parallel()

	for _, tc := range ngwafWorkspaceAlertIntegrationAcceptanceCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testAccFastlyNGWAFWorkspaceAlertIntegrationDataSource(t, tc)
		})
	}
}

func ngwafWorkspaceAlertIntegrationAcceptanceCases() []ngwafWorkspaceAlertIntegrationAcceptanceCase {
	return []ngwafWorkspaceAlertIntegrationAcceptanceCase{
		{
			name:            "datadog",
			resourceType:    "fastly_ngwaf_workspace_alert_datadog_integration",
			resourceName:    "fastly_ngwaf_workspace_alert_datadog_integration.test",
			dataSourceName:  "data.fastly_ngwaf_workspace_alert_datadog_integrations.test",
			basicBlock:      "ngwaf_workspace_alert_datadog_integration_basic.tf",
			updatedBlock:    "ngwaf_workspace_alert_datadog_integration_updated.tf",
			dataSourceBlock: "ngwaf_workspace_alert_datadog_integration_datasource.tf",
			updateAttr:      "key",
			updatedValue:    "1234567890abcdef1234567890abcdef2",
		},
		{
			name:            "jira",
			resourceType:    "fastly_ngwaf_workspace_alert_jira_integration",
			resourceName:    "fastly_ngwaf_workspace_alert_jira_integration.test",
			dataSourceName:  "data.fastly_ngwaf_workspace_alert_jira_integrations.test",
			basicBlock:      "ngwaf_workspace_alert_jira_integration_basic.tf",
			updatedBlock:    "ngwaf_workspace_alert_jira_integration_updated.tf",
			dataSourceBlock: "ngwaf_workspace_alert_jira_integration_datasource.tf",
			updateAttr:      "host",
			updatedValue:    "https://example-updated.atlassian.net",
		},
		{
			name:            "mailing_list",
			resourceType:    "fastly_ngwaf_workspace_alert_mailing_list_integration",
			resourceName:    "fastly_ngwaf_workspace_alert_mailing_list_integration.test",
			dataSourceName:  "data.fastly_ngwaf_workspace_alert_mailing_list_integrations.test",
			basicBlock:      "ngwaf_workspace_alert_mailing_list_integration_basic.tf",
			updatedBlock:    "ngwaf_workspace_alert_mailing_list_integration_updated.tf",
			dataSourceBlock: "ngwaf_workspace_alert_mailing_list_integration_datasource.tf",
			updateAttr:      "address",
			updatedValue:    "alerts-updated@example.com",
		},
		{
			name:            "microsoft_teams",
			resourceType:    "fastly_ngwaf_workspace_alert_microsoft_teams_integration",
			resourceName:    "fastly_ngwaf_workspace_alert_microsoft_teams_integration.test",
			dataSourceName:  "data.fastly_ngwaf_workspace_alert_microsoft_teams_integrations.test",
			basicBlock:      "ngwaf_workspace_alert_microsoft_teams_integration_basic.tf",
			updatedBlock:    "ngwaf_workspace_alert_microsoft_teams_integration_updated.tf",
			dataSourceBlock: "ngwaf_workspace_alert_microsoft_teams_integration_datasource.tf",
			updateAttr:      "webhook",
			updatedValue:    "https://example.com/webhooks/my-service-2",
		},
		{
			name:            "opsgenie",
			resourceType:    "fastly_ngwaf_workspace_alert_opsgenie_integration",
			resourceName:    "fastly_ngwaf_workspace_alert_opsgenie_integration.test",
			dataSourceName:  "data.fastly_ngwaf_workspace_alert_opsgenie_integrations.test",
			basicBlock:      "ngwaf_workspace_alert_opsgenie_integration_basic.tf",
			updatedBlock:    "ngwaf_workspace_alert_opsgenie_integration_updated.tf",
			dataSourceBlock: "ngwaf_workspace_alert_opsgenie_integration_datasource.tf",
			updateAttr:      "key",
			updatedValue:    "1234567890abcdef1234567890abcdef2",
		},
		{
			name:            "pagerduty",
			resourceType:    "fastly_ngwaf_workspace_alert_pagerduty_integration",
			resourceName:    "fastly_ngwaf_workspace_alert_pagerduty_integration.test",
			dataSourceName:  "data.fastly_ngwaf_workspace_alert_pagerduty_integrations.test",
			basicBlock:      "ngwaf_workspace_alert_pagerduty_integration_basic.tf",
			updatedBlock:    "ngwaf_workspace_alert_pagerduty_integration_updated.tf",
			dataSourceBlock: "ngwaf_workspace_alert_pagerduty_integration_datasource.tf",
			updateAttr:      "key",
			updatedValue:    "1234567890abcdef1234567890abcdef2",
		},
		{
			name:            "slack",
			resourceType:    "fastly_ngwaf_workspace_alert_slack_integration",
			resourceName:    "fastly_ngwaf_workspace_alert_slack_integration.test",
			dataSourceName:  "data.fastly_ngwaf_workspace_alert_slack_integrations.test",
			basicBlock:      "ngwaf_workspace_alert_slack_integration_basic.tf",
			updatedBlock:    "ngwaf_workspace_alert_slack_integration_updated.tf",
			dataSourceBlock: "ngwaf_workspace_alert_slack_integration_datasource.tf",
			updateAttr:      "webhook",
			updatedValue:    "https://example.com/webhooks/my-service-2",
		},
		{
			name:            "webhook",
			resourceType:    "fastly_ngwaf_workspace_alert_webhook_integration",
			resourceName:    "fastly_ngwaf_workspace_alert_webhook_integration.test",
			dataSourceName:  "data.fastly_ngwaf_workspace_alert_webhook_integrations.test",
			basicBlock:      "ngwaf_workspace_alert_webhook_integration_basic.tf",
			updatedBlock:    "ngwaf_workspace_alert_webhook_integration_updated.tf",
			dataSourceBlock: "ngwaf_workspace_alert_webhook_integration_datasource.tf",
			updateAttr:      "webhook",
			updatedValue:    "https://example.com/webhooks/my-service-2",
		},
	}
}

func testAccFastlyNGWAFWorkspaceAlertIntegrationLifecycle(t *testing.T, tc ngwafWorkspaceAlertIntegrationAcceptanceCase) {
	t.Helper()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-alert-%s-%s", strings.ReplaceAll(tc.name, "_", "-"), suffix)
	alertDescription := fmt.Sprintf("tf-test-%s-alert-%s", strings.ReplaceAll(tc.name, "_", "-"), suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceAlertIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceAlertIntegration(tc.basicBlock, workspaceName, alertDescription),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tc.resourceName, "description", alertDescription),
					resource.TestCheckResourceAttrPair(tc.resourceName, "workspace_id", "fastly_ngwaf_workspace.test", "id"),
					resource.TestCheckResourceAttrSet(tc.resourceName, "id"),
					CheckNGWAFWorkspaceAlertIntegrationExists(tc.resourceName),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceAlertIntegration(tc.updatedBlock, workspaceName, alertDescription),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tc.resourceName, "description", alertDescription),
					resource.TestCheckResourceAttr(tc.resourceName, tc.updateAttr, tc.updatedValue),
					CheckNGWAFWorkspaceAlertIntegrationExists(tc.resourceName),
				),
			},
			{
				Config:   ConfigNGWAFWorkspaceAlertIntegration(tc.updatedBlock, workspaceName, alertDescription),
				PlanOnly: true,
			},
			{
				ResourceName:      tc.resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: ImportStateIDForNGWAFWorkspaceAlertIntegration(tc.resourceName),
			},
		},
	})
}

func testAccFastlyNGWAFWorkspaceAlertIntegrationDataSource(t *testing.T, tc ngwafWorkspaceAlertIntegrationAcceptanceCase) {
	t.Helper()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-alert-ds-%s-%s", strings.ReplaceAll(tc.name, "_", "-"), suffix)
	alertDescription := fmt.Sprintf("tf-test-%s-alert-ds-%s", strings.ReplaceAll(tc.name, "_", "-"), suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceAlertIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceAlertIntegration(tc.dataSourceBlock, workspaceName, alertDescription),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(tc.dataSourceName, "alerts.#", "1"),
					resource.TestCheckResourceAttrSet(tc.dataSourceName, "id"),
				),
			},
		},
	})
}

func ConfigNGWAFWorkspaceAlertIntegration(blockFile, workspaceName, alertDescription string) string {
	return RenderBlock("internal/acceptance_tests/blocks/"+blockFile, map[string]string{
		"WORKSPACE_NAME":    workspaceName,
		"ALERT_DESCRIPTION": alertDescription,
	})
}

func ImportStateIDForNGWAFWorkspaceAlertIntegration(n string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return "", fmt.Errorf("not found: %s", n)
		}
		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["workspace_id"], rs.Primary.ID), nil
	}
}

func CheckNGWAFWorkspaceAlertIntegrationExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		alertID := rs.Primary.ID

		if err := getNGWAFWorkspaceAlertIntegration(context.Background(), client, rs.Type, workspaceID, alertID); err != nil {
			return fmt.Errorf("unable to retrieve NGWAF workspace alert integration %s: %w", alertID, err)
		}

		return nil
	}
}

func CheckNGWAFWorkspaceAlertIntegrationDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if !strings.HasPrefix(rs.Type, "fastly_ngwaf_workspace_alert_") || !strings.HasSuffix(rs.Type, "_integration") {
			continue
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		if err := getNGWAFWorkspaceAlertIntegration(context.Background(), client, rs.Type, workspaceID, rs.Primary.ID); err == nil {
			return fmt.Errorf("NGWAF workspace alert integration %s still exists after destroy", rs.Primary.ID)
		}
	}

	return nil
}

func getNGWAFWorkspaceAlertIntegration(ctx context.Context, c *fastly.Client, resourceType, workspaceID, alertID string) error {
	switch resourceType {
	case "fastly_ngwaf_workspace_alert_datadog_integration":
		_, err := datadog.Get(ctx, c, &datadog.GetInput{WorkspaceID: &workspaceID, AlertID: &alertID})
		return err
	case "fastly_ngwaf_workspace_alert_jira_integration":
		_, err := jira.Get(ctx, c, &jira.GetInput{WorkspaceID: &workspaceID, AlertID: &alertID})
		return err
	case "fastly_ngwaf_workspace_alert_mailing_list_integration":
		_, err := mailinglist.Get(ctx, c, &mailinglist.GetInput{WorkspaceID: &workspaceID, AlertID: &alertID})
		return err
	case "fastly_ngwaf_workspace_alert_microsoft_teams_integration":
		_, err := microsoftteams.Get(ctx, c, &microsoftteams.GetInput{WorkspaceID: &workspaceID, AlertID: &alertID})
		return err
	case "fastly_ngwaf_workspace_alert_opsgenie_integration":
		_, err := opsgenie.Get(ctx, c, &opsgenie.GetInput{WorkspaceID: &workspaceID, AlertID: &alertID})
		return err
	case "fastly_ngwaf_workspace_alert_pagerduty_integration":
		_, err := pagerduty.Get(ctx, c, &pagerduty.GetInput{WorkspaceID: &workspaceID, AlertID: &alertID})
		return err
	case "fastly_ngwaf_workspace_alert_slack_integration":
		_, err := slack.Get(ctx, c, &slack.GetInput{WorkspaceID: &workspaceID, AlertID: &alertID})
		return err
	case "fastly_ngwaf_workspace_alert_webhook_integration":
		_, err := webhook.Get(ctx, c, &webhook.GetInput{WorkspaceID: &workspaceID, AlertID: &alertID})
		return err
	default:
		return fmt.Errorf("unsupported alert integration resource type %q", resourceType)
	}
}
