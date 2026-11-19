package acceptancetests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

// botManagementTestBotID must belong to botManagementTestCategoryID.
const (
	botManagementTestCategoryID = "cat_ai_crawler"
	botManagementTestBotID      = "bot_gptbot"
)

// TestAccFastlyBotManagement_lifecycle enables ContentGuard in its own step
// before looking up the workspace, matching the documented requirement that
// the workspace exists before the data source reads it.
func TestAccFastlyBotManagement_lifecycle(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	workspaceName := fmt.Sprintf("tf-test-bot-management-%s", acctest.RandString(10))

	const (
		workspace = "fastly_bot_management_workspace.test"
		category  = "fastly_bot_management_category.test"
		bot       = "fastly_bot_management_bot.test"
	)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigProductEnablementBotManagementOnly(serviceName, domainName, backendName, "on"),
				Check:  CheckServiceExists("fastly_service_cdn_auto.test"),
			},
			{
				Config: ConfigBotManagement(serviceName, domainName, backendName, workspaceName, "log", "block", "allow"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_bot_management_workspace.by_service", "id"),
					resource.TestCheckResourceAttrPair("data.fastly_bot_management_workspace.by_id", "id", "data.fastly_bot_management_workspace.by_service", "id"),
					resource.TestCheckTypeSetElemAttrPair("data.fastly_bot_management_workspace.by_service", "services.*", "fastly_service_cdn_auto.test", "id"),

					resource.TestCheckResourceAttrPair(workspace, "workspace_id", "data.fastly_bot_management_workspace.by_service", "id"),
					resource.TestCheckResourceAttrPair(workspace, "id", workspace, "workspace_id"),
					resource.TestCheckResourceAttr(workspace, "name", workspaceName),
					resource.TestCheckResourceAttr(workspace, "description", "Managed by Terraform acceptance tests"),
					resource.TestCheckResourceAttr(workspace, "protection_mode", "log"),

					resource.TestCheckResourceAttr(category, "category_id", botManagementTestCategoryID),
					resource.TestCheckResourceAttr(category, "action", "block"),
					resource.TestCheckResourceAttrSet(category, "name"),

					resource.TestCheckResourceAttr(bot, "bot_id", botManagementTestBotID),
					resource.TestCheckResourceAttr(bot, "category_id", botManagementTestCategoryID),
					resource.TestCheckResourceAttr(bot, "action", "allow"),
					resource.TestCheckResourceAttrSet(bot, "name"),

					resource.TestCheckTypeSetElemNestedAttrs("data.fastly_bot_management_categories.test", "categories.*", map[string]string{
						"category_id": botManagementTestCategoryID,
					}),
					resource.TestCheckTypeSetElemNestedAttrs("data.fastly_bot_management_bots.test", "bots.*", map[string]string{
						"bot_id":      botManagementTestBotID,
						"category_id": botManagementTestCategoryID,
					}),
				),
			},
			{
				Config: ConfigBotManagement(serviceName, domainName, backendName, workspaceName, "log", "block", "inherit"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(bot, "action", "inherit"),
					checkBotManagementBotAction(category, botManagementTestBotID, "inherit"),
				),
			},
			{
				Config: ConfigBotManagement(serviceName, domainName, backendName, workspaceName, "block", "challenge", "block"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(workspace, "protection_mode", "block"),
					resource.TestCheckResourceAttr(category, "action", "challenge"),
					resource.TestCheckResourceAttr(bot, "action", "block"),
				),
			},
			{
				Config:   ConfigBotManagement(serviceName, domainName, backendName, workspaceName, "block", "challenge", "block"),
				PlanOnly: true,
			},
			{
				ResourceName:      workspace,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      category,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      bot,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:    bot,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
			},
			{
				// Removing the bot resource leaves its action unchanged.
				Config: ConfigBotManagement(serviceName, domainName, backendName, workspaceName, "block", "challenge", ""),
				Check:  checkBotManagementBotAction("fastly_bot_management_category.test", botManagementTestBotID, "block"),
			},
		},
	})
}

func ConfigBotManagement(serviceName, domainName, backendName, workspaceName, protectionMode, categoryAction, botAction string) string {
	return ConfigProductEnablementBotManagementOnly(serviceName, domainName, backendName, "on") + "\n" +
		RenderBlock("internal/acceptance_tests/blocks/bot_management.tf", map[string]string{
			"WORKSPACE_NAME":        workspaceName,
			"WORKSPACE_DESCRIPTION": "Managed by Terraform acceptance tests",
			"PROTECTION_MODE":       protectionMode,
			"CATEGORY_ID":           botManagementTestCategoryID,
			"CATEGORY_ACTION":       categoryAction,
			"BOT_ID":                botManagementTestBotID,
			"BOT_ACTION":            botAction,
		})
}

// checkBotManagementBotAction reads the bot straight from the API, using the
// workspace and category from the named category resource, since the bot
// resource itself is gone by the time this runs.
func checkBotManagementBotAction(categoryResource, botID, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[categoryResource]
		if !ok {
			return fmt.Errorf("not found: %s", categoryResource)
		}
		workspaceID := rs.Primary.Attributes["workspace_id"]
		categoryID := rs.Primary.Attributes["category_id"]

		client, err := NewFastlyClient()
		if err != nil {
			return err
		}
		got, err := policy.GetBot(context.Background(), client, &policy.GetBotInput{
			WorkspaceID: &workspaceID,
			CategoryID:  &categoryID,
			BotID:       &botID,
		})
		if err != nil {
			return fmt.Errorf("reading bot %s: %w", botID, err)
		}
		if got.Action.Type != want {
			return fmt.Errorf("bot %s action = %q, want %q", botID, got.Action.Type, want)
		}
		return nil
	}
}
